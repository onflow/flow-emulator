package handler

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client/convert"
	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/dapperlabs/flow-emulator/server/backend"
)

type Handler struct {
	backend *backend.Backend
}

func New(backend *backend.Backend) *Handler {
	return &Handler{
		backend: backend,
	}
}

// Ping the Access API server for a response.
func (h *Handler) Ping(ctx context.Context, req *access.PingRequest) (*access.PingResponse, error) {
	return &access.PingResponse{}, nil
}

func (h *Handler) GetNetworkParameters(
	context.Context,
	*access.GetNetworkParametersRequest,
) (*access.GetNetworkParametersResponse, error) {
	panic("implement me")
}

// SendTransaction submits a transaction to the network.
func (h *Handler) SendTransaction(
	ctx context.Context,
	req *access.SendTransactionRequest,
) (*access.SendTransactionResponse, error) {
	txMsg := req.GetTransaction()

	tx, err := convert.MessageToTransaction(txMsg)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	err = h.backend.SendTransaction(ctx, tx)
	if err != nil {
		return nil, err
	}

	return &access.SendTransactionResponse{
		Id: tx.ID().Bytes(),
	}, nil
}

// GetLatestBlockHeader gets the latest sealed block header.
func (h *Handler) GetLatestBlockHeader(
	ctx context.Context,
	req *access.GetLatestBlockHeaderRequest,
) (*access.BlockHeaderResponse, error) {
	block, err := h.backend.GetLatestBlockHeader(ctx)
	if err != nil {
		return nil, err
	}

	return blockToHeaderResponse(block)
}

// GetBlockHeaderByHeight gets a block header by height.
func (h *Handler) GetBlockHeaderByHeight(
	ctx context.Context,
	req *access.GetBlockHeaderByHeightRequest,
) (*access.BlockHeaderResponse, error) {
	block, err := h.backend.GetBlockHeaderByHeight(ctx, req.GetHeight())
	if err != nil {
		return nil, err
	}

	return blockToHeaderResponse(block)
}

// GetBlockHeaderByID gets a block header by ID.
func (h *Handler) GetBlockHeaderByID(
	ctx context.Context,
	req *access.GetBlockHeaderByIDRequest,
) (*access.BlockHeaderResponse, error) {
	blockID := sdk.HashToID(req.GetId())

	block, err := h.backend.GetBlockByID(ctx, blockID)
	if err != nil {
		return nil, err
	}

	return blockToHeaderResponse(block)
}

// GetLatestBlock gets the latest sealed block.
func (h *Handler) GetLatestBlock(
	ctx context.Context,
	req *access.GetLatestBlockRequest,
) (*access.BlockResponse, error) {
	block, err := h.backend.GetLatestBlock(ctx)
	if err != nil {
		return nil, err
	}

	return blockResponse(block)
}

// GetBlockByHeight gets a block by height.
func (h *Handler) GetBlockByHeight(
	ctx context.Context,
	req *access.GetBlockByHeightRequest,
) (*access.BlockResponse, error) {
	block, err := h.backend.GetBlockByHeight(ctx, req.GetHeight())
	if err != nil {
		return nil, err
	}

	return blockResponse(block)
}

// GetBlockByHeight gets a block by ID.
func (h *Handler) GetBlockByID(
	ctx context.Context,
	req *access.GetBlockByIDRequest,
) (*access.BlockResponse, error) {
	blockID := sdk.HashToID(req.GetId())

	block, err := h.backend.GetBlockByID(ctx, blockID)
	if err != nil {
		return nil, err
	}

	return blockResponse(block)
}

// GetCollectionByID gets a collection by ID.
func (h *Handler) GetCollectionByID(
	ctx context.Context,
	req *access.GetCollectionByIDRequest,
) (*access.CollectionResponse, error) {
	id := sdk.HashToID(req.GetId())

	col, err := h.backend.GetCollectionByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return &access.CollectionResponse{
		Collection: convert.CollectionToMessage(*col),
	}, nil
}

