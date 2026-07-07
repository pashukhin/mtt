GO      ?= go
BIN     := bin/mtt
PKGS    := ./...
VERSION ?=

ifneq ($(VERSION),)
LDFLAGS := -ldflags "-X github.com/pashukhin/mtt/internal/cli.version=$(VERSION)"
endif

.PHONY: all build install smoke test fmt fmt-check vet lint check tidy clean

all: build

build:
	$(GO) build $(LDFLAGS) -o $(BIN) ./cmd/mtt

install:
	$(GO) install $(LDFLAGS) ./cmd/mtt

# smoke: install into a throwaway GOBIN and check the binary runs. Not part of
# `check` (it does a real go install; check stays hermetic).
smoke:
	@tmp=$$(mktemp -d); \
	trap 'rm -rf "$$tmp"' EXIT; \
	GOBIN=$$tmp $(GO) install $(LDFLAGS) ./cmd/mtt; \
	v=$$("$$tmp/mtt" version); \
	if [ -z "$$v" ]; then echo "smoke: empty version output"; exit 1; fi; \
	"$$tmp/mtt" --help >/dev/null; \
	echo "OK: smoke (mtt version = $$v)"

test:
	$(GO) test -race -cover $(PKGS)

fmt:
	gofmt -w .
	goimports -w -local github.com/pashukhin/mtt .

fmt-check:
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then \
		echo "gofmt required for:"; echo "$$out"; exit 1; fi

vet:
	$(GO) vet $(PKGS)

lint:
	golangci-lint run

check: fmt-check vet lint test build
	@echo "OK: make check passed"

tidy:
	$(GO) mod tidy

clean:
	rm -rf bin
