---
name: lnget
version: 0.2.0
description: HTTP client with automatic L402 Lightning micropayment support
metadata:
  openclaw:
    requires:
      bins: ["lnget"]
    capabilities:
      - http_download
      - l402_payment
      - token_management
      - event_logging
    interfaces:
      - cli
      - mcp
    input_format: json
    output_format: json
    auth_methods:
      - lnd_macaroon
      - lnc_pairing
      - env_vars
---

# lnget

Download files with automatic L402 Lightning micropayments. When a server
returns HTTP 402 Payment Required with an L402 challenge, lnget pays the
Lightning invoice and retries the request automatically.

Tokens are cached per-domain so subsequent requests reuse them without
additional payments.

## Installation

```bash
# From source
go install github.com/lightninglabs/lnget/cmd/lnget@latest

# Or build locally
make install
```

## Quick Reference

```bash
# Fetch URL, get JSON metadata + response body inline
lnget --json --print-body https://api.example.com/data.json

# Pipe response body to stdout (two ways)
lnget -q https://api.example.com/data.json | jq .
lnget -o - https://api.example.com/data.json

# Preview payment without spending
lnget --dry-run https://api.example.com/paid-endpoint

# Agent-first: JSON input + output
lnget --json --params '{"url": "https://api.example.com/data", "max_cost": 500}'

# Introspect full CLI schema
lnget schema --all
```

## Key Rules for Agents

1. **Always use `--json`** for machine-readable output
2. **Use `--dry-run` first** before making payments to preview cost
3. **Use `--print-body`** with `--json` to get response content inline
4. **Use `-q` or `-o -`** when you only want the raw response body
5. **Use `--fields`** on list commands to limit output to needed fields
6. **Use `--force`** on destructive commands (`tokens clear`) in non-TTY mode
7. **Check `lnget schema <command>`** for parameter details

## Output Modes

lnget has three distinct output modes. Understanding them prevents the
common mistake of expecting the response body from `--json`.

### `--json` (metadata mode)
Returns structured metadata about the download. Does NOT include the
response body by default.

```bash
$ lnget --json https://example.com/price/USD
{
  "url": "https://example.com/price/USD",
  "output_path": "USD",
  "size": 67,
  "content_type": "application/json",
  "status_code": 200,
  "l402_paid": false,
  "duration": "431ms",
  "duration_ms": 430
}
```

### `--json --print-body` (metadata + body)
Includes the response body as a `"body"` field in the JSON output.
Only works for text content types under 1MB.

```bash
$ lnget --json --print-body https://example.com/price/USD
{
  "url": "https://example.com/price/USD",
  "output_path": "USD",
  "size": 67,
  "content_type": "application/json",
  "status_code": 200,
  "l402_paid": false,
  "duration": "431ms",
  "duration_ms": 430,
  "body": "{\"USD\":98234.50}"
}
```

### `-q` or `-o -` (raw body to stdout)
Pipes the response body directly to stdout with no metadata.
Best for piping into `jq` or other processors.

```bash
$ lnget -q https://example.com/price/USD
{"USD":98234.50}

$ lnget -o - https://example.com/price/USD
{"USD":98234.50}
```

## Dry-Run Mode

Always preview before paying:

```bash
$ lnget --dry-run https://api.example.com/paid-endpoint
{
  "dry_run": true,
  "url": "https://api.example.com/paid-endpoint",
  "has_cached_token": false,
  "requires_l402": true,
  "invoice_amount_sat": 100,
  "within_budget": true,
  "max_cost_sats": 1000
}
```

Exit code 10 on success (no action taken).

## Payment Control

```bash
# Set max payment (default: 1000 sats)
lnget --max-cost 500 https://api.example.com/data

# Set max routing fee (default: 10 sats)
lnget --max-fee 5 https://api.example.com/data

# Disable auto-pay entirely
lnget --no-pay https://api.example.com/data
```

## Agent-First JSON Input

Use `--params` for structured request specification:

```bash
lnget --json --params '{
  "url": "https://api.example.com/data",
  "method": "POST",
  "headers": {"Content-Type": "application/json"},
  "data": "{\"query\": \"value\"}",
  "max_cost": 500,
  "max_fee": 5
}'
```

## Token Management

```bash
# List tokens (limit output for context window)
lnget tokens list --json --fields domain,amount_sat --limit 10

# Stream as NDJSON (one object per line)
lnget tokens list --ndjson

# Show token for a specific domain
lnget tokens show example.com --json

# Remove token for a domain
lnget tokens remove example.com

# Clear all tokens (requires --force in non-TTY)
lnget tokens clear --force
```

## Lightning Backend

```bash
# Check connection status
lnget ln status --json

# Get detailed node info
lnget ln info --json

# Pair with LNC (secure stdin method)
echo "word1 word2 ... word10" | lnget ln lnc pair --stdin

# List saved LNC sessions
lnget ln lnc sessions --json
```

## Schema Introspection

```bash
# List all commands
lnget schema

# Schema for a specific command
lnget schema tokens

# Full CLI schema tree (JSON)
lnget schema --all
```

## Exit Codes

| Code | Meaning                                  |
|------|------------------------------------------|
| 0    | Success                                  |
| 1    | General error                            |
| 2    | Invalid arguments or payment too expensive |
| 3    | L402 payment failed                      |
| 4    | Network or connection error              |
| 5    | Authentication failure                   |
| 6    | Rate limited                             |
| 10   | Dry-run completed (no action taken)      |

## Error Format

Errors are JSON on stderr:
```json
{"error": true, "code": "payment_failed", "message": "no route found", "exit_code": 3}
```

## Configuration

Config file: `~/.lnget/config.yaml`

```bash
# Show current config
lnget config show --json

# Set values via JSON
lnget config set --json '{"l402": {"max_cost_sats": 5000}}'

# Set single value via dot-path
lnget config set l402.max_cost_sats 5000

# Initialize default config
lnget config init
```

### Environment Variables

```bash
export LNGET_LN_MODE=lnd                          # or: lnc, none
export LNGET_LN_LND_HOST=localhost:10009
export LNGET_LN_LND_TLS_CERT=/path/to/tls.cert
export LNGET_LN_LND_MACAROON=/path/to/admin.macaroon
export LNGET_LN_LNC_PAIRING_PHRASE="word1 word2 ... word10"
```

## MCP Integration

Expose lnget as typed MCP tools for agent frameworks:

```bash
lnget mcp serve
```

Available tools: `download`, `dry_run`, `tokens_list`, `tokens_show`,
`tokens_remove`, `ln_status`, `events_list`, `events_stats`, `config_show`.

## Common Agent Patterns

### Fetch and parse JSON API response
```bash
body=$(lnget -q https://api.example.com/data)
echo "$body" | jq '.result'
```

### Check cost before paying
```bash
# Step 1: dry-run to see cost
result=$(lnget --dry-run https://api.example.com/paid)
amount=$(echo "$result" | jq '.invoice_amount_sat')

# Step 2: pay if within budget
if [ "$amount" -lt 500 ]; then
  lnget --json --print-body https://api.example.com/paid
fi
```

### Check if domain has cached token
```bash
if lnget tokens show example.com --json >/dev/null 2>&1; then
  echo "Token cached, no payment needed"
fi
```

### Download with progress to file
```bash
lnget -o large-file.zip https://api.example.com/file.zip
```

### Resume interrupted download
```bash
lnget -c -o large-file.zip https://api.example.com/file.zip
```

## File Locations

| Path                           | Description              |
|--------------------------------|--------------------------|
| `~/.lnget/config.yaml`        | Configuration file       |
| `~/.lnget/tokens/<domain>/`   | Cached L402 tokens       |
| `~/.lnget/events.db`          | Payment event log        |
| `~/.lnget/lnget.log`          | Application log          |
| `~/.lnget/lnc/sessions/`      | Saved LNC sessions       |
