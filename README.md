# lnget

lnget is a command-line HTTP client that handles L402 Lightning payments transparently. It was designed for programmatic access to paid APIs—when a server returns HTTP 402 Payment Required, lnget automatically pays the invoice via Lightning and retries the request. No manual intervention, no payment flow interruption.

## The Problem

The web is increasingly experimenting with micropayments. L402 (formerly LSAT) enables APIs to charge per-request using Lightning Network invoices. A server responds with 402 Payment Required, includes a macaroon and invoice in the `WWW-Authenticate` header, and expects the client to pay before retrying.

Existing HTTP clients don't understand this flow. You'd need to parse the challenge header, extract the invoice, switch to a Lightning wallet, pay, extract the preimage, construct the authorization header, and retry. For a single request that's tedious. For automated pipelines or AI agents consuming paid APIs, it's a blocker.

## The Solution

lnget handles the entire L402 flow automatically:

```bash
lnget https://api.example.com/premium-data.json
```

If the server returns 402, lnget parses the challenge, pays the invoice through your configured Lightning backend, and retries with the proper `Authorization: L402` header. The response streams to stdout or a file, just like wget or curl.

Tokens are cached per-domain, so subsequent requests reuse the paid credential without additional payments:

```bash
# First request: pays invoice, caches token
lnget https://api.example.com/data/1

# Second request: reuses token, no payment
lnget https://api.example.com/data/2
```

## Installation

```bash
go install github.com/lightninglabs/lnget/cmd/lnget@latest
```

Or build from source:

```bash
git clone https://github.com/lightninglabs/lnget.git
cd lnget
make install
```

## Configuration

lnget needs a Lightning backend to pay invoices. Configure it once:

```bash
# Initialize config file
lnget config init

# Edit ~/.lnget/config.yaml with your lnd details
```

Example config for external lnd:

```yaml
ln:
  mode: lnd
  lnd:
    host: localhost:10009
    tls_cert: ~/.lnd/tls.cert
    macaroon: ~/.lnd/data/chain/bitcoin/mainnet/admin.macaroon
```

Or use environment variables:

```bash
export LNGET_LN_LND_HOST=localhost:10009
export LNGET_LN_LND_MACAROON=~/.lnd/data/chain/bitcoin/mainnet/admin.macaroon
export LNGET_LN_LND_TLS_CERT=~/.lnd/tls.cert
```

Verify the connection:

```bash
lnget ln status
```

## Usage

Basic usage mirrors wget and curl:

```bash
# Download to stdout
lnget https://api.example.com/data.json

# Save to file
lnget -o data.json https://api.example.com/data.json

# Quiet mode for piping
lnget -q https://api.example.com/data.json | jq .

# POST request with data
lnget -X POST -d '{"query": "test"}' https://api.example.com/search

# Custom headers
lnget -H "Accept: application/json" https://api.example.com/data
```

### Payment Controls

Set limits on automatic payments:

```bash
# Maximum invoice amount to pay automatically (default: 1000 sats)
lnget --max-cost 5000 https://api.example.com/expensive-endpoint

# Maximum routing fee (default: 10 sats)
lnget --max-fee 50 https://api.example.com/data

# Disable automatic payment (just show the 402 response)
lnget --no-pay https://api.example.com/data
```

### Resume Support

Resume interrupted downloads:

```bash
lnget -c https://api.example.com/large-file.zip
```

If the server supports Range requests and you have a partial file, lnget continues from where it left off.

### Token Management

View and manage cached L402 tokens:

```bash
# List all cached tokens
lnget tokens list

# Show token for specific domain
lnget tokens show api.example.com

# Remove token (forces re-payment on next request)
lnget tokens remove api.example.com

# Clear all tokens
lnget tokens clear
```

## Output Formats

lnget supports two output modes: human-readable (default for terminals) and JSON (default when piped):

```bash
# Force JSON output
lnget --json https://api.example.com/data

# Force human output
lnget --human https://api.example.com/data
```

JSON output is structured for programmatic consumption:

```json
{
  "url": "https://api.example.com/data.json",
  "status": 200,
  "size": 1024,
  "duration": "0.5s",
  "payment": {
    "amount_sat": 100,
    "fee_sat": 2,
    "preimage": "abc123..."
  }
}
```

## Why This Matters for Agents

