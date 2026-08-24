.PHONY: fmt vet test test-race check

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

vet:
	go vet ./...

test:
	go test ./...

test-race:
	go test -race ./...

check:
	@test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './vendor/*'))" || (echo 'gofmt required' && gofmt -l $$(find . -name '*.go' -not -path './vendor/*') && exit 1)
	go vet ./...
	go test ./...
