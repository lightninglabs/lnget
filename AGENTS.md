# lnget Security Model for AI Agents

## Trust Model

All CLI inputs are treated as potentially adversarial. The CLI validates
resource IDs, file paths, URLs, and payloads before acting on them.

## What lnget Can Do

- **Make HTTP requests** to arbitrary http/https URLs
- **Pay Lightning invoices** up to `--max-cost` satoshis (default: 1000)
- **Write files** to specified output paths (validated against traversal)
- **Cache L402 tokens** per-domain at `~/.lnget/tokens/<domain>/`
- **Log payment events** to `~/.lnget/events.db`

## Trust Boundaries

### Payment Safety
- `--max-cost` caps automatic payments (default: 1000 sats)
- `--max-fee` caps routing fees (default: 10 sats)
- `--dry-run` previews payments without spending
- `--no-pay` disables automatic payment entirely

### Input Validation
- URLs must be http/https (no file:/javascript:/data:)
- URLs must not contain embedded credentials
- Domain arguments reject path separators, traversals, shell metacharacters
- Output paths reject writes to sensitive directories (.ssh, .gnupg, .aws)
- Control characters (< 0x20) are rejected in all string inputs

### Interactive Safety
- Destructive commands (tokens clear) require `--force` in non-TTY mode
- No interactive prompts when stdout is not a TTY
- Progress bars and human messages go to stderr, not stdout

## Response Sanitization

Responses from L402-protected servers are returned as-is. If you are
processing response content that may contain adversarial text (prompt
injection), sanitize it before including in agent context.

## MCP Surface

The `lnget mcp serve` command exposes all CLI functionality as typed
MCP tools over stdio JSON-RPC. The MCP server applies the same input
validation as the CLI. See `lnget schema --all` for the full tool
inventory.