AI agents need to consume APIs programmatically. When those APIs require payment, the agent needs a seamless way to authorize and pay without human intervention.

lnget provides:

- **Automatic payment flow**: No manual steps between 402 response and authorized retry
- **Token caching**: Pay once, reuse the credential for subsequent requests
- **Cost controls**: `--max-cost` prevents runaway spending
- **JSON output**: Structured responses that agents can parse directly
- **Quiet mode**: `-q` suppresses everything except the response body

An agent can call `lnget -q --max-cost 1000 https://api.example.com/data | jq .result` and get the data, with payment handled transparently in the background.

## Lightning Backends

lnget supports multiple Lightning backends:

### External lnd

Connect to a running lnd instance:

```yaml
ln:
  mode: lnd
  lnd:
    host: localhost:10009
    tls_cert: ~/.lnd/tls.cert
    macaroon: ~/.lnd/data/chain/bitcoin/mainnet/admin.macaroon
```

### Lightning Node Connect (LNC)

Connect via LNC pairing phrase for remote nodes:

```bash
lnget ln lnc pair "your-pairing-phrase-here"
```

Then set the backend:

```yaml
ln:
  mode: lnc
```

### Embedded Neutrino (Experimental)

Run a lightweight SPV wallet directly in lnget:

```bash
lnget ln neutrino init
```

```yaml
ln:
  mode: neutrino
```

## How It Works

When you run `lnget https://api.example.com/data`:

1. lnget checks for a cached token for `api.example.com`
2. If found and valid, includes `Authorization: L402 <macaroon>:<preimage>` header
3. Makes the HTTP request
4. If server returns 402 with `WWW-Authenticate: L402 macaroon="...", invoice="..."`:
   - Parses the macaroon and invoice from the header
   - Verifies the invoice amount is within `--max-cost`
   - Pays the invoice via the configured Lightning backend
   - Stores the token (macaroon + preimage) for the domain
   - Retries the request with the authorization header
5. Streams the response to stdout or the output file

Tokens are stored at `~/.lnget/tokens/<domain>/` and persist across invocations.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Payment required but exceeded max cost |
| 3 | Payment failed |
| 4 | Network/connection error |

## Dashboard

lnget includes a web dashboard for monitoring your L402 spending, tokens, and wallet status.

### Running the Dashboard

```bash
# Start the API server (serves data from ~/.lnget/events.db)
lnget serve --addr localhost:2402

# In another terminal, start the dashboard dev server
cd dashboard
npm install
npm run dev
# Open http://localhost:3001
```

The dashboard shows:
- **Dashboard**: Total spending, payment counts, active tokens, wallet balance, spending charts
- **Tokens**: Cached L402 tokens with domain, amount, status, and management actions
- **Payments**: Full payment history with filters, volume charts, success rates
- **Status**: Lightning backend info, wallet balance, configuration overview

### Event Logging

lnget automatically records all L402 payment events to `~/.lnget/events.db` (SQLite). This is enabled by default and can be configured:

```yaml
events:
  enabled: true
  db_path: ~/.lnget/events.db
```

### API Server

`lnget serve` exposes a REST API on `localhost:2402`:

| Endpoint | Description |
|----------|-------------|
| `GET /api/events` | List payment events (query: limit, offset, domain, status) |
| `GET /api/events/stats` | Aggregate spending statistics |
| `GET /api/events/domains` | Per-domain spending breakdown |
| `GET /api/tokens` | List all cached tokens |
| `DELETE /api/tokens/:domain` | Remove a token |
| `GET /api/status` | Lightning backend status |
| `GET /api/config` | Current configuration (sensitive fields redacted) |

## Development

```bash
make build       # build binary
make install     # install to $GOPATH/bin
make unit        # run tests
make lint        # run linters
make fmt         # format code
```

The codebase follows a functional core / imperative shell pattern:

- `l402/` - Token handling, challenge parsing, storage (pure/testable)
- `client/` - HTTP client with L402 transport layer
- `ln/` - Lightning backend implementations
- `events/` - SQLite event store for payment logging
- `api/` - REST API server for dashboard
- `cli/` - Cobra command definitions
- `cmd/lnget/` - Entry point
- `dashboard/` - Next.js web dashboard

See [docs/agents.md](docs/agents.md) for architecture details aimed at AI agents working on this codebase.

## License

MIT
