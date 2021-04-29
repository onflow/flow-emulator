module github.com/onflow/flow-emulator

go 1.13

require (
	github.com/dgraph-io/badger/v2 v2.0.3
	github.com/fxamacker/cbor/v2 v2.2.1-0.20201006223149-25f67fca9803
	github.com/golang/mock v1.4.4
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/improbable-eng/grpc-web v0.12.0
	github.com/logrusorgru/aurora v0.0.0-20200102142835-e9ef32dff381
	github.com/onflow/cadence v0.15.1
	github.com/onflow/flow-go v0.16.3-0.20210427194927-6050c2a3ae42 // https://github.com/onflow/flow-go/tree/v0.16 commit: https://github.com/onflow/flow-go/commit/6050c2a3ae42d6333ffcfd79d187f56a1011fc35
	github.com/onflow/flow-go-sdk v0.18.0
	github.com/onflow/flow-go/crypto v0.12.0
	github.com/onflow/flow/protobuf/go/flow v0.2.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.7.1
	github.com/prometheus/common v0.14.0
	github.com/psiemens/graceland v1.0.0
	github.com/psiemens/sconfig v0.0.0-20190623041652-6e01eb1354fc
	github.com/rs/zerolog v1.19.0
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/cobra v0.0.6
	github.com/stretchr/testify v1.7.0
	google.golang.org/grpc v1.31.1
)
