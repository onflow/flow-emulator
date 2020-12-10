module github.com/onflow/flow-emulator

go 1.13

require (
	github.com/davecgh/go-spew v1.1.1
	github.com/dgraph-io/badger/v2 v2.0.3
	github.com/fxamacker/cbor/v2 v2.2.1-0.20201006223149-25f67fca9803
	github.com/golang/mock v1.4.4
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/improbable-eng/grpc-web v0.12.0
	github.com/logrusorgru/aurora v0.0.0-20200102142835-e9ef32dff381
	github.com/onflow/cadence v0.10.2
	// this references: https://github.com/onflow/flow-go/tree/feature/multiple-contract-support.
	github.com/onflow/flow-go v0.12.6
	github.com/onflow/flow-go-sdk v0.12.2
	github.com/onflow/flow-go/crypto v0.11.1-0.20201124074740-4553dbb0bc38
	github.com/onflow/flow/protobuf/go/flow v0.1.8
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.5.1
	github.com/prometheus/common v0.9.1
	github.com/psiemens/graceland v1.0.0
	github.com/psiemens/sconfig v0.0.0-20190623041652-6e01eb1354fc
	github.com/rs/zerolog v1.19.0
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/cobra v0.0.6
	github.com/stretchr/testify v1.6.1
	google.golang.org/grpc v1.31.1
)

replace github.com/fxamacker/cbor/v2 => github.com/turbolent/cbor/v2 v2.2.1-0.20200911003300-cac23af49154
