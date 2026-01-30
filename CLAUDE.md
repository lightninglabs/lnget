# lnget Agent Assistant Guide

> **IMPORTANT**: For complete style guidelines with detailed examples, see
> [`development_guidelines.md`](development_guidelines.md). This file provides
> a quick reference for AI agents.

## Project Overview

`lnget` is a command-line HTTP client similar to `wget` that natively supports
L402 (Lightning HTTP 402) authentication. When a server responds with HTTP 402
Payment Required and an L402 challenge, `lnget` automatically:

1. Parses the `WWW-Authenticate` header containing a macaroon and invoice
2. Pays the Lightning invoice via a connected lnd node
3. Retries the request with the paid L402 token
4. Caches tokens for reuse on subsequent requests

### Core Dependencies

- **aperture/l402**: L402 token handling, parsing, and storage
  (`github.com/lightninglabs/aperture/l402`)
- **lndclient**: Lightning Network client for invoice payments
  (`github.com/lightninglabs/lndclient`)
- **btcd/btcutil**: Bitcoin utilities and amount handling

## Essential Commands

### Building and Testing
- `make build` - Compile the project
- `make install` - Install to $GOPATH/bin
- `make tidy-module-check` - Verify module files are tidy
- `make lint` - Run the linter (must pass before committing)
- `make fmt` - Format all Go source files
- `make clean` - Remove build artifacts

### Testing Commands
- Single package: `make unit pkg=<package> case=<test> timeout=5m`
- Debug with logs: `make unit log="stdlog trace" pkg=<package> case=<test>`
- All tests: `make unit`

## Code Style Quick Reference

**IMPORTANT**: Editors must be configured with **tab = 8 spaces** for correct
formatting.

### Function and Method Comments
- **Every function and method** (including unexported ones) must have a comment
  starting with the function/method name
- Comments should explain **how/why**, not just what
- Use literate programming style—comments should be additive and insightful
- All exported functions need detailed documentation

### GoDoc for Exported Identifiers
- Any exported identifier (type, const, var, func, method) must have a GoDoc
  comment that starts with the identifier name.
- Exported struct fields must have a GoDoc comment (GoDoc style, starting with
  the field name) and wrapped to 80 columns.
- All GoDoc-style comments must be wrapped to 80 columns.

### Comments for Non-trivial Code
- Any non-trivial code blocks (multi-step algorithms, subtle invariants,
  concurrency/locking, retries/idempotency, tricky encodings) must include
  explanatory comments that describe the "why" and any invariants.
- These explanatory comments should also be wrapped to 80 columns.

### Code Organization and Spacing
- 80-character line limit (best effort)
- Organize code into logical stanzas separated by blank lines
- Add explanatory comments between stanzas
- Spacing between switch/select cases
- When wrapping function calls, put closing paren on its own line with all
  args on new lines

### Error and Log Message Formatting
Log and error messages use compact form to minimize lines while staying under
80 characters:

**WRONG**
```go
return fmt.Errorf(
	"failed to pay invoice for %s: %v",
	url, err,
)
```

**RIGHT**
```go
return fmt.Errorf("failed to pay invoice for %s: %v", url, err)
```

### Structured Logging
**YOU MUST** use structured log methods (ending in `S`) with static messages:
- First parameter: `context.Context`
- Second parameter: static string (no `fmt.Sprintf`)
- Remaining parameters: key-value pairs using `slog.Int()`, `btclog.Fmt()`,
  `btclog.Hex()`, etc.
- One key-value pair per line for readability
- Lines can exceed 80 chars for structured logging

Example:
```go
log.InfoS(ctx, "L402 payment completed",
	slog.String("url", targetURL),
	btclog.Fmt("amount_sat", "%.0f", float64(amountMsat)/1000))
```

### Error Log Levels
**CRITICAL**: Only use `error` level for **internal errors never expected
during normal operation**.
- External triggers (network failures, payment failures, server errors) should
  use lower levels (`warn`, `info`, `debug`)
- If a user could cause it, it's not an error-level log

## L402 Implementation Patterns

### Token Flow
1. Make initial HTTP request to target URL
2. If 402 received, parse `WWW-Authenticate: L402 macaroon="...", invoice="..."`
3. Decode macaroon and invoice from the challenge
4. Verify invoice amount is within configured max cost
5. Store pending token before initiating payment
6. Pay invoice via lnd
7. Store paid token with preimage
8. Retry request with `Authorization: L402 <macaroon>:<preimage>` header

