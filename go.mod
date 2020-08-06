module github.com/dapperlabs/flow-emulator

go 1.13

require (
	github.com/dapperlabs/flow-go v0.4.1-0.20200811000347-954202e2f254
	github.com/dapperlabs/flow-go/crypto v0.3.2-0.20200708192840-30b3e2d5a586
	github.com/davecgh/go-spew v1.1.1
	github.com/dgraph-io/badger/v2 v2.0.3
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/go-metrics v0.0.1 // indirect
	github.com/fxamacker/cbor/v2 v2.2.0
	github.com/golang/mock v1.4.3
	github.com/golang/protobuf v1.4.0
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/improbable-eng/grpc-web v0.12.0
	github.com/logrusorgru/aurora v0.0.0-20200102142835-e9ef32dff381
	github.com/onflow/cadence v0.8.0
	github.com/onflow/flow-go-sdk v0.9.0
	github.com/onflow/flow/protobuf/go/flow v0.1.5-0.20200806013300-5011e9d6a292
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.5.1
	github.com/prometheus/common v0.9.1
	github.com/psiemens/graceland v1.0.0
	github.com/psiemens/sconfig v0.0.0-20190623041652-6e01eb1354fc
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/cobra v0.0.6
	github.com/stretchr/testify v1.6.1
	golang.org/x/sys v0.0.0-20200509044756-6aff5f38e54f // indirect
	google.golang.org/grpc v1.28.0
)

replace github.com/dapperlabs/flow-go => ../flow-go
