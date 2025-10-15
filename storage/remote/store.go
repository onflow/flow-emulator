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
	"encoding/hex"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/fvm/environment"
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

		// Only record failures for rate limit errors
		if isRateLimitError(err) {
			s.circuitBreaker.recordFailure()
		}

		// Check if this is a rate limit error
		if isRateLimitError(err) {
			s.logger.Info().
				Str("operation", operation).
				Msg("Rate limit detected, will retry with backoff")
		}

		// For all errors (including rate limits), continue with retry logic
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
	// COMPATIBILITY SHIM: Account Key Deduplication Migration
	// TODO: Remove after Flow release - this shim provides backward compatibility
	// for pre-migration networks by synthesizing migrated registers from legacy data
	applyKeyDeduplication bool
}

type Option func(*Store)

// WithForkURL configures the remote access/observer node gRPC endpoint.
// Expects raw host:port with no scheme.
func WithForkURL(url string) Option {
	return func(store *Store) {
		// enforce raw host:port only
		if strings.Contains(url, "://") {
			// keep as-is; the New() will error when dialing
		}
		store.host = url
	}
}

// WithRPCHost sets access/observer node host. Deprecated: use WithForkURL.
func WithRPCHost(host string, chainID flowgo.ChainID) Option {
	return func(store *Store) {
		// Keep legacy behavior: set host and (optionally) chainID for validation.
		store.host = host
		store.chainID = chainID
	}
}

// WithStartBlockHeight sets the start height for the store.
// WithForkBlockNumber sets the pinned fork block/height.
func WithForkBlockNumber(height uint64) Option {
	return func(store *Store) {
		store.forkHeight = height
	}
}

