GO ?= go
GO_ENV = GOCACHE=$(CURDIR)/.cache/go-build GOTOOLCHAIN=go1.25.7

.PHONY: test-go setup-m1 evaluate-m1 setup-m2 test-contract-m2 benchmark-m2-gas benchmark-m2-e2e benchmark-m2 setup-m3 test-contract-m3 benchmark-m3-gas benchmark-m3-e2e benchmark-m3 setup-m4 test-contract-m4 benchmark-m4-gas benchmark-m4-e2e benchmark-m4 setup-m5 test-contract-m5 benchmark-m5-gas benchmark-m5-e2e benchmark-m5-verifier-ablation benchmark-m5 setup-m6 test-contract-m6 benchmark-m6-gas benchmark-m6-e2e benchmark-m6

test-go:
	$(GO_ENV) $(GO) test ./...

setup-m1:
	$(GO_ENV) GOMAXPROCS=8 $(GO) run ./cmd/setup_m1_v2

evaluate-m1:
	$(GO_ENV) GOMAXPROCS=8 $(GO) run ./cmd/evaluate_m1_v2

.PHONY: test-contract-m1 benchmark-m1-key-recovery benchmark-m1-decrypt benchmark-m1-anvil benchmark-m1-audit benchmark-m1 check-m1

test-contract-m1:
	FOUNDRY_IMAGE=ghcr.io/foundry-rs/foundry:v1.7.1 docker compose -p zkdpp-v2-m1 run --rm foundry sh -lc 'cd m1 && forge test -vv'

benchmark-m1-key-recovery:
	$(GO_ENV) GOMAXPROCS=8 $(GO) run ./cmd/benchmark_m1_v2 -mode key-recovery

benchmark-m1-decrypt:
	$(GO_ENV) GOMAXPROCS=8 $(GO) run ./cmd/benchmark_m1_v2 -mode decrypt

benchmark-m1-anvil:
	$(GO_ENV) GOMAXPROCS=8 $(GO) run ./cmd/benchmark_m1_v2 -mode anvil

benchmark-m1-audit:
	$(GO_ENV) GOMAXPROCS=8 $(GO) run ./cmd/benchmark_m1_v2 -mode audit
	ANVIL_PORT=18546 FOUNDRY_IMAGE=ghcr.io/foundry-rs/foundry:v1.7.1 docker compose -p zkdpp-v2-m1 up -d anvil
	$(GO_ENV) GOMAXPROCS=8 $(GO) run ./cmd/benchmark_m1_rpc
	ANVIL_PORT=18546 FOUNDRY_IMAGE=ghcr.io/foundry-rs/foundry:v1.7.1 docker compose -p zkdpp-v2-m1 down

benchmark-m1: benchmark-m1-key-recovery benchmark-m1-decrypt benchmark-m1-anvil benchmark-m1-audit

check-m1:
	$(GO_ENV) $(GO) run ./cmd/benchmark_m1_v2 -mode checks

setup-m2:
	$(GO_ENV) $(GO) run ./cmd/setup_m2

test-contract-m2:
	docker compose run --rm foundry forge test -vv

benchmark-m2-gas:
	$(GO_ENV) $(GO) run ./cmd/benchmark_m2 -mode gas

benchmark-m2-e2e:
	$(GO_ENV) $(GO) run ./cmd/benchmark_m2 -mode e2e

benchmark-m2: benchmark-m2-gas benchmark-m2-e2e

setup-m3:
	$(GO_ENV) $(GO) run ./cmd/setup_m3

test-contract-m3:
	docker compose run --rm foundry forge test -vv

benchmark-m3-gas:
	$(GO_ENV) $(GO) run ./cmd/benchmark_m3 -mode gas

benchmark-m3-e2e:
	$(GO_ENV) $(GO) run ./cmd/benchmark_m3 -mode e2e

benchmark-m3: benchmark-m3-gas benchmark-m3-e2e

setup-m4:
	$(GO_ENV) $(GO) run ./cmd/setup_m4

test-contract-m4:
	docker compose run --rm foundry forge test -vv

benchmark-m4-gas:
	$(GO_ENV) $(GO) run ./cmd/benchmark_m4 -mode gas

benchmark-m4-e2e:
	$(GO_ENV) $(GO) run ./cmd/benchmark_m4 -mode e2e

benchmark-m4: benchmark-m4-gas benchmark-m4-e2e

setup-m5:
	$(GO_ENV) $(GO) run ./cmd/setup_m5

test-contract-m5:
	docker compose run --rm foundry forge test -vv

benchmark-m5-gas:
	$(GO_ENV) $(GO) run ./cmd/benchmark_m5 -mode gas

benchmark-m5-e2e:
	$(GO_ENV) $(GO) run ./cmd/benchmark_m5 -mode e2e

benchmark-m5-verifier-ablation:
	$(GO_ENV) $(GO) run ./cmd/benchmark_m5 -mode ablation

benchmark-m5: benchmark-m5-gas benchmark-m5-e2e benchmark-m5-verifier-ablation

setup-m6:
	$(GO_ENV) $(GO) run ./cmd/setup_m6

