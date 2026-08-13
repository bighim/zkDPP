PINNED_GO := $(abspath $(CURDIR)/../poc-v2/.cache/gomod/golang.org/toolchain@v0.0.1-go1.25.7.darwin-arm64/bin/go)
GO ?= $(if $(wildcard $(PINNED_GO)),$(PINNED_GO),go)
GO_ENV = GOCACHE=$(CURDIR)/.cache/go-build GOMODCACHE=$(abspath $(CURDIR)/../poc-v2/.cache/gomod) GOTOOLCHAIN=local
RUNS := 1
ANVIL_PORT ?= 18545
BESU_PORT ?= 9545

.PHONY: conformance setup test-go test-vectors benchmark-gadgets event-fixtures test-contract test-e2e \
	benchmark-anvil benchmark-anvil-e2e benchmark-besu benchmark-besu-e2e benchmark-meta \
	compare-poc-v2 report validate-results benchmark-all reproduce

conformance:
	$(GO_ENV) $(GO) run ./cmd/conformance

setup:
	$(GO_ENV) $(GO) run ./cmd/setup -profile all -gadget all

test-go:
	$(GO_ENV) $(GO) test ./...

test-vectors:
	$(GO_ENV) $(GO) run ./cmd/testvectors

benchmark-gadgets:
	$(GO_ENV) $(GO) run ./cmd/benchmark-gadgets -profile poseidon2 -threads 1 -runs 1
	$(GO_ENV) $(GO) run ./cmd/benchmark-gadgets -profile poseidon2 -threads 8 -runs 1
	$(GO_ENV) $(GO) run ./cmd/benchmark-gadgets -profile sha256 -threads 1 -runs 1
	$(GO_ENV) $(GO) run ./cmd/benchmark-gadgets -profile sha256 -threads 8 -runs 1

event-fixtures:
	$(GO_ENV) $(GO) run ./cmd/benchmark-payload -profile poseidon2 -scenario independent -threads 8 -runs 1
	$(GO_ENV) $(GO) run ./cmd/benchmark-payload -profile poseidon2 -scenario canonical -threads 8 -runs 1
	$(GO_ENV) $(GO) run ./cmd/benchmark-payload -profile sha256 -scenario independent -threads 8 -runs 1
	$(GO_ENV) $(GO) run ./cmd/benchmark-payload -profile sha256 -scenario canonical -threads 8 -runs 1

test-contract:
	docker compose run --rm foundry forge test -vv

test-e2e: conformance test-go test-vectors test-contract

benchmark-anvil:
	ANVIL_PORT=$(ANVIL_PORT) RUNS=1 ./scripts/benchmark-anvil.sh

benchmark-anvil-e2e:
	ANVIL_PORT=$(ANVIL_PORT) RUNS=1 ./scripts/benchmark-anvil-e2e.sh

benchmark-besu:
	BESU_PORT=$(BESU_PORT) RUNS=1 ./scripts/benchmark-besu.sh

benchmark-besu-e2e:
	BESU_PORT=$(BESU_PORT) RUNS=1 ./scripts/benchmark-besu-e2e.sh

benchmark-meta:
	$(GO_ENV) $(GO) run ./cmd/benchmarkmeta -root . -out benchmarks/environment.json

compare-poc-v2:
	$(GO_ENV) $(GO) run ./cmd/compare -root .

benchmark-all: benchmark-gadgets event-fixtures benchmark-anvil benchmark-anvil-e2e benchmark-besu benchmark-besu-e2e benchmark-meta compare-poc-v2

report:
	$(GO_ENV) $(GO) run ./cmd/report -root . -out "Sahai POC Evaluation Report.tex"

validate-results:
	$(GO_ENV) $(GO) run ./cmd/validate-results -root .

reproduce: setup test-e2e benchmark-all report validate-results
