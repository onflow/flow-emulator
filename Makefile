# The short Git commit hash
SHORT_COMMIT := $(shell git rev-parse --short HEAD)
# Name of the cover profile
COVER_PROFILE := cover.out
# Disable go sum database lookup for private repos
GOPRIVATE := github.com/dapperlabs/*
# Ensure go bin path is in path (Especially for CI)
PATH := $(PATH):$(GOPATH)/bin
# OS
UNAME := $(shell uname)

GOPATH ?= $(HOME)/go

.PHONY: install-tools
install-tools:
	mkdir -p ${GOPATH}; \
	cd ${GOPATH}; \
	GO111MODULE=on go install go.uber.org/mock/mockgen@latest; \
	GO111MODULE=on go install github.com/axw/gocov/gocov@latest; \
	GO111MODULE=on go install github.com/matm/gocov-html@latest; \
	GO111MODULE=on go install github.com/sanderhahn/gozip/cmd/gozip@latest;

.PHONY: test
test:
	GO111MODULE=on go test -coverprofile=$(COVER_PROFILE) $(if $(JSON_OUTPUT),-json,) ./...

.PHONY: run
run:
	GO111MODULE=on go run ./cmd/emulator

.PHONY: coverage
coverage:
ifeq ($(COVER), true)
	# file has to be called index.html
	gocov convert $(COVER_PROFILE) > cover.json
	./cover-summary.sh
	gocov-html cover.json > index.html
	# coverage.zip will automatically be picked up by teamcity
	gozip -c coverage.zip index.html
endif

.PHONY: generate
generate: generate-mocks

.PHONY: generate-mocks
generate-mocks:
	GO111MODULE=on ${GOPATH}/bin/mockgen -destination=emulator/mocks/emulator.go -package=mocks github.com/onflow/flow-emulator/emulator Emulator
	GO111MODULE=on ${GOPATH}/bin/mockgen -destination=storage/mocks/store.go -package=mocks github.com/onflow/flow-emulator/storage Store
	GO111MODULE=on ${GOPATH}/bin/mockgen -destination=internal/mocks/access.go -package=mocks github.com/onflow/flow-emulator/internal AccessAPIClient
	GO111MODULE=on ${GOPATH}/bin/mockgen -destination=internal/mocks/executiondata.go -package=mocks github.com/onflow/flow-emulator/internal ExecutionDataAPIClient

.PHONY: ci
ci: install-tools test check-tidy test coverage check-headers

.PHONY: install-linter
install-linter:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.4.0

.PHONY: lint
lint:
	golangci-lint run -v ./...

.PHONY: fix-lint
fix-lint:
	golangci-lint run -v --fix ./...


.PHONY: check-headers
check-headers:
	@./check-headers.sh

.PHONY: check-tidy
check-tidy: generate
	go mod tidy
	git diff --exit-code
