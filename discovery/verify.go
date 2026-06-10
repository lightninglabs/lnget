package discovery

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/chaincfg"
	aperturel402 "github.com/lightninglabs/aperture/l402"
	"github.com/lightningnetwork/lnd/zpay32"
	"gopkg.in/macaroon.v2"
)

// DecodedInvoice holds the fields of a BOLT 11 invoice the discovery client
// cross-checks against a quote.
type DecodedInvoice struct {
	// Destination is the hex-encoded compressed public key of the payee.
	Destination string

	// PaymentHash is the invoice's payment hash.
	PaymentHash [32]byte

	// AmountSat is the invoice amount in satoshis (0 if unset).
	AmountSat int64
}

// candidateNets are the networks we attempt to decode an invoice against. We
// try each until one matches the invoice's human-readable prefix.
var candidateNets = []*chaincfg.Params{
	&chaincfg.MainNetParams,
	&chaincfg.TestNet3Params,
	&chaincfg.RegressionNetParams,
	&chaincfg.SimNetParams,
	&chaincfg.SigNetParams,
}

// DecodeInvoice decodes a BOLT 11 invoice in-process, without contacting a
// Lightning node, extracting the payee, payment hash, and amount.
func DecodeInvoice(invoice string) (*DecodedInvoice, error) {
	var lastErr error
	for _, net := range candidateNets {
		inv, err := zpay32.Decode(invoice, net)
		if err != nil {
			lastErr = err
			continue
		}

		decoded := &DecodedInvoice{}
		if inv.PaymentHash != nil {
			decoded.PaymentHash = *inv.PaymentHash
		}
		if inv.Destination != nil {
			decoded.Destination = hex.EncodeToString(
				inv.Destination.SerializeCompressed(),
			)
		}
		if inv.MilliSat != nil {
			decoded.AmountSat = int64(*inv.MilliSat) / 1000
		}

		return decoded, nil
	}

	return nil, fmt.Errorf("failed to decode invoice: %w", lastErr)
}

// VerifyOptions parameterizes quote verification.
type VerifyOptions struct {
	// MaxCostSat is the client's maximum payment threshold in satoshis. A
	// value of zero disables the budget check.
	MaxCostSat int64

	// NodePubKey, if set, is the hex-encoded node public key the invoice
	// MUST be payable to (typically the manifest provider's node_pubkey).
	NodePubKey string
}

// VerifyQuote checks a quote response before the client pays it. It confirms
// the returned macaroon binds to the invoice and encodes the requested bundle,
// that the invoice fits the budget, and (optionally) that it pays the expected
// node. It returns nil only if every check passes.
func VerifyQuote(resp *QuoteResponse, req *QuoteRequest,
	opts VerifyOptions) error {

	mac, err := decodeMacaroon(resp.Macaroon)
	if err != nil {
		return err
	}

	id, err := aperturel402.DecodeIdentifier(bytes.NewReader(mac.Id()))
	if err != nil {
		return fmt.Errorf("failed to decode macaroon identifier: %w", err)
	}

	invoice, err := DecodeInvoice(resp.Invoice)
	if err != nil {
		return err
	}

	// The macaroon must commit to the payment hash of the invoice we are
	// about to pay.
	if id.PaymentHash != invoice.PaymentHash {
		return fmt.Errorf("macaroon payment hash %x does not match "+
			"invoice %x", id.PaymentHash, invoice.PaymentHash)
	}

	// The macaroon's caveats must encode the bundle we asked for.
	if err := verifyBundle(mac, req); err != nil {
		return err
	}

	// The invoice amount must be within budget.
	if opts.MaxCostSat > 0 && invoice.AmountSat > opts.MaxCostSat {
		return fmt.Errorf("invoice amount %d sats exceeds max cost %d "+
			"sats", invoice.AmountSat, opts.MaxCostSat)
	}

	// If a node pubkey is expected, the invoice must pay to it.
	if opts.NodePubKey != "" &&
		!strings.EqualFold(invoice.Destination, opts.NodePubKey) {

		return fmt.Errorf("invoice pays to %s, expected %s",
			invoice.Destination, opts.NodePubKey)
	}

	return nil
}

// verifyBundle checks that the macaroon's caveats grant at least the requested
// bundle and are no weaker than requested.
func verifyBundle(mac *macaroon.Macaroon, req *QuoteRequest) error {
	// The services caveat must include the requested service.
	servicesVal, ok := aperturel402.HasCaveat(
		mac, aperturel402.CondServices,
	)
	if !ok {
		return fmt.Errorf("macaroon has no services caveat")
	}
	if !servicesIncludes(servicesVal, req.Service) {
		return fmt.Errorf("macaroon services %q do not include %q",
			servicesVal, req.Service)
	}

	// Each requested capability must be granted.
	if len(req.Capabilities) > 0 {
		capsCond := req.Service + aperturel402.CondCapabilitiesSuffix
		capsVal, ok := aperturel402.HasCaveat(mac, capsCond)
		if !ok {
			return fmt.Errorf("macaroon has no capabilities caveat "+
				"for %q", req.Service)
		}
		granted := splitList(capsVal)
		for _, want := range req.Capabilities {
			if !contains(granted, want) {
				return fmt.Errorf("macaroon does not grant "+
					"capability %q", want)
			}
		}
	}

	// Each requested constraint must be present and no more restrictive
	// than requested.
	for cond, raw := range req.Constraints {
		want, ok := stringifyValue(raw)
		if !ok {
			continue
		}
		got, ok := aperturel402.HasCaveat(mac, cond)
		if !ok {
			return fmt.Errorf("macaroon missing constraint %q", cond)
		}

		// For integer constraints the granted value must be at least
		// what we requested (more is acceptable); otherwise it must
		// match exactly.
		wantN, wantErr := strconv.ParseInt(want, 10, 64)
		gotN, gotErr := strconv.ParseInt(got, 10, 64)
		if wantErr == nil && gotErr == nil {
			if gotN < wantN {
				return fmt.Errorf("constraint %q granted %d is "+
					"weaker than requested %d", cond, gotN,
					wantN)
			}
		} else if got != want {
			return fmt.Errorf("constraint %q granted %q, requested "+
				"%q", cond, got, want)
		}
	}

	return nil
}

// decodeMacaroon decodes a base64-encoded macaroon, trying standard then
// URL-safe encoding.
func decodeMacaroon(b64 string) (*macaroon.Macaroon, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		raw, err = base64.RawURLEncoding.DecodeString(b64)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to decode macaroon: %w", err)
	}

	mac := &macaroon.Macaroon{}
	if err := mac.UnmarshalBinary(raw); err != nil {
		return nil, fmt.Errorf("invalid macaroon: %w", err)
	}
	return mac, nil
}

// servicesIncludes reports whether a services caveat value (name:tier pairs)
// includes the named service.
func servicesIncludes(value, service string) bool {
	for _, entry := range strings.Split(value, ",") {
		name := entry
		if idx := strings.Index(entry, ":"); idx >= 0 {
			name = entry[:idx]
		}
		if name == service {
			return true
		}
	}
	return false
}

// splitList splits a comma-separated caveat list.
func splitList(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// stringifyValue renders a constraint value (int, float, or string) as the
// string used in a caveat.
func stringifyValue(raw interface{}) (string, bool) {
	switch v := raw.(type) {
	case string:
		return v, true
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10), true
		}
		return strconv.FormatFloat(v, 'f', -1, 64), true
	case int:
		return strconv.Itoa(v), true
	case int64:
		return strconv.FormatInt(v, 10), true
	default:
		return "", false
	}
}
