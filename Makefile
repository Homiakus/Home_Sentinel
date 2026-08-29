GREMLINS_VERSION := v0.6.0
GOVULNCHECK_VERSION := v1.7.0
CYCLONEDX_GOMOD_VERSION := v1.10.0
TOOLS_DIR ?= .tools
ARTIFACTS_DIR ?= .artifacts
ENGLOOP := go run ./cmd/sentinel-engloop
SUPPLYCHAIN := go run ./cmd/sentinel-supplychain
SENTINELCTL := go run ./cmd/sentinelctl

.PHONY: fmt vet test test-race check engloop-reconcile engloop-strict engloop-gates edge-suite supply-chain govulncheck-install govulncheck sbom-install sbom gremlins-install mutation-diff manage setup doctor build image start stop restart status stack-config stack-up stack-down stack-restart stack-status stack-pull

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

vet:
	go vet ./...

test:
	go test ./...

test-race:
	go test -race ./...

manage:
	$(SENTINELCTL) menu

setup:
	$(SENTINELCTL) setup

doctor:
	$(SENTINELCTL) doctor

build:
	$(SENTINELCTL) build

image:
	$(SENTINELCTL) image

start:
	$(SENTINELCTL) start

stop:
	$(SENTINELCTL) stop

restart:
	$(SENTINELCTL) restart

status:
	$(SENTINELCTL) status

stack-config:
	$(SENTINELCTL) stack-config

stack-up:
	$(SENTINELCTL) stack-up

stack-down:
	$(SENTINELCTL) stack-down

stack-restart:
	$(SENTINELCTL) stack-restart

stack-status:
	$(SENTINELCTL) stack-status

stack-pull:
	$(SENTINELCTL) stack-pull

engloop-reconcile:
	$(ENGLOOP) reconcile --root .

engloop-strict:
	$(ENGLOOP) reconcile --root . --strict

engloop-gates:
	@test -n "$(CHANGED_FILE)" || (echo 'CHANGED_FILE is required' && exit 2)
	$(ENGLOOP) gates --changed-file "$(CHANGED_FILE)"

edge-suite:
	@test -n "$(MODEL)" || (echo 'MODEL is required, e.g. docs/testing/edge-model.example.json' && exit 2)
	$(ENGLOOP) edge --file "$(MODEL)"

supply-chain:
	$(SUPPLYCHAIN) --root .

govulncheck-install:
	mkdir -p "$(TOOLS_DIR)"
	GOBIN="$(CURDIR)/$(TOOLS_DIR)" go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)

govulncheck: govulncheck-install
	"$(TOOLS_DIR)/govulncheck" ./...

sbom-install:
	mkdir -p "$(TOOLS_DIR)"
	GOBIN="$(CURDIR)/$(TOOLS_DIR)" go install github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@$(CYCLONEDX_GOMOD_VERSION)

sbom: sbom-install
	mkdir -p "$(ARTIFACTS_DIR)"
	"$(TOOLS_DIR)/cyclonedx-gomod" mod -json -output "$(ARTIFACTS_DIR)/go-mod.bom.json" .

gremlins-install:
	mkdir -p "$(TOOLS_DIR)"
	GOBIN="$(CURDIR)/$(TOOLS_DIR)" go install github.com/go-gremlins/gremlins/cmd/gremlins@$(GREMLINS_VERSION)

mutation-diff: gremlins-install
	@test -n "$(BASE)" || (echo 'BASE is required, e.g. origin/main or HEAD^' && exit 2)
	mkdir -p "$(ARTIFACTS_DIR)"
	"$(TOOLS_DIR)/gremlins" unleash --diff "$(BASE)" --output "$(ARTIFACTS_DIR)/gremlins.json" --output-statuses lc
	$(ENGLOOP) mutation --file "$(ARTIFACTS_DIR)/gremlins.json"

check:
	@test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './vendor/*'))" || (echo 'gofmt required' && gofmt -l $$(find . -name '*.go' -not -path './vendor/*') && exit 1)
	go vet ./...
	go test ./...
	$(SUPPLYCHAIN) --root .
	$(ENGLOOP) reconcile --root .
