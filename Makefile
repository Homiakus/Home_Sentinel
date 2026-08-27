GREMLINS_VERSION ?= v0.6.0
TOOLS_DIR ?= .tools
ARTIFACTS_DIR ?= .artifacts
ENGLOOP := go run ./cmd/sentinel-engloop

.PHONY: fmt vet test test-race check engloop-reconcile engloop-strict engloop-gates edge-suite gremlins-install mutation-diff

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

vet:
	go vet ./...

test:
	go test ./...

test-race:
	go test -race ./...

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
	$(ENGLOOP) reconcile --root .
