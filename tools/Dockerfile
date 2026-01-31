FROM golang:1.25

ENV GOCACHE=/tmp/build/.cache
ENV GOMODCACHE=/tmp/build/.modcache

COPY . /tmp/tools

RUN cd /tmp/tools \
    && mkdir -p /tmp/build/.cache \
    && mkdir -p /tmp/build/.modcache \
    && go mod download \
    && go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint \
    && chmod -R 777 /tmp/build/ \
    && git config --global --add safe.directory /build

WORKDIR /build

# Run golangci-lint from /build directory.
# LINT_WORKERS env var is optional for limiting concurrency.
CMD ["sh", "-c", "golangci-lint run -v ${LINT_WORKERS:-} ./..."]
