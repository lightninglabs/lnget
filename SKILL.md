---
name: lnget
version: 0.1.0
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
returns HTTP 402 Payment Required with an L402 challenge, lnget
automatically pays the Lightning invoice and retries the request.

## Quick Reference

```bash
# JSON metadata + inline response body
lnget --json --print-body https://api.example.com/data.json

# Pipe raw response body to stdout
lnget -q https://api.example.com/data.json | jq .
lnget -o - https://api.example.com/data.json

# Preview payment without executing
lnget --dry-run https://api.example.com/paid-endpoint

# Agent-first JSON input
lnget --json --params '{"url": "https://api.example.com/data", "max_cost": 500}'

# Introspect CLI schema
lnget schema --all

# Manage tokens
lnget tokens list --json --fields domain,amount_sat

# Check Lightning backend
lnget ln status --json
```

## Key Rules

1. Always use `--json` for machine-readable output
2. Use `--print-body` with `--json` to get response content inline
3. Use `--dry-run` before making payments
4. Use `-q` or `-o -` when you only want the raw response body
5. Use `--fields` to limit output to needed fields
6. Use `--force` on destructive commands (tokens clear)
7. Check `lnget schema <command>` for parameter details

## Full skill documentation

See `skills/lnget/SKILL.md` for comprehensive usage guide.
