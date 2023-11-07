/*
 * Flow Emulator
 *
 * Copyright 2019 Dapper Labs, Inc.
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
	"os"
	"testing"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-emulator/adapters"
	"github.com/onflow/flow-emulator/storage/sqlite"

	archive "github.com/onflow/flow-archive/api/archive/client"
	"github.com/onflow/flow-archive/codec/zbor"
	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/model/encoding/rlp"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	emulator "github.com/onflow/flow-emulator/emulator"
)

func BytesToRegister(b []byte) (*flowgo.RegisterID, error) {
	var decoded flowgo.RegisterID
	unmarshaler := rlp.NewMarshaler()
	err := unmarshaler.Unmarshal(b, &decoded)
	if err != nil {
		return nil, fmt.Errorf("could not decode register ID: %w", err)
	}
	return &decoded, nil
}

var _ archive.APIClient = testClient{}

const testHeight = uint64(53115699)

type testClient struct {
	registerMap map[string][]byte
	header      []byte
}

// newTestClient implements the archive client interface.
//
// The response data is obtained from fixture files which we created by
// observing a real client usage. This data should be update once in a while
// and this can be done by adding a simple observer to the real client call and
// serializing the response to the files.
func newTestClient() (*testClient, error) {
	encoded, err := os.ReadFile("storage_registers_fixture")
	if err != nil {
		return nil, err
	}

	var regMap map[string][]byte
	err = zbor.NewCodec().Decode(encoded, &regMap)
	if err != nil {
		return nil, err
	}

	header, err := os.ReadFile("storage_header_fixture")
	if err != nil {
		return nil, err
	}

	return &testClient{
		registerMap: regMap,
		header:      header,
	}, nil
}

func (a testClient) GetFirst(ctx context.Context, in *archive.GetFirstRequest, opts ...grpc.CallOption) (*archive.GetFirstResponse, error) {
	panic("Not needed")
}

func (a testClient) GetLast(ctx context.Context, in *archive.GetLastRequest, opts ...grpc.CallOption) (*archive.GetLastResponse, error) {
	return &archive.GetLastResponse{Height: testHeight}, nil // a random height
}

func (a testClient) GetHeightForBlock(ctx context.Context, in *archive.GetHeightForBlockRequest, opts ...grpc.CallOption) (*archive.GetHeightForBlockResponse, error) {
	panic("Not needed")
}

func (a testClient) GetCommit(ctx context.Context, in *archive.GetCommitRequest, opts ...grpc.CallOption) (*archive.GetCommitResponse, error) {
	panic("Not needed")
}

func (a testClient) GetHeader(ctx context.Context, in *archive.GetHeaderRequest, opts ...grpc.CallOption) (*archive.GetHeaderResponse, error) {
	return &archive.GetHeaderResponse{
		Height: testHeight,
		Data:   a.header,
	}, nil
}

func (a testClient) GetEvents(ctx context.Context, in *archive.GetEventsRequest, opts ...grpc.CallOption) (*archive.GetEventsResponse, error) {
	panic("Not needed")
}

func (a testClient) GetRegisterValues(ctx context.Context, in *archive.GetRegisterValuesRequest, opts ...grpc.CallOption) (*archive.GetRegisterValuesResponse, error) {
	for key := range a.registerMap {
		fmt.Println(key)
	}

	regBytes := in.Registers[0]
	register, _ := BytesToRegister(regBytes)
	key := convert.RegisterIDToLedgerKey(*register)
	path, _ := pathfinder.KeyToPath(key, 1)
	val, ok := a.registerMap[hex.EncodeToString(path[:])]

	if !ok {
		return nil, fmt.Errorf("register not found in test fixture")
	}

	return &archive.GetRegisterValuesResponse{
		Values: [][]byte{val},
	}, nil
}

func (a testClient) GetCollection(ctx context.Context, in *archive.GetCollectionRequest, opts ...grpc.CallOption) (*archive.GetCollectionResponse, error) {
	panic("Not needed")
}

func (a testClient) ListCollectionsForHeight(ctx context.Context, in *archive.ListCollectionsForHeightRequest, opts ...grpc.CallOption) (*archive.ListCollectionsForHeightResponse, error) {
	panic("Not needed")
}

func (a testClient) GetGuarantee(ctx context.Context, in *archive.GetGuaranteeRequest, opts ...grpc.CallOption) (*archive.GetGuaranteeResponse, error) {
	panic("Not needed")
}

func (a testClient) GetTransaction(ctx context.Context, in *archive.GetTransactionRequest, opts ...grpc.CallOption) (*archive.GetTransactionResponse, error) {
	panic("Not needed")
}

func (a testClient) GetHeightForTransaction(ctx context.Context, in *archive.GetHeightForTransactionRequest, opts ...grpc.CallOption) (*archive.GetHeightForTransactionResponse, error) {
	panic("Not needed")
}

func (a testClient) ListTransactionsForHeight(ctx context.Context, in *archive.ListTransactionsForHeightRequest, opts ...grpc.CallOption) (*archive.ListTransactionsForHeightResponse, error) {
	panic("Not needed")
}

func (a testClient) GetResult(ctx context.Context, in *archive.GetResultRequest, opts ...grpc.CallOption) (*archive.GetResultResponse, error) {
	panic("Not needed")
}

func (a testClient) GetSeal(ctx context.Context, in *archive.GetSealRequest, opts ...grpc.CallOption) (*archive.GetSealResponse, error) {
	panic("Not needed")
}

func (a testClient) ListSealsForHeight(ctx context.Context, in *archive.ListSealsForHeightRequest, opts ...grpc.CallOption) (*archive.ListSealsForHeightResponse, error) {
	panic("Not needed")
}

func Test_SimulatedMainnetTransaction(t *testing.T) {
	t.Parallel()

	client, err := newTestClient()
	require.NoError(t, err)

	provider, err := sqlite.New(sqlite.InMemory)
	require.NoError(t, err)

	remoteStore, err := New(provider, WithClient(client))
	require.NoError(t, err)

	b, err := emulator.New(
		emulator.WithStore(remoteStore),
		emulator.WithStorageLimitEnabled(false),
		emulator.WithTransactionValidationEnabled(false),
		emulator.WithChainID(flowgo.Mainnet),
	)
	logger := zerolog.Nop()
	adapter := adapters.NewSDKAdapter(&logger, b)
	require.NoError(t, err)

	script := []byte(`
		import Ping from 0x9799f28ff0453528
		
		transaction {
			execute {
				Ping.echo()
			}
		}
	`)
	addr := flowsdk.HexToAddress("0x9799f28ff0453528")
	tx := flowsdk.NewTransaction().
		SetScript(script).
		SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(addr, 0, 0).
		SetPayer(addr)

	err = adapter.SendTransaction(context.Background(), *tx)
	require.NoError(t, err)

	txRes, err := b.ExecuteNextTransaction()
	require.NoError(t, err)

	_, err = b.CommitBlock()
	require.NoError(t, err)

	assert.NoError(t, txRes.Error)

	require.Len(t, txRes.Events, 1)
	assert.Equal(t, txRes.Events[0].String(), "A.9799f28ff0453528.Ping.PingEmitted: 0x953f6f26d61710cb0e140bfde1022483b9ef410ddd181bac287d9968c84f4778")
	assert.Equal(t, txRes.Events[0].Value.String(), `A.9799f28ff0453528.Ping.PingEmitted(sound: "ping ping ping")`)
}

func Test_SimulatedMainnetTransactionWithChanges(t *testing.T) {
	t.Parallel()
	client, err := newTestClient()
	require.NoError(t, err)

	provider, err := sqlite.New(sqlite.InMemory)
	require.NoError(t, err)

	remoteStore, err := New(provider, WithClient(client))
	require.NoError(t, err)

	b, err := emulator.New(
		emulator.WithStore(remoteStore),
		emulator.WithStorageLimitEnabled(false),
		emulator.WithTransactionValidationEnabled(false),
		emulator.WithChainID(flowgo.Mainnet),
	)
	require.NoError(t, err)

	logger := zerolog.Nop()
	adapter := adapters.NewSDKAdapter(&logger, b)

	script := []byte(`
		import Ping from 0x9799f28ff0453528
		
		transaction {
			execute {
				Ping.sound = "pong pong pong"
			}
		}
	`)
	addr := flowsdk.HexToAddress("0x9799f28ff0453528")
	tx := flowsdk.NewTransaction().
		SetScript(script).
		SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(addr, 0, 0).
		SetPayer(addr)

	err = adapter.SendTransaction(context.Background(), *tx)
	require.NoError(t, err)

	txRes, err := b.ExecuteNextTransaction()
	require.NoError(t, err)
	require.NoError(t, txRes.Error)

	_, err = b.CommitBlock()
	require.NoError(t, err)

	script = []byte(`
		import Ping from 0x9799f28ff0453528
		
		transaction {
			execute {
				Ping.echo()
			}
		}
	`)
	tx = flowsdk.NewTransaction().
		SetScript(script).
		SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(addr, 0, 0).
		SetPayer(addr)

	err = adapter.SendTransaction(context.Background(), *tx)
	require.NoError(t, err)

	txRes, err = b.ExecuteNextTransaction()
	require.NoError(t, err)

	_, err = b.CommitBlock()
	require.NoError(t, err)

	assert.NoError(t, txRes.Error)

	require.Len(t, txRes.Events, 1)
	assert.Equal(t, txRes.Events[0].String(), "A.9799f28ff0453528.Ping.PingEmitted: 0x953f6f26d61710cb0e140bfde1022483b9ef410ddd181bac287d9968c84f4778")
	assert.Equal(t, txRes.Events[0].Value.String(), `A.9799f28ff0453528.Ping.PingEmitted(sound: "pong pong pong")`)
}
