package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	flowsdk "github.com/onflow/flow-go-sdk"
	flowgo "github.com/onflow/flow-go/model/flow"
	flowaccess "github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/flow-emulator/convert"
	"github.com/onflow/flow-emulator/utils"
)

// TestForkWithPersist verifies that forking with --persist actually caches registers to SQLite
// and can resume from the cached state
func TestForkWithPersist(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()

	const target = "access.testnet.nodes.onflow.org:9000"

	// Get remote latest sealed height to pin fork
	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		utils.DefaultGRPCRetryInterceptor(),
	)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	remote := flowaccess.NewAccessAPIClient(conn)

	rh, err := remote.GetLatestBlockHeader(context.Background(), &flowaccess.GetLatestBlockHeaderRequest{IsSealed: true})
	require.NoError(t, err)
	remoteHeight := rh.Block.Height - 10

	// Create temp directory for DB
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	cfg := &Config{
		DBPath:                    dbPath,
		Persist:                   true, // Enable persist mode
		Snapshot:                  false,
		SkipTransactionValidation: true,
		ChainID:                   flowgo.Testnet,
		ForkHost:                  target,
		ForkHeight:                remoteHeight,
	}

	// First run: write to storage
	srv := NewEmulatorServer(&logger, cfg)
	require.NotNil(t, srv)

	latest, err := srv.Emulator().GetLatestBlock()
	require.NoError(t, err)
	require.Equal(t, remoteHeight+1, latest.Height)

	sk := srv.Emulator().ServiceKey()

	// Write an Int into account storage
	writeScript := []byte(`
        transaction {
            prepare(acct: auth(Storage) &Account) {
                acct.storage.save<Int>(42, to: /storage/persistTest)
            }
        }
    `)
	tx := flowsdk.NewTransaction().
		SetScript(writeScript).
		SetReferenceBlockID(flowsdk.Identifier(latest.ID())).
		SetProposalKey(flowsdk.Address(sk.Address), sk.Index, sk.SequenceNumber).
		SetPayer(flowsdk.Address(sk.Address)).
		AddAuthorizer(flowsdk.Address(sk.Address))

	signer, err := sk.Signer()
	require.NoError(t, err)

	err = tx.SignEnvelope(flowsdk.Address(sk.Address), sk.Index, signer)
	require.NoError(t, err)

	err = srv.Emulator().AddTransaction(*convert.SDKTransactionToFlow(*tx))
	require.NoError(t, err)

	_, results, err := srv.Emulator().ExecuteAndCommitBlock()
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.True(t, results[0].Succeeded())

	t.Log("First run: Wrote value to storage")

	// Stop emulator (simulates restart)
	srv.Stop()

	// Verify SQLite file exists and has data
	info, err := os.Stat(dbPath)
	require.NoError(t, err)
	require.True(t, info.Size() > 0, "SQLite DB should have data")
	t.Logf("SQLite DB size: %d bytes", info.Size())

	// Second run: restart with same DB and read from cache
	cfg2 := &Config{
		DBPath:                    dbPath, // Same DB!
		Persist:                   true,
		Snapshot:                  false,
		SkipTransactionValidation: true,
		ChainID:                   flowgo.Testnet,
		ForkHost:                  target,
		ForkHeight:                remoteHeight,
	}

	srv2 := NewEmulatorServer(&logger, cfg2)
	require.NotNil(t, srv2)

	latest2, err := srv2.Emulator().GetLatestBlock()
	require.NoError(t, err)

	sk2 := srv2.Emulator().ServiceKey()

	// Read the value we wrote in the previous run
	readScript := []byte(`
		transaction {
			prepare(acct: auth(Storage) &Account) {
				let value = acct.storage.borrow<&Int>(from: /storage/persistTest)
				log(value != nil && *value == 42)
			}
		}
	`)
	readTx := flowsdk.NewTransaction().
		SetScript(readScript).
		SetReferenceBlockID(flowsdk.Identifier(latest2.ID())).
		SetProposalKey(flowsdk.Address(sk2.Address), sk2.Index, sk2.SequenceNumber).
		SetPayer(flowsdk.Address(sk2.Address)).
		AddAuthorizer(flowsdk.Address(sk2.Address))

	signer2, err := sk2.Signer()
	require.NoError(t, err)

	err = readTx.SignEnvelope(flowsdk.Address(sk2.Address), sk2.Index, signer2)
	require.NoError(t, err)

	err = srv2.Emulator().AddTransaction(*convert.SDKTransactionToFlow(*readTx))
	require.NoError(t, err)

	_, readResults, err := srv2.Emulator().ExecuteAndCommitBlock()
	require.NoError(t, err)

	require.Len(t, readResults, 1)
	require.True(t, readResults[0].Succeeded())

	logs := readResults[0].Logs
	found := false
	for _, l := range logs {
		if l == "true" {
			found = true
			break
		}
	}
	require.True(t, found, "Should have found stored value from SQLite cache, logs: %v", logs)

	t.Log("Second run: Successfully read value from SQLite cache!")
}