// GetTransaction gets a transaction by ID.
func (h *Handler) GetTransaction(
	ctx context.Context,
	req *access.GetTransactionRequest,
) (*access.TransactionResponse, error) {
	id := sdk.HashToID(req.GetId())

	tx, err := h.backend.GetTransaction(ctx, id)
	if err != nil {
		return nil, err
	}

	txMsg, err := convert.TransactionToMessage(*tx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &access.TransactionResponse{
		Transaction: txMsg,
	}, nil
}

// GetTransactionResult gets a transaction by ID.
func (h *Handler) GetTransactionResult(
	ctx context.Context,
	req *access.GetTransactionRequest,
) (*access.TransactionResultResponse, error) {
	id := sdk.HashToID(req.GetId())

	result, err := h.backend.GetTransactionResult(ctx, id)
	if err != nil {
		return nil, err
	}

	res, err := convert.TransactionResultToMessage(*result)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return res, nil
}

// GetAccount returns an account by address at the latest sealed block.
func (h *Handler) GetAccount(
	ctx context.Context,
	req *access.GetAccountRequest,
) (*access.GetAccountResponse, error) {
	address := sdk.BytesToAddress(req.GetAddress())

	account, err := h.backend.GetAccount(ctx, address)
	if err != nil {
		return nil, err
	}

	return &access.GetAccountResponse{
		Account: convert.AccountToMessage(*account),
	}, nil
}

// GetAccountAtLatestBlock returns an account by address at the latest sealed block.
func (h *Handler) GetAccountAtLatestBlock(
	ctx context.Context,
	req *access.GetAccountAtLatestBlockRequest,
) (*access.AccountResponse, error) {
	address := sdk.BytesToAddress(req.GetAddress())

	account, err := h.backend.GetAccountAtLatestBlock(ctx, address)
	if err != nil {
		return nil, err
	}

	return &access.AccountResponse{
		Account: convert.AccountToMessage(*account),
	}, nil
}

func (h *Handler) GetAccountAtBlockHeight(
	ctx context.Context,
	request *access.GetAccountAtBlockHeightRequest,
) (*access.AccountResponse, error) {
	panic("implement me")
}

// ExecuteScriptAtLatestBlock executes a script at a the latest block
func (h *Handler) ExecuteScriptAtLatestBlock(
	ctx context.Context,
	req *access.ExecuteScriptAtLatestBlockRequest,
) (*access.ExecuteScriptResponse, error) {
	script := req.GetScript()
	arguments := req.GetArguments()

	value, err := h.backend.ExecuteScriptAtLatestBlock(ctx, script, arguments)
	if err != nil {
		return nil, err
	}

	return &access.ExecuteScriptResponse{
		Value: value,
	}, nil
}

// ExecuteScriptAtBlockHeight executes a script at a specific block height
func (h *Handler) ExecuteScriptAtBlockHeight(
	ctx context.Context,
	req *access.ExecuteScriptAtBlockHeightRequest,
) (*access.ExecuteScriptResponse, error) {
	script := req.GetScript()
	arguments := req.GetArguments()
	blockHeight := req.GetBlockHeight()

	value, err := h.backend.ExecuteScriptAtBlockHeight(ctx, script, arguments, blockHeight)
	if err != nil {
		return nil, err
	}

	return &access.ExecuteScriptResponse{
		Value: value,
	}, nil
}

// ExecuteScriptAtBlockID executes a script at a specific block ID
func (h *Handler) ExecuteScriptAtBlockID(
	ctx context.Context,
	req *access.ExecuteScriptAtBlockIDRequest,
) (*access.ExecuteScriptResponse, error) {
	script := req.GetScript()
	arguments := req.GetArguments()
	blockID := sdk.HashToID(req.GetBlockId())

	value, err := h.backend.ExecuteScriptAtBlockID(ctx, script, arguments, blockID)
	if err != nil {
		return nil, err
	}

	return &access.ExecuteScriptResponse{
		Value: value,
	}, nil
}

// GetEventsForHeightRange returns events matching a query.
func (h *Handler) GetEventsForHeightRange(
	ctx context.Context,
	req *access.GetEventsForHeightRangeRequest,
) (*access.EventsResponse, error) {
	eventType := req.GetType()
	startHeight := req.GetStartHeight()
	endHeight := req.GetEndHeight()

	results, err := h.backend.GetEventsForHeightRange(ctx, eventType, startHeight, endHeight)
	if err != nil {
		return nil, err
	}

	resultMessages, err := blockEventsToMessages(results)
	if err != nil {
		return nil, err
	}

	return &access.EventsResponse{
		Results: resultMessages,
	}, nil
}

// GetEventsForBlockIDs returns events matching a set of block IDs.
func (h *Handler) GetEventsForBlockIDs(
	ctx context.Context,
	req *access.GetEventsForBlockIDsRequest,
) (*access.EventsResponse, error) {
	eventType := req.GetType()
	blockIDs := convert.MessagesToIdentifiers(req.GetBlockIds())

	results, err := h.backend.GetEventsForBlockIDs(ctx, eventType, blockIDs)
	if err != nil {
		return nil, err
	}

	resultMessages, err := blockEventsToMessages(results)
	if err != nil {
		return nil, err
	}

	return &access.EventsResponse{
		Results: resultMessages,
	}, nil
}

// blockToHeaderResponse constructs a block header response from a block.
func blockToHeaderResponse(block *sdk.Block) (*access.BlockHeaderResponse, error) {
	msg, err := convert.BlockHeaderToMessage(*&block.BlockHeader)
	if err != nil {
		return nil, err
	}

	return &access.BlockHeaderResponse{
		Block: msg,
	}, nil
}

// blockResponse constructs a block response from a block.
func blockResponse(block *sdk.Block) (*access.BlockResponse, error) {
	msg, err := convert.BlockToMessage(*block)
	if err != nil {
		return nil, err
	}

	return &access.BlockResponse{
		Block: msg,
	}, nil
}

func blockEventsToMessages(blocks []backend.BlockEvents) (results []*access.EventsResponse_Result, err error) {
	results = make([]*access.EventsResponse_Result, len(blocks))

	for i, block := range blocks {
		result, err := blockEventsToMessage(block)
		if err != nil {
			return nil, err
		}

		results[i] = result
	}

	return results, nil
}

func blockEventsToMessage(block backend.BlockEvents) (result *access.EventsResponse_Result, err error) {
	eventMessages := make([]*entities.Event, len(block.Events))
	for i, event := range block.Events {
		eventMessages[i], err = convert.EventToMessage(event)
		if err != nil {
			return nil, err
		}
	}

	return &access.EventsResponse_Result{
		BlockId:     block.Block.ID[:],
		BlockHeight: block.Block.Height,
		Events:      eventMessages,
	}, nil
}
