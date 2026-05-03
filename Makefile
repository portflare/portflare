.PHONY: help website-serve test test-protocol test-auth test-server test-client build build-auth build-server build-client docker-build docker-build-auth docker-build-server docker-build-client status push push-root push-protocol push-auth push-server push-client

GO_IMAGE ?= golang:1.23
WEBSITE_PORT ?= 8080

help:
	@echo "Portflare workspace targets"
	@echo ""
	@echo "Website:"
	@echo "  make website-serve        Serve ./website on http://127.0.0.1:$${WEBSITE_PORT:-$(WEBSITE_PORT)}"
	@echo ""
	@echo "Go tests via Docker:"
	@echo "  make test                 Run protocol, auth, server, and client tests"
	@echo "  make test-protocol        Run protocol tests"
	@echo "  make test-auth            Run auth tests"
	@echo "  make test-server          Run server tests"
	@echo "  make test-client          Run client tests"
	@echo ""
	@echo "Go builds via Docker:"
	@echo "  make build                Build auth, server, and client"
	@echo "  make build-auth           Build auth service"
	@echo "  make build-server         Build server"
	@echo "  make build-client         Build client"
	@echo ""
	@echo "Docker images:"
	@echo "  make docker-build         Build auth, server, and client images"
	@echo "  make docker-build-auth    Build auth image"
	@echo "  make docker-build-server  Build server image"
	@echo "  make docker-build-client  Build client image"
	@echo ""
	@echo "Utilities:"
	@echo "  make status               Show git status for workspace repos"
	@echo "  make push                 Push root, protocol, auth, server, and client repos"

website-serve:
	python3 -m http.server $(WEBSITE_PORT) -d website

test: test-protocol test-auth test-server test-client

test-protocol:
	docker run --rm -v "$(CURDIR)/protocol:/src" -w /src $(GO_IMAGE) go test ./...

test-auth:
	docker run --rm -v "$(CURDIR)/auth:/src" -w /src $(GO_IMAGE) go test ./...

test-server:
	docker run --rm -v "$(CURDIR)/server:/src" -w /src $(GO_IMAGE) go test ./...

test-client:
	docker run --rm -v "$(CURDIR)/client:/src" -w /src $(GO_IMAGE) go test ./...

build: build-auth build-server build-client

build-auth:
	docker run --rm -v "$(CURDIR)/auth:/src" -w /src $(GO_IMAGE) go build ./cmd/portflare-auth

build-server:
	docker run --rm -v "$(CURDIR)/server:/src" -w /src $(GO_IMAGE) go build ./cmd/portflare-server

build-client:
	docker run --rm -v "$(CURDIR)/client:/src" -w /src $(GO_IMAGE) go build ./cmd/portflare

docker-build: docker-build-auth docker-build-server docker-build-client

docker-build-auth:
	@if [ -f auth/Dockerfile ]; then \
		docker build -t portflare-auth:dev auth; \
	else \
		echo "auth/Dockerfile not found; skipping auth image"; \
	fi

docker-build-server:
	docker build -t portflare-server:dev server

docker-build-client:
	docker build -t portflare-client:dev client

status:
	@echo "== root ==" && git status --short
	@echo "== protocol ==" && git -C protocol status --short
	@echo "== auth ==" && git -C auth status --short
	@echo "== server ==" && git -C server status --short
	@echo "== client ==" && git -C client status --short

push: push-root push-protocol push-auth push-server push-client

push-root:
	git push

push-protocol:
	git -C protocol push

push-auth:
	git -C auth push

push-server:
	git -C server push

push-client:
	git -C client push