### Key Types (from aperture/l402)
- `l402.Token` - Stores macaroon, payment hash, preimage, amounts
- `l402.Store` - Interface for token persistence
- `l402.FileStore` - File-based token storage implementation

### HTTP Client Considerations
- Use `http.Client` with custom `Transport` or wrapper for L402 handling
- Handle both HTTP and gRPC L402 challenges (different response formats)
- Support configurable timeouts for both HTTP requests and Lightning payments
- Allow insecure (non-TLS) connections via flag for testing

## Git Commit Guidelines

### Commit Message Format
```
pkg: Short summary in present tense (≤50 chars)

Longer explanation if needed, wrapped at 72 characters. Explain WHY
this change is being made and any relevant context, not just WHAT
changed.
```

**Commit message rules**:
- First line: present tense ("Fix bug" not "Fixed bug")
- Prefix with package name: `cmd:`, `client:`, `multi:` (for multiple packages)
- Subject ≤50 characters
- Body wrapped at 72 characters
- Blank line between subject and body

### Commit Granularity
**IMPORTANT**: Prefer small, atomic commits that build independently.

Separate commits for:
- Bug fixes (one fix per commit)
- Code restructuring/refactoring
- File moves or renames
- New subsystems or features
- Integration of new functionality

## Testing Philosophy

### Coverage Requirements
Strive for **near 90% test coverage** where practical.

### Testing Approaches
- **Unit tests**: HTTP client logic, token parsing, configuration validation
- **Integration tests**: End-to-end with mock L402 server and mock lnd
- **Table-driven tests**: Cover edge cases for URL parsing, header handling

### Before Committing
**YOU MUST** run tests before every commit:

1. Run module tidy check: `make tidy-module-check`
2. Run unit tests: `make unit pkg=$pkg case=$case timeout=5m`
3. Run lint: `make lint`
4. **Check logs carefully**:
   - Verify structured logging format is correct
   - Ensure no log spam
   - **No `[ERR]` lines should appear** unless testing error paths

## CLI Design Principles

### wget-like Interface
Maintain familiar wget semantics where applicable:
- `lnget <url>` - Fetch URL, save to file named from URL path
- `lnget -O <file> <url>` - Fetch URL, save to specified file
- `lnget -q <url>` - Quiet mode
- `lnget --max-cost <sats>` - Maximum satoshis to pay automatically

### L402-specific Flags
- `--lnd-host` - lnd gRPC host
- `--lnd-macaroon-path` - Path to lnd macaroon
- `--lnd-tls-cert-path` - Path to lnd TLS certificate
- `--max-cost` - Maximum invoice amount to pay automatically (default: 1000 sats)
- `--max-routing-fee` - Maximum routing fee (default: 10 sats)
- `--token-dir` - Directory to store L402 tokens
- `--allow-insecure` - Allow non-TLS connections

### Exit Codes
- `0` - Success
- `1` - General error
- `2` - Payment required but exceeded max cost
- `3` - Payment failed
- `4` - Network/connection error

## Common Pitfalls to Avoid

1. **Do not pay invoices without checking amount** - Always verify against
   max cost before paying
2. **Do not lose pending tokens** - Store token before payment, handle
   interrupted payments gracefully
3. **Do not ignore payment tracking** - If payment is in-flight, track it
   rather than paying again
4. **Do not use `error` log level for payment failures** - These are expected
   external events
5. **Do not skip tests** - All new code requires test coverage
6. **Do not use 4-space tabs** - Configure editor for 8-space tabs
7. **Do not hardcode lnd connection details** - Always use configuration
8. **Do not commit without running `make lint`** - Linter must pass

## Project Structure

```
lnget/
├── cmd/
│   └── lnget/
│       └── main.go           # CLI entrypoint and flag parsing
├── client/
│   ├── client.go             # HTTP client with L402 support
│   ├── client_test.go
│   ├── transport.go          # Custom http.RoundTripper for L402
│   └── transport_test.go
├── config/
│   ├── config.go             # Configuration loading and validation
│   └── config_test.go
├── development_guidelines.md # Detailed style guide
├── CLAUDE.md                 # This file
├── Makefile
├── go.mod
└── go.sum
```

## Additional Resources

- **[`development_guidelines.md`](development_guidelines.md)** - Complete style
  guide with extensive WRONG/RIGHT examples
- **[aperture/l402](https://github.com/lightninglabs/aperture/tree/master/l402)**
  - L402 reference implementation
- **[L402 Protocol Spec](https://github.com/lightninglabs/L402)** - Protocol
  specification
