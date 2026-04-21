FROM golang:1.23-bookworm AS build
WORKDIR /src
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -o /out/reverse-server ./cmd/reverse-server

FROM gcr.io/distroless/base-debian12
COPY --from=build /out/reverse-server /usr/local/bin/reverse-server
ENTRYPOINT ["/usr/local/bin/reverse-server"]
