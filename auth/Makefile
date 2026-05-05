VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X github.com/portflare/auth/internal/buildinfo.Version=$(VERSION) \
	-X github.com/portflare/auth/internal/buildinfo.Commit=$(COMMIT) \
	-X github.com/portflare/auth/internal/buildinfo.Date=$(BUILD_DATE)

.PHONY: test build

test:
	go test ./...

build:
	go build -ldflags '$(LDFLAGS)' -o dist/bin/portflare-auth ./cmd/portflare-auth
