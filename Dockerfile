# Builder image
FROM golang:1.20.4-alpine3.18 AS builder

WORKDIR /build

ARG VERSION

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build GOMODCACHE=/go/pkg/mod GOCACHE=/root/.cache/go-build go build -v -ldflags '-w -s -X 'github.com/coufalja/memcached-mysql/memcached.Version=${VERSION}

# Runtime image
FROM alpine:3.18.0
RUN apk --no-cache add ca-certificates

COPY --from=builder /build/memcached-mysql /app/memcached-mysql
WORKDIR /app

ENTRYPOINT ["./memcached-mysql"]
