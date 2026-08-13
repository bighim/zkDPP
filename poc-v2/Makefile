GO ?= go
GO_ENV = GOCACHE=$(CURDIR)/.cache/go-build GOMODCACHE=$(CURDIR)/.cache/gomod GOTOOLCHAIN=go1.25.7
RUNS ?= 1
ANVIL_PORT ?= 8545

.PHONY: test-go generate-reference generate-poseidon setup test-contract test-e2e benchmark benchmark-anvil benchmark-anvil-gas benchmark-anvil-e2e benchmark-process3x3

test-go:
	$(GO_ENV) $(GO) test ./...

generate-reference:
	$(GO_ENV) $(GO) run ./cmd/genreference

generate-poseidon:
	$(GO_ENV) $(GO) run ./cmd/genposeidon

setup: generate-reference generate-poseidon
	$(GO_ENV) $(GO) run ./cmd/setup -relation all

test-contract: setup
	docker compose run --rm foundry forge test -vv

test-e2e: test-go setup test-contract

benchmark:
	$(GO_ENV) $(GO) run ./cmd/benchmark -profile all

benchmark-anvil: benchmark-anvil-gas benchmark-anvil-e2e

benchmark-anvil-gas: setup
	ANVIL_PORT=$(ANVIL_PORT) ./scripts/benchmark-anvil-gas.sh

benchmark-anvil-e2e: setup
	ANVIL_PORT=$(ANVIL_PORT) RUNS=$(RUNS) ./scripts/benchmark-anvil-e2e.sh

benchmark-process3x3:
	$(GO_ENV) $(GO) run ./cmd/benchmark -profile process-s3-3to3
