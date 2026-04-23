GO ?= go

.PHONY: check fmt fmt-check test vet run

check: fmt-check vet test

fmt:
	gofmt -w $$(find . -type f -name '*.go')

fmt-check:
	@test -z "$$(gofmt -l $$(find . -type f -name '*.go'))" || \
		(echo "gofmt reported unformatted files:" && gofmt -l $$(find . -type f -name '*.go') && exit 1)

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

run:
	OPENAI_MOCK_DEBUG=1 $(GO) run ./cmd/openai-mock
