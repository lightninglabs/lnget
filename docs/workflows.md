# lnget Workflow Examples

Common usage patterns for lnget, from basic downloads to agent integration.

## Basic Downloads

### Simple fetch

```bash
# Download to stdout
lnget https://api.example.com/data.json

# Save to file
lnget -o data.json https://api.example.com/data.json

# Save with original filename (from URL)
lnget https://api.example.com/report.pdf
# Creates: report.pdf
```

### Quiet mode for scripting

```bash
# Suppress progress, only output response body
lnget -q https://api.example.com/data.json

# Pipe to other tools
lnget -q https://api.example.com/data.json | jq '.results[]'

# Save to variable
DATA=$(lnget -q https://api.example.com/data.json)
```

### POST requests

```bash
# POST with JSON body
lnget -X POST -d '{"query": "bitcoin"}' https://api.example.com/search

# POST with custom content type
lnget -X POST \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "q=bitcoin&limit=10" \
  https://api.example.com/search
```

## Payment Control

### Cost limits

```bash
# Only pay invoices under 500 sats
lnget --max-cost 500 https://api.example.com/data.json

# Allow higher payments for expensive endpoints
lnget --max-cost 10000 https://api.example.com/premium-report.pdf

# Limit routing fees
lnget --max-fee 5 https://api.example.com/data.json
```

### Preview without paying

```bash
# See the 402 response without paying
lnget --no-pay https://api.example.com/data.json

# Useful for checking price before committing
lnget --no-pay -v https://api.example.com/expensive.json 2>&1 | grep -i invoice
```

### Verbose payment info

```bash
# See payment details
lnget -v https://api.example.com/data.json

# Output includes:
# - Invoice amount
# - Payment hash
# - Routing fee
# - Preimage (after payment)
```

## Token Management

### Inspect cached tokens

```bash
# List all cached tokens
lnget tokens list

# Show token for specific domain
lnget tokens show api.example.com

# JSON output for scripting
lnget tokens list --json | jq '.tokens[] | {domain, amount_paid_sat}'
```

### Force re-authentication

```bash
# Remove token for domain (next request will pay again)
lnget tokens remove api.example.com

# Clear all tokens
lnget tokens clear --force
```

### Check if token exists before request

```bash
# Script pattern: only make request if we have a valid token
if lnget tokens show api.example.com >/dev/null 2>&1; then
    lnget -q https://api.example.com/data.json
else
    echo "No token cached, would need to pay"
fi
```

## Resume Downloads

### Continue interrupted download

```bash
# Start large download
lnget -o large-file.zip https://api.example.com/large-file.zip
# Ctrl+C to interrupt

# Resume from where it left off
lnget -c -o large-file.zip https://api.example.com/large-file.zip
```

### Check resume support

```bash
# Verbose output shows if server supports Range requests
lnget -v -c https://api.example.com/file.zip 2>&1 | grep -i range
```

## Output Formats

### JSON output for agents

```bash
# Force JSON output (default when piped)
lnget --json https://api.example.com/data.json

# Parse with jq
lnget --json https://api.example.com/data.json | jq '{
  url: .url,
  paid: .payment.amount_sat,
  data: .body | fromjson
}'
```

### Human-readable output

```bash
# Force human output (default for terminals)
lnget --human https://api.example.com/data.json

# Shows formatted progress, payment info, etc.
```

## Agent Integration

### Fetch data for LLM consumption

```bash
# Get data and pipe to agent
lnget -q --max-cost 1000 https://api.example.com/knowledge.json | \
  jq -r '.content' | \
  llm "Summarize this information"
```

### Batch requests with cost tracking

```bash
#!/bin/bash
URLS=(
  "https://api.example.com/data1.json"
  "https://api.example.com/data2.json"
  "https://api.example.com/data3.json"
)

TOTAL_COST=0
for url in "${URLS[@]}"; do
  result=$(lnget --json -q "$url")
  cost=$(echo "$result" | jq -r '.payment.amount_sat // 0')
  TOTAL_COST=$((TOTAL_COST + cost))
  echo "$url: ${cost} sats"
done
echo "Total spent: ${TOTAL_COST} sats"
```