// WithStartBlockHeight is deprecated: use WithForkBlockNumber.
func WithStartBlockHeight(height uint64) Option { return WithForkBlockNumber(height) }

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

	// COMPATIBILITY SHIM: Enable for all networks
	// TODO: Remove after Forte release - provides backward compatibility
	store.applyKeyDeduplication = true

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

		// COMPATIBILITY SHIM: Account Key Deduplication Migration
		// TODO: Remove after Flow release - synthesizes migrated registers from legacy data
		if s.applyKeyDeduplication {
			normalizedKey := normalizeKey(id.Key)

			// COMPATIBILITY SHIM: Synthesize migrated registers from legacy data
			// TODO: Remove after Flow release - these functions provide backward compatibility
			if isAPK0Key(normalizedKey) && len(value) == 0 {
				// Fallback apk_0 -> public_key_0
				legacy, err := s.fetchRemoteRegister(ctx, id.Owner, "public_key_0", lookupHeight)
				if err != nil {
					return nil, err
				}
				if len(legacy) > 0 {
					value = legacy
				}
			} else if isPKBKey(normalizedKey) && len(value) == 0 {
				// Synthesize pk_b<batchIndex> from individual public_key_* registers
				batchIdx, ok := parsePKBBatchIndex(normalizedKey)
				if ok {
					synthesized, err := s.synthesizeBatchPublicKeys(ctx, id.Owner, batchIdx, lookupHeight)
					if err != nil {
						return nil, err
					}
					if len(synthesized) > 0 {
						value = synthesized
					}
				}
			} else if isSNKey(normalizedKey) && len(value) == 0 {
				// Synthesize sn_<keyIndex> from public_key_<keyIndex> sequence numbers
				keyIdx, ok := parseSNKeyIndex(normalizedKey)
				if ok {
					synthesized, err := s.synthesizeSequenceNumber(ctx, id.Owner, keyIdx, lookupHeight)
					if err != nil {
						return nil, err
					}
					if len(synthesized) > 0 {
						value = synthesized
					}
				}
			} else if isAccountStatusKey(normalizedKey) {
				// Synthesize account status v4 with key metadata from legacy registers
				synthesized, err := s.synthesizeAccountStatusV4(ctx, id.Owner, lookupHeight)
				if err != nil {
					return nil, err
				}
				if len(synthesized) > 0 {
					value = synthesized
				}
			}
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

// COMPATIBILITY SHIM: Helper functions for account key deduplication migration
// TODO: Remove after Flow release - these functions provide backward compatibility

// normalizeKey decodes a hex-prefixed ("#<hex>") key into its string form; otherwise returns the key unchanged.
func normalizeKey(key string) string {
	if strings.HasPrefix(key, "#") {
		hexPart := key[1:]
		if b, err := hex.DecodeString(hexPart); err == nil {
			return string(b)
		}
	}
	return key
}

func isAPK0Key(key string) bool {
	return key == flowgo.AccountPublicKey0RegisterKey
}

func isPKBKey(key string) bool {
	return strings.HasPrefix(key, flowgo.BatchPublicKeyRegisterKeyPrefix)
}

func isSNKey(key string) bool {
	return strings.HasPrefix(key, flowgo.SequenceNumberRegisterKeyPrefix)
}

func isAccountStatusKey(key string) bool {
	return key == flowgo.AccountStatusKey
}

func parsePKBBatchIndex(key string) (uint32, bool) {
	if !isPKBKey(key) {
		return 0, false
	}
	suffix := strings.TrimPrefix(key, flowgo.BatchPublicKeyRegisterKeyPrefix)
	n, err := strconv.ParseUint(suffix, 10, 32)
	if err != nil {
		return 0, false
	}
	return uint32(n), true
}

func parseSNKeyIndex(key string) (uint32, bool) {
	if !isSNKey(key) {
		return 0, false
	}
	suffix := strings.TrimPrefix(key, flowgo.SequenceNumberRegisterKeyPrefix)
	n, err := strconv.ParseUint(suffix, 10, 32)
	if err != nil {
		return 0, false
	}
	return uint32(n), true
}

// fetchRemoteRegister fetches a single register value from the remote at a fixed height.
func (s *Store) fetchRemoteRegister(ctx context.Context, owner string, key string, height uint64) ([]byte, error) {
	registerID := convert.RegisterIDToMessage(flowgo.RegisterID{Key: key, Owner: owner})
	response, err := s.executionClient.GetRegisterValues(ctx, &executiondata.GetRegisterValuesRequest{
		BlockHeight: height,
		RegisterIds: []*entities.RegisterID{registerID},
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, nil
		}
		return nil, err
	}
	if response != nil && len(response.Values) > 0 {
		return response.Values[0], nil
	}
	return nil, nil
}

// COMPATIBILITY SHIM: Batch register fetching to reduce RPC calls
// TODO: Remove after Flow release - batches multiple register lookups into single RPC call
func (s *Store) fetchRemoteRegisters(ctx context.Context, owner string, keys []string, height uint64) (map[string][]byte, error) {
	if len(keys) == 0 {
		return make(map[string][]byte), nil
	}

	registerIDs := make([]*entities.RegisterID, len(keys))
	for i, key := range keys {
		registerIDs[i] = convert.RegisterIDToMessage(flowgo.RegisterID{Key: key, Owner: owner})
	}

	var response *executiondata.GetRegisterValuesResponse
	err := s.retryWithBackoff(ctx, "GetRegisterValuesBatch", func() error {
		var err error
		response, err = s.executionClient.GetRegisterValues(ctx, &executiondata.GetRegisterValuesRequest{
			BlockHeight: height,
			RegisterIds: registerIDs,
		})
		return err
	})

	if err != nil {
		if status.Code(err) == codes.NotFound {
			return make(map[string][]byte), nil
		}
		return nil, err
	}

	result := make(map[string][]byte)
	if response != nil && len(response.Values) > 0 {
		for i, key := range keys {
			if i < len(response.Values) && len(response.Values[i]) > 0 {
				result[key] = response.Values[i]
			}
		}
	}
	return result, nil
}

// COMPATIBILITY SHIM: Batch public key synthesis
// TODO: Remove after Flow release - builds batch public key payloads from individual public_key_* registers
func (s *Store) synthesizeBatchPublicKeys(ctx context.Context, owner string, batchIndex uint32, height uint64) ([]byte, error) {
	// Load account status to get key count
	statusBytes, err := s.fetchRemoteRegister(ctx, owner, flowgo.AccountStatusKey, height)
	if err != nil {
		return nil, err
	}
	if len(statusBytes) == 0 {
		return nil, nil
	}
	status, err := environment.AccountStatusFromBytes(statusBytes)
	if err != nil {
		return nil, fmt.Errorf("could not parse account status: %w", err)
	}
	count := status.AccountPublicKeyCount()
	if count == 0 {
		return nil, nil
	}

	const max = environment.MaxPublicKeyCountInBatch
	start := batchIndex * max
	// storedKeyIndex range is [start, min(start+max-1, count-1)]
	if start >= count {
		return nil, nil
	}
	end := start + max - 1
	if end > count-1 {
		end = count - 1
	}

	batch := make([]byte, 0, 1+(end-start+1)*8) // rough capacity

	// Batch 0 reserves index 0 as nil placeholder to align indices
	if batchIndex == 0 {
		batch = append(batch, 0x00)
	}

	// Batch fetch all legacy public key registers for this batch to reduce RPC calls
	legacyKeys := make([]string, 0, end-start+1)
	for i := start; i <= end; i++ {
		if i == 0 {
			continue // skip key 0
		}
		legacyKeys = append(legacyKeys, fmt.Sprintf("public_key_%d", i))
	}

	legacyValues, err := s.fetchRemoteRegisters(ctx, owner, legacyKeys, height)
	if err != nil {
		return nil, err
	}

	for i := start; i <= end; i++ {
		if i == 0 {
			// stored key 0 is apk_0 and not included in batch payload (nil placeholder already added)
			continue
		}
		legacyKey := fmt.Sprintf("public_key_%d", i)
		legacyVal := legacyValues[legacyKey]

		if len(legacyVal) == 0 {
			// keep index alignment with zero-length entry
			batch = append(batch, 0x00)
			continue
		}

		// Decode legacy account public key to extract public material
		decoded, err := flowgo.DecodeAccountPublicKey(legacyVal, uint32(i))
		if err != nil {
			// cannot decode -> keep alignment with zero-length entry
			batch = append(batch, 0x00)
			continue
		}
		stored := flowgo.StoredPublicKey{
			PublicKey: decoded.PublicKey,
			SignAlgo:  decoded.SignAlgo,
			HashAlgo:  decoded.HashAlgo,
		}
		enc, err := flowgo.EncodeStoredPublicKey(stored)
		if err != nil {
			batch = append(batch, 0x00)
			continue
		}
		if len(enc) > 255 {
			// out of spec for batch encoding; skip with placeholder
			batch = append(batch, 0x00)
			continue
		}
		batch = append(batch, byte(len(enc)))
		batch = append(batch, enc...)
	}

	return batch, nil
}

// COMPATIBILITY SHIM: Sequence number synthesis
// TODO: Remove after Flow release - builds sequence number registers from legacy public_key_* registers
func (s *Store) synthesizeSequenceNumber(ctx context.Context, owner string, keyIndex uint32, height uint64) ([]byte, error) {
	if keyIndex == 0 {
		// key 0 sequence number lives in apk_0
		return nil, nil
	}
	legacyKey := fmt.Sprintf("public_key_%d", keyIndex)
	legacyVal, err := s.fetchRemoteRegister(ctx, owner, legacyKey, height)
	if err != nil {
		return nil, err
	}
	if len(legacyVal) == 0 {
		return nil, nil
	}
	decoded, err := flowgo.DecodeAccountPublicKey(legacyVal, keyIndex)
	if err != nil {
		return nil, nil
	}
	if decoded.SeqNumber == 0 {
		return nil, nil
	}
	enc, err := flowgo.EncodeSequenceNumber(decoded.SeqNumber)
	if err != nil {
		return nil, nil
	}
	return enc, nil
}

// COMPATIBILITY SHIM: Account status v4 synthesis
// TODO: Remove after Flow release - synthesizes v4 account status with key metadata from legacy registers
func (s *Store) synthesizeAccountStatusV4(ctx context.Context, owner string, height uint64) ([]byte, error) {
	// Load existing account status (v3)
	statusBytes, err := s.fetchRemoteRegister(ctx, owner, flowgo.AccountStatusKey, height)
	if err != nil {
		return nil, err
	}
	if len(statusBytes) == 0 {
		return nil, nil
	}

	status, err := environment.AccountStatusFromBytes(statusBytes)
	if err != nil {
		return nil, fmt.Errorf("could not parse account status: %w", err)
	}

	count := status.AccountPublicKeyCount()
	if count <= 1 {
		// No key metadata needed for accounts with 0-1 keys
		return statusBytes, nil
	}

	// Build key metadata from legacy registers
	keyMetadata, err := s.buildKeyMetadataFromLegacy(ctx, owner, count, height)
	if err != nil {
		return nil, fmt.Errorf("could not build key metadata: %w", err)
	}

	// Create new status with key metadata by parsing the original bytes and appending metadata
	// The AccountStatus struct has unexported fields, so we work with the byte representation
	originalBytes := status.ToBytes()

	// Set account status v4 flag (0x40 = accountStatusV4WithNoDeduplicationFlag)
	if len(originalBytes) > 0 {
		originalBytes[0] = 0x40
	}

	// Append key metadata to the original account status bytes
	newBytes := make([]byte, len(originalBytes)+len(keyMetadata))
	copy(newBytes, originalBytes)
	copy(newBytes[len(originalBytes):], keyMetadata)

	return newBytes, nil
}

// COMPATIBILITY SHIM: Key metadata construction
// TODO: Remove after Flow release - builds key metadata from legacy public_key_* registers
func (s *Store) buildKeyMetadataFromLegacy(ctx context.Context, owner string, count uint32, height uint64) ([]byte, error) {
	// For pre-migration networks, we build minimal key metadata:
	// - Weight and revoked status for keys 1..count-1 (RLE encoded)
	// - No deduplication mappings (storedKeyIndex == keyIndex)
	// - No digests (not needed for basic functionality)

	if count <= 1 {
		return nil, nil
	}

	// Batch fetch all legacy public key registers to reduce RPC calls
	legacyKeys := make([]string, count-1)
	for i := uint32(1); i < count; i++ {
		legacyKeys[i-1] = fmt.Sprintf("public_key_%d", i)
	}

	legacyValues, err := s.fetchRemoteRegisters(ctx, owner, legacyKeys, height)
	if err != nil {
		return nil, err
	}

	// Build weight and revoked status for keys 1..count-1
	var weightAndRevoked []byte

	for i := uint32(1); i < count; i++ {
		legacyKey := fmt.Sprintf("public_key_%d", i)
		legacyVal := legacyValues[legacyKey]

		if len(legacyVal) == 0 {
			// Default values for missing keys
			weightAndRevoked = append(weightAndRevoked, 0, 0) // weight=0, revoked=false
			continue
		}

		decoded, err := flowgo.DecodeAccountPublicKey(legacyVal, i)
		if err != nil {
			// Default values for unparseable keys
			weightAndRevoked = append(weightAndRevoked, 0, 0)
			continue
		}

		// Encode weight and revoked status (RLE format)
		weight := uint16(decoded.Weight)
		if weight > 1000 {
			weight = 1000 // clamp to max
		}

		revokedFlag := uint16(0)
		if decoded.Revoked {
			revokedFlag = 0x8000 // high bit set
		}

		weightAndRevoked = append(weightAndRevoked, byte(weight>>8), byte(weight&0xFF))
		weightAndRevoked = append(weightAndRevoked, byte(revokedFlag>>8), byte(revokedFlag&0xFF))
	}

	// Build minimal key metadata:
	// - Length-prefixed weight and revoked status
	// - Start index for digests (0, no digests)
	// - Length-prefixed empty digests

	metadata := make([]byte, 0, 4+len(weightAndRevoked)+4+4)

	// Length-prefixed weight and revoked status
	metadata = append(metadata, byte(len(weightAndRevoked)>>24), byte(len(weightAndRevoked)>>16), byte(len(weightAndRevoked)>>8), byte(len(weightAndRevoked)))
	metadata = append(metadata, weightAndRevoked...)

	// Start index for digests (0)
	metadata = append(metadata, 0, 0, 0, 0)

	// Length-prefixed empty digests
	metadata = append(metadata, 0, 0, 0, 0)

	return metadata, nil
}
