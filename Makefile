GO      ?= go
BIN     := bin/mtt
PKGS    := ./...

.PHONY: all build install test fmt fmt-check vet lint check tidy clean

all: build

build:
	$(GO) build -o $(BIN) ./cmd/mtt

install:
	$(GO) install ./cmd/mtt

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
