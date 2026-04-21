FROM golang:1.23-bookworm AS build
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

FROM gcr.io/distroless/base-debian12
COPY --from=build /out/reverse-client /usr/local/bin/reverse-client
ENTRYPOINT ["/usr/local/bin/reverse-client","daemon"]
