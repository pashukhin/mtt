GO      ?= go
BIN     := bin/mtt
PKGS    := ./...
VERSION ?=

# Dev builds derive the version from git when VERSION is not passed; release must
# always be stamped with an explicit VERSION (see the release target's guard).
BUILD_VERSION := $(if $(VERSION),$(VERSION),$(shell git describe --tags --always --dirty 2>/dev/null))
BUILD_LDFLAGS := $(if $(BUILD_VERSION),-ldflags "-X github.com/pashukhin/mtt/internal/cli.version=$(BUILD_VERSION)")
RELEASE_LDFLAGS := -ldflags "-X github.com/pashukhin/mtt/internal/cli.version=$(VERSION)"

DIST      := dist
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.PHONY: all build install smoke release test fmt fmt-check vet lint check tidy clean

all: build

build:
	$(GO) build $(BUILD_LDFLAGS) -o $(BIN) ./cmd/mtt

install:
	$(GO) install $(BUILD_LDFLAGS) ./cmd/mtt

# smoke: install into a throwaway GOBIN and check the binary runs. Not part of
# `check` (it does a real go install; check stays hermetic).
smoke:
	@set -e; \
	tmp=$$(mktemp -d); \
	trap 'rm -rf "$$tmp"' EXIT; \
	GOBIN=$$tmp $(GO) install $(BUILD_LDFLAGS) ./cmd/mtt; \
	v=$$("$$tmp/mtt" version); \
	if [ -z "$$v" ]; then echo "smoke: empty version output"; exit 1; fi; \
	"$$tmp/mtt" --help >/dev/null; \
	echo "OK: smoke (mtt version = $$v)"

# release: cross-compile version-stamped binaries for every target platform into
# dist/ + a SHA256SUMS. Requires VERSION (a release must be stamped). Pure Go (no
# cgo) so cross-compilation is a plain GOOS/GOARCH build. Not part of `check`
# (cross-compiles + writes dist/, non-hermetic — like smoke).
release:
	@if [ -z "$(VERSION)" ]; then \
		echo "release: VERSION is required (e.g. make release VERSION=v0.9.0)"; exit 1; fi
	@rm -rf $(DIST); mkdir -p $(DIST)
	@set -e; for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		out="$(DIST)/mtt_$(VERSION)_$${os}_$${arch}"; \
		if [ "$$os" = "windows" ]; then out="$$out.exe"; fi; \
		echo "building $$out"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 $(GO) build $(RELEASE_LDFLAGS) -o "$$out" ./cmd/mtt; \
	done
	@cd $(DIST) && sha256sum mtt_* > SHA256SUMS
	@echo "OK: release $(VERSION) -> $(DIST)/ ($$(ls $(DIST) | grep -c '^mtt_') binaries)"

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