### Conditional payment based on budget

```bash
#!/bin/bash
BUDGET_SATS=5000
SPENT=0

fetch_if_budget() {
  local url=$1
  local max_cost=$2

  if [ $((SPENT + max_cost)) -gt $BUDGET_SATS ]; then
    echo "Budget exceeded, skipping $url"
    return 1
  fi

  result=$(lnget --json --max-cost "$max_cost" -q "$url" 2>/dev/null)
  if [ $? -eq 0 ]; then
    cost=$(echo "$result" | jq -r '.payment.amount_sat // 0')
    SPENT=$((SPENT + cost))
    echo "$result" | jq -r '.body'
    return 0
  fi
  return 1
}

# Use it
fetch_if_budget "https://api.example.com/cheap.json" 100
fetch_if_budget "https://api.example.com/expensive.json" 1000
echo "Total spent: $SPENT / $BUDGET_SATS sats"
```

### Retry with exponential backoff

```bash
#!/bin/bash
fetch_with_retry() {
  local url=$1
  local max_attempts=3
  local delay=1

  for ((i=1; i<=max_attempts; i++)); do
    if result=$(lnget --json -q "$url" 2>/dev/null); then
      echo "$result"
      return 0
    fi
    echo "Attempt $i failed, retrying in ${delay}s..." >&2
    sleep $delay
    delay=$((delay * 2))
  done

  echo "Failed after $max_attempts attempts" >&2
  return 1
}

fetch_with_retry "https://api.example.com/data.json"
```

## CI/CD Integration

### GitHub Actions example

```yaml
name: Fetch paid data
on: workflow_dispatch

jobs:
  fetch:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout lnget
        uses: actions/checkout@v4
        with:
          repository: lightninglabs/lnget
          path: lnget
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: lnget/go.mod
      - name: Build lnget
        run: |
          cd lnget
          go build -o /usr/local/bin/lnget ./cmd/lnget

      - name: Configure lnget
        run: |
          mkdir -p ~/.lnget
          cat > ~/.lnget/config.yaml << EOF
          ln:
            mode: lnd
            lnd:
              host: ${{ secrets.LND_HOST }}
              tls_cert_base64: ${{ secrets.LND_TLS_CERT }}
              macaroon_base64: ${{ secrets.LND_MACAROON }}
          EOF

      - name: Fetch data
        run: |
          lnget --max-cost 1000 -o data.json https://api.example.com/data.json

      - name: Use data
        run: cat data.json | jq '.results'
```

### Docker usage

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git
RUN git clone https://github.com/lightninglabs/lnget.git .
RUN go build -o /go/bin/lnget ./cmd/lnget

FROM alpine:latest
COPY --from=builder /go/bin/lnget /usr/local/bin/
ENTRYPOINT ["lnget"]
```

```bash
# Run in container
docker run -v ~/.lnget:/root/.lnget lnget https://api.example.com/data.json
```

## Debugging

### Verbose output

```bash
# Full request/response details
lnget -v https://api.example.com/data.json

# Shows:
# - Request headers
# - Response status
# - L402 challenge details
# - Payment progress
# - Token caching
```

### Check backend status

```bash
# Verify Lightning connection before making requests
lnget ln status
lnget ln info
```

### Inspect raw 402 response

```bash
# Use --no-pay to see the challenge without paying
lnget --no-pay -v https://api.example.com/data.json 2>&1

# Extract invoice for manual inspection
lnget --no-pay https://api.example.com/data.json 2>&1 | \
  grep -oP 'invoice="\K[^"]+' | \
  lncli decodepayreq -
```

## Exit Code Handling

```bash
#!/bin/bash
lnget https://api.example.com/data.json
exit_code=$?

case $exit_code in
  0) echo "Success" ;;
  1) echo "General error" ;;
  2) echo "Payment would exceed --max-cost" ;;
  3) echo "Payment failed (no route, insufficient balance, etc.)" ;;
  4) echo "Network error (connection refused, timeout, etc.)" ;;
  *) echo "Unknown error: $exit_code" ;;
esac
```
