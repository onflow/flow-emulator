/*
 * Flow Emulator
 *
 * Copyright Flow Foundation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package remote

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/onflow/flow/protobuf/go/flow/executiondata"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-emulator/storage"
	"github.com/onflow/flow-emulator/storage/sqlite"
	"github.com/onflow/flow-emulator/types"
)

// Retry and circuit breaker configuration
const (
	maxRetries     = 5
	baseDelay      = 100 * time.Millisecond
	maxDelay       = 30 * time.Second
	jitterFactor   = 0.1
	circuitTimeout = 30 * time.Second // Circuit breaker timeout
)

// isRateLimitError checks if the error is a rate limiting error
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}

	st, ok := status.FromError(err)
	if !ok {
		return false
	}

	// Check for common rate limiting gRPC codes
	switch st.Code() {
	case codes.ResourceExhausted:
		return true
	case codes.Unavailable:
		// Sometimes rate limits are returned as unavailable
		return strings.Contains(st.Message(), "rate") ||
			strings.Contains(st.Message(), "limit") ||
			strings.Contains(st.Message(), "throttle")
	case codes.Aborted:
		// Some services return rate limits as aborted
		return strings.Contains(st.Message(), "rate") ||
			strings.Contains(st.Message(), "limit")
	}

	return false
}

// exponentialBackoffWithJitter calculates delay with exponential backoff and jitter
func exponentialBackoffWithJitter(attempt int) time.Duration {
	if attempt <= 0 {
		return baseDelay
	}

	// Calculate exponential delay: baseDelay * 2^(attempt-1)
	delay := float64(baseDelay) * math.Pow(2, float64(attempt-1))

	// Cap at maxDelay
	if delay > float64(maxDelay) {
		delay = float64(maxDelay)
	}

	// Add jitter: ±10% random variation
	jitter := delay * jitterFactor * (2*rand.Float64() - 1)
	delay += jitter

	// Ensure minimum delay
	if delay < float64(baseDelay) {
		delay = float64(baseDelay)
	}

	return time.Duration(delay)
}

// retryWithBackoff executes a function with exponential backoff retry on rate limit errors
func (s *Store) retryWithBackoff(ctx context.Context, operation string, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check circuit breaker first
		if !s.circuitBreaker.canMakeRequest() {
			s.logger.Debug().
				Str("operation", operation).
				Msg("Circuit breaker is open, skipping request")
			return fmt.Errorf("circuit breaker is open")
		}

		err := fn()
		if err == nil {
			s.circuitBreaker.recordSuccess()
			return nil
		}

		lastErr = err

		// Only retry on recognized, transient rate limit errors
		if !isRateLimitError(err) {
			return err
		}

		// Record circuit breaker failure for rate limit errors
		s.circuitBreaker.recordFailure()

		// Continue with retry logic for rate limits only
		if attempt == maxRetries {
			s.logger.Warn().
				Str("operation", operation).
				Int("attempt", attempt+1).
				Err(err).
				Msg("Request failed after max attempts")
			return err
		}

		// Calculate delay and wait
		delay := exponentialBackoffWithJitter(attempt)
		s.logger.Debug().
			Str("operation", operation).
			Int("attempt", attempt+1).
			Dur("delay", delay).
			Err(err).
			Msg("Request failed, retrying with backoff")

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return lastErr
}

// circuitBreaker implements a simple circuit breaker pattern
type circuitBreaker struct {
	mu       sync.RWMutex
	failures int
	lastFail time.Time
	timeout  time.Duration
}

// canMakeRequest checks if requests can be made (circuit breaker is closed)
func (cb *circuitBreaker) canMakeRequest() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	// If we've had recent failures, check timeout
	if cb.failures > 0 && time.Since(cb.lastFail) < cb.timeout {
		return false
	}

	return true
}

// recordFailure records a failure and opens the circuit breaker
func (cb *circuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFail = time.Now()
}

// recordSuccess records a success and closes the circuit breaker
func (cb *circuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0 // Reset on success
}

type Store struct {
	*sqlite.Store
	executionClient executiondata.ExecutionDataAPIClient
	accessClient    access.AccessAPIClient
	grpcConn        *grpc.ClientConn
	host            string
	chainID         flowgo.ChainID
	forkHeight      uint64
	logger          *zerolog.Logger
	circuitBreaker  *circuitBreaker
}

type Option func(*Store)

// WithForkHost configures the remote access/observer node gRPC endpoint.
// Expects raw host:port with no scheme.
func WithForkHost(host string) Option {
	return func(store *Store) {
		store.host = host
	}
}

// WithRPCHost sets access/observer node host. Deprecated: use WithForkHost.
func WithRPCHost(host string, chainID flowgo.ChainID) Option {
	return func(store *Store) {
		// Keep legacy behavior: set host and (optionally) chainID for validation.
		store.host = host
		store.chainID = chainID
	}
}

// WithStartBlockHeight sets the start height for the store.
// WithForkHeight sets the pinned fork height.
func WithForkHeight(height uint64) Option {
	return func(store *Store) {
		store.forkHeight = height
	}
}

// WithStartBlockHeight is deprecated: use WithForkHeight.
func WithStartBlockHeight(height uint64) Option { return WithForkHeight(height) }

// WithClient can set an rpc host client
//
// This is mostly use for testing.
func WithClient(
	executionClient executiondata.ExecutionDataAPIClient,
	accessClient access.AccessAPIClient,
) Option {
	return func(store *Store) {
		store.executionClient = executionClient
		store.accessClient = accessClient
	}
}

func New(provider *sqlite.Store, logger *zerolog.Logger, options ...Option) (*Store, error) {
	store := &Store{
		Store:  provider,
		logger: logger,
		circuitBreaker: &circuitBreaker{
			timeout: circuitTimeout,
		},
	}

	for _, opt := range options {
		opt(store)
	}

	if store.executionClient == nil {
		if store.host == "" {
			return nil, fmt.Errorf("rpc host must be provided")
		}

		conn, err := grpc.NewClient(
			store.host,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1024*1024*1024)),
		)
		if err != nil {
			return nil, fmt.Errorf("could not connect to rpc host: %w", err)
		}

		store.grpcConn = conn
		store.executionClient = executiondata.NewExecutionDataAPIClient(conn)
		store.accessClient = access.NewAccessAPIClient(conn)
	}

	var params *access.GetNetworkParametersResponse
	err := store.retryWithBackoff(context.Background(), "GetNetworkParameters", func() error {
		var err error
		params, err = store.accessClient.GetNetworkParameters(context.Background(), &access.GetNetworkParametersRequest{})
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("could not get network parameters: %w", err)
	}

	// If a chainID was provided (legacy path), validate it matches the remote. If not provided, skip.
	if store.chainID != "" {
		if params.ChainId != store.chainID.String() {
			return nil, fmt.Errorf("chain ID of rpc host does not match chain ID provided in config: %s != %s", params.ChainId, store.chainID)
		}
	}

	// Record remote chain ID if not already set via options
	if store.chainID == "" {
		store.chainID = flowgo.ChainID(params.ChainId)
	}

	if err := store.initializeStartBlock(context.Background()); err != nil {
		return nil, err
	}

	store.DataGetter = store
	store.DataSetter = store
	store.KeyGenerator = &storage.DefaultKeyGenerator{}

	return store, nil
}

// initializeStartBlock initializes and stores the fork height and local latest height.
func (s *Store) initializeStartBlock(ctx context.Context) error {
	// the fork height may already be set in the db if restarting from persistent store
	forkHeight, err := s.ForkedBlockHeight(ctx)
	if err == nil && forkHeight > 0 {
		s.forkHeight = forkHeight
		return nil
	}
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("could not get forked block height: %w", err)
	}

	// use the current latest block from the rpc host if no height was provided
	if s.forkHeight == 0 {
		var resp *access.BlockHeaderResponse
		err := s.retryWithBackoff(ctx, "GetLatestBlockHeader", func() error {
			var err error
			resp, err = s.accessClient.GetLatestBlockHeader(ctx, &access.GetLatestBlockHeaderRequest{IsSealed: true})
			return err
		})
		if err != nil {
			return fmt.Errorf("could not get last block height: %w", err)
		}
		s.forkHeight = resp.Block.Height
	}

	s.logger.Info().
		Uint64("forkHeight", s.forkHeight).
		Str("host", s.host).
		Str("chainId", s.chainID.String()).
		Msg("Using fork height")

	// store the initial fork height. any future queries for data on the rpc host will be fixed
	// to this height.
	err = s.StoreForkedBlockHeight(ctx, s.forkHeight)
	if err != nil {
		return fmt.Errorf("could not set start block height: %w", err)
	}

	// initialize the local latest height.
	err = s.SetBlockHeight(s.forkHeight)
	if err != nil {
		return fmt.Errorf("could not set start block height: %w", err)
	}

	return nil
}

func (s *Store) BlockByID(ctx context.Context, blockID flowgo.Identifier) (*flowgo.Block, error) {
	var height uint64
	block, err := s.DefaultStore.BlockByID(ctx, blockID)
	if err == nil {
		height = block.Height
	} else if errors.Is(err, storage.ErrNotFound) {
		var heightRes *access.BlockHeaderResponse
		err := s.retryWithBackoff(ctx, "GetBlockHeaderByID", func() error {
			var err error
			heightRes, err = s.accessClient.GetBlockHeaderByID(ctx, &access.GetBlockHeaderByIDRequest{Id: blockID[:]})
			return err
		})
		if err != nil {
			return nil, err
		}
		height = heightRes.Block.Height
	} else {
		return nil, err
	}

	return s.BlockByHeight(ctx, height)
}

func (s *Store) LatestBlock(ctx context.Context) (flowgo.Block, error) {
	latestBlockHeight, err := s.LatestBlockHeight(ctx)
	if err != nil {
		return flowgo.Block{}, fmt.Errorf("could not get local latest block: %w", err)
	}

	block, err := s.BlockByHeight(ctx, latestBlockHeight)
	if err != nil {
		return flowgo.Block{}, err
	}

	return *block, nil
}

func (s *Store) BlockByHeight(ctx context.Context, height uint64) (*flowgo.Block, error) {
	block, err := s.DefaultStore.BlockByHeight(ctx, height)
	if err == nil {
		return block, nil
	}

	latestBlockHeight, err := s.LatestBlockHeight(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get local latest block: %w", err)
	}
	if height > latestBlockHeight {
		return nil, &types.BlockNotFoundByHeightError{Height: height}
	}

	var blockRes *access.BlockHeaderResponse
	err = s.retryWithBackoff(ctx, "GetBlockHeaderByHeight", func() error {
		var err error
		blockRes, err = s.accessClient.GetBlockHeaderByHeight(ctx, &access.GetBlockHeaderByHeightRequest{Height: height})
		return err
	})
	if err != nil {
		return nil, err
	}

	header, err := convert.MessageToBlockHeader(blockRes.GetBlock())
	if err != nil {
		return nil, err
	}

	payload := flowgo.NewEmptyPayload()
	return &flowgo.Block{
		Payload:    *payload,
		HeaderBody: header.HeaderBody,
	}, nil
}

func (s *Store) LedgerByHeight(
	ctx context.Context,
	blockHeight uint64,
) (snapshot.StorageSnapshot, error) {
	// Create a snapshot with LRU cache to avoid duplicate RPC calls within the same snapshot
	// LRU cache with max 1000 entries to prevent memory bloat
	cache, err := lru.New[string, flowgo.RegisterValue](1000)
	if err != nil {
		return nil, fmt.Errorf("failed to create LRU cache: %w", err)
	}

	return snapshot.NewReadFuncStorageSnapshot(func(id flowgo.RegisterID) (flowgo.RegisterValue, error) {
		// Check LRU cache first to avoid duplicate RPC calls within this snapshot
		if cachedValue, exists := cache.Get(id.String()); exists {
			return cachedValue, nil
		}
		// create a copy so updating it doesn't affect future calls
		lookupHeight := blockHeight

		// first try to see if we have local stored ledger
		value, err := s.DefaultStore.GetBytesAtVersion(
			ctx,
			s.Storage(storage.LedgerStoreName),
			[]byte(id.String()),
			lookupHeight,
		)
		if err == nil && value != nil {
			return value, nil
		}

		// FVM expects an empty byte array if the value is not found
		value = []byte{}

		// if we don't have it, get it from the rpc host
		// for consistency, always use data at the forked height for future blocks
		if lookupHeight > s.forkHeight {
			lookupHeight = s.forkHeight
		}

		registerID := convert.RegisterIDToMessage(flowgo.RegisterID{Key: id.Key, Owner: id.Owner})
		var response *executiondata.GetRegisterValuesResponse

		err = s.retryWithBackoff(ctx, "GetRegisterValues", func() error {
			response, err = s.executionClient.GetRegisterValues(ctx, &executiondata.GetRegisterValuesRequest{
				BlockHeight: lookupHeight,
				RegisterIds: []*entities.RegisterID{registerID},
			})
			return err
		})

		if err != nil {
			if status.Code(err) != codes.NotFound {
				return nil, err
			}
		}

		if response != nil && len(response.Values) > 0 {
			value = response.Values[0]
		}

		// cache the value for future use
		err = s.DataSetter.SetBytesWithVersion(
			ctx,
			s.Storage(storage.LedgerStoreName),
			[]byte(id.String()),
			value,
			lookupHeight)
		if err != nil {
			return nil, fmt.Errorf("could not cache ledger value: %w", err)
		}

		// Cache in LRU cache for this snapshot to avoid duplicate RPC calls
		cache.Add(id.String(), value)
		return value, nil
	}), nil
}
func (s *Store) Stop() {
	_ = s.grpcConn.Close()
}