test-contract-m6:
	docker compose run --rm foundry forge test -vv

benchmark-m6-gas:
	$(GO_ENV) $(GO) run ./cmd/benchmark_m6 -mode gas

benchmark-m6-e2e:
	$(GO_ENV) $(GO) run ./cmd/benchmark_m6 -mode e2e

benchmark-m6: benchmark-m6-gas benchmark-m6-e2e

.PHONY: setup-m6-b1 evaluate-m6-b1 test-contract-m6-b1 benchmark-m6-b1-gas benchmark-m6-b1-decrypt benchmark-m6-b1 check-m6-b1

setup-m6-b1:
	$(GO_ENV) GOMAXPROCS=8 $(GO) run ./cmd/setup_m6_b1

evaluate-m6-b1:
	$(GO_ENV) GOMAXPROCS=8 $(GO) run ./cmd/evaluate_m6_b1

test-contract-m6-b1:
	FOUNDRY_IMAGE=ghcr.io/foundry-rs/foundry:v1.7.1 docker compose -p zkdpp-m6-b1 run --rm foundry forge test -vv

benchmark-m6-b1-gas:
	$(GO_ENV) GOMAXPROCS=8 $(GO) run ./cmd/benchmark_m6_b1 -mode gas

benchmark-m6-b1-decrypt:
	$(GO_ENV) GOMAXPROCS=8 $(GO) run ./cmd/benchmark_m6_b1 -mode decrypt

benchmark-m6-b1: benchmark-m6-b1-gas
	$(MAKE) benchmark-m6-b1-decrypt

check-m6-b1:
	$(GO_ENV) $(GO) run ./cmd/benchmark_m6_b1 -mode checks

.PHONY: setup-m7 evaluate-m7 test-contract-m7 benchmark-m7-gas benchmark-m7-audit benchmark-m7 check-m7

setup-m7:
	$(GO_ENV) GOMAXPROCS=8 $(GO) run ./cmd/setup_m7

evaluate-m7:
	$(GO_ENV) GOMAXPROCS=8 $(GO) run ./cmd/evaluate_m7

test-contract-m7:
	FOUNDRY_IMAGE=ghcr.io/foundry-rs/foundry:v1.7.1 docker compose -p zkdpp-m7 run --rm foundry forge test -vv

benchmark-m7-gas:
	$(GO_ENV) GOMAXPROCS=8 $(GO) run ./cmd/benchmark_m7 -mode gas

benchmark-m7-audit:
	$(GO_ENV) GOMAXPROCS=8 $(GO) run ./cmd/benchmark_m7 -mode audit

benchmark-m7: benchmark-m7-gas
	$(MAKE) benchmark-m7-audit

check-m7:
	$(GO_ENV) $(GO) run ./cmd/benchmark_m7 -mode checks

.PHONY: setup-m8 evaluate-m8 test-contract-m8 benchmark-m8-gas benchmark-m8-audit benchmark-m8 check-m8

setup-m8:
	$(GO_ENV) GOMAXPROCS=8 $(GO) run ./cmd/setup_m8

evaluate-m8:
	$(GO_ENV) GOMAXPROCS=8 $(GO) run ./cmd/evaluate_m8

test-contract-m8:
	FOUNDRY_IMAGE=ghcr.io/foundry-rs/foundry:v1.7.1 docker compose -p zkdpp-m8 run --rm foundry forge test -vv

benchmark-m8-gas:
	$(GO_ENV) GOMAXPROCS=8 $(GO) run ./cmd/benchmark_m8 -mode gas

benchmark-m8-audit:
	$(GO_ENV) GOMAXPROCS=8 $(GO) run ./cmd/benchmark_m8 -mode audit

benchmark-m8: benchmark-m8-gas
	$(MAKE) benchmark-m8-audit

check-m8:
	$(GO_ENV) $(GO) run ./cmd/benchmark_m8 -mode checks

.PHONY: setup-m9-final evaluate-m9 test-contract-m9 benchmark-m9-setup benchmark-m9-anvil benchmark-m9-audit benchmark-m9 check-m9

setup-m9-final:
	$(GO_ENV) GOMAXPROCS=8 $(GO) run ./cmd/setup_m9_final

evaluate-m9:
	$(GO_ENV) GOMAXPROCS=8 $(GO) run ./cmd/evaluate_m9

test-contract-m9:
	FOUNDRY_IMAGE=ghcr.io/foundry-rs/foundry:v1.7.1 docker compose -p zkdpp-m9 run --rm foundry forge test -vv

benchmark-m9-setup:
	$(GO_ENV) GOMAXPROCS=8 $(GO) run ./cmd/benchmark_m9 -mode setup

benchmark-m9-anvil:
	$(GO_ENV) GOMAXPROCS=8 $(GO) run ./cmd/benchmark_m9 -mode anvil

benchmark-m9-audit:
	$(GO_ENV) GOMAXPROCS=8 $(GO) run ./cmd/benchmark_m9 -mode audit

benchmark-m9: benchmark-m9-setup benchmark-m9-anvil benchmark-m9-audit

check-m9:
	$(GO_ENV) $(GO) run ./cmd/benchmark_m9 -mode checks
