FROM golang:1.25

# Install golangci-lint.
RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Set working directory.
WORKDIR /build

# Default command.
CMD ["golangci-lint", "run", "-v"]
