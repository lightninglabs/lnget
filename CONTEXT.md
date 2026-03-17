# lnget Agent Context

## Quick Start

```bash
# Always use --json for machine-readable output
lnget --json https://api.example.com/data.json

# Preview before paying (no mutations)
lnget --dry-run https://api.example.com/paid-endpoint

# Download with full JSON result
lnget --json -o output.json https://api.example.com/data.json

# Full request specification via JSON
lnget --json --params '{"url": "https://api.example.com/data", "max_cost": 500}'
```

## Preferred Invocation Patterns

- **ALWAYS** use `--json` when invoking programmatically
- **ALWAYS** use `--dry-run` before mutating operations (payments)
- **ALWAYS** use `--fields` on list commands to limit output size
- **NEVER** pass user-provided strings directly as domain args without
  validation (the CLI validates, but defense in depth)
- **NEVER** omit `--force` on destructive commands in non-interactive mode

## Output Format

JSON is the default when stdout is not a TTY. When piped or called by an
agent, output is always JSON unless `--human` is explicitly set.

### Download Result (stdout)
```json
{
  "url": "https://api.example.com/data.json",
  "output_path": "data.json",
  "size": 1024,
  "content_type": "application/json",
  "status_code": 200,
  "l402_paid": true,
  "l402_amount_sat": 100,
  "l402_fee_sat": 1,
  "duration": "1.234s",
  "duration_ms": 1234
}
```

### Dry-Run Result (stdout)
```json
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

### Error Format (stderr)
```json
{"error": true, "code": "payment_failed", "message": "no route found", "exit_code": 3}
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Invalid arguments or payment too expensive |
| 3 | L402 payment failed |
| 4 | Network or connection error |
| 5 | Authentication failure |
| 6 | Rate limited |
| 10 | Dry-run completed (no action taken) |

## Schema Introspection

```bash
lnget schema                # List all commands
lnget schema tokens         # Schema for tokens command
lnget schema --all          # Full CLI schema tree
```

## Context Window Discipline

```bash
# Limit token list to specific fields
lnget tokens list --fields domain,amount_sat

# Cap results
lnget tokens list --limit 10

# Stream as NDJSON (one object per line)
lnget tokens list --ndjson
```

## Auth Configuration for Headless/Agent Use

### LND (file-based credentials)
```bash
export LNGET_LN_MODE=lnd
export LNGET_LN_LND_HOST=localhost:10009
export LNGET_LN_LND_TLS_CERT=/path/to/tls.cert
export LNGET_LN_LND_MACAROON=/path/to/admin.macaroon
```

### LNC (pairing phrase via env var)
```bash
export LNGET_LN_MODE=lnc
export LNGET_LN_LNC_PAIRING_PHRASE="word1 word2 ... word10"
```

### LNC (stdin for security)
```bash
echo "word1 word2 ... word10" | lnget ln lnc pair --stdin
```

### No payment backend
```bash
export LNGET_LN_MODE=none
```

## Config Mutation

```bash
# Bulk update via JSON
lnget config set --json '{"l402": {"max_cost_sats": 5000}}'

# Single value via dot-path
lnget config set l402.max_cost_sats 5000
```
