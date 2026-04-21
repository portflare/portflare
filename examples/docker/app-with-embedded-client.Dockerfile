FROM golang:1.23-bookworm AS reverse-client-build
WORKDIR /src
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build \
    -ldflags "-s -w -X github.com/portflare/portflare/internal/buildinfo.Version=${VERSION} -X github.com/portflare/portflare/internal/buildinfo.Commit=${COMMIT} -X github.com/portflare/portflare/internal/buildinfo.Date=${BUILD_DATE}" \
    -o /out/reverse-client ./cmd/reverse-client

FROM node:22-bookworm-slim
WORKDIR /app
RUN npm install -g http-server
COPY --from=reverse-client-build /out/reverse-client /usr/local/bin/reverse-client
COPY examples/docker/embedded-entrypoint.sh /usr/local/bin/embedded-entrypoint.sh
RUN chmod +x /usr/local/bin/embedded-entrypoint.sh
ENV REVERSE_CLIENT_LISTEN_ADDR=127.0.0.1:9901
ENV REVERSE_CLIENT_DISCOVER=true
ENV REVERSE_CLIENT_DISCOVER_ALLOW=3000
ENV REVERSE_CLIENT_DISCOVER_DENY=22,2375,2376
ENV REVERSE_CLIENT_DISCOVER_NAMES=3000=web
EXPOSE 3000
ENTRYPOINT ["/usr/local/bin/embedded-entrypoint.sh"]
CMD ["http-server", ".", "-p", "3000"]
