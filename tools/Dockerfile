FROM golang:1.25

# Install golangci-lint v2.
RUN go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

# Set working directory.
WORKDIR /build

# Default command.
CMD ["golangci-lint", "run", "-v"]
