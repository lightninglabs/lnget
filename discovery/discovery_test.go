package discovery

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	aperturel402 "github.com/lightninglabs/aperture/l402"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/zpay32"
	"gopkg.in/macaroon.v2"
)

const bespokeManifest = `{
  "version": "1.0",
  "provider": {"name": "Acme Weather", "node_pubkey": "02abc"},
  "currencies": ["msat"],
  "quote_endpoint": "/l402/quote",
  "services": [{
    "name": "weather",
    "capabilities": ["forecast", "historical"],
    "resources": [{
      "path": "/v1/forecast",
      "pricing": {"model": "fixed", "price_msat": 21000},
      "constraints": {"weather_monthly_requests": {"type": "integer", "max": 1000000}}
    }]
  }],
  "caveats": {"services": {"type": "service_list", "attenuation": "subset"}}
}`

const openAPIManifest = `{
  "openapi": "3.1.0",
  "info": {
    "title": "Acme Weather",
    "x-l402-version": "1.0",
    "x-l402-provider": {"node_pubkey": "02abc", "currencies": ["msat"]}
  },
  "servers": [{"url": "https://api.weather.example"}],
  "x-l402-quote-endpoint": "/l402/quote",
  "x-l402-caveats": {"services": {"type": "service_list", "attenuation": "subset"}},
  "paths": {
    "/v1/forecast": {
      "parameters": [],
      "get": {
        "x-l402-service": "weather",
        "x-l402-capability": "forecast",
        "x-l402-tiers": [{"tier": 0, "name": "base"}],
        "x-l402-pricing": {"model": "formula", "base_msat": 1000,
          "components": [{"constraint": "forecast_monthly_requests",
            "price_msat_per_unit": 10, "unit": 1}]},
        "x-l402-constraints": {"forecast_monthly_requests": {"type": "integer", "max": 1000000}}
      }
    }
  }
}`

func TestParseManifestBespoke(t *testing.T) {
	t.Parallel()

	m, err := ParseManifest([]byte(bespokeManifest))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if m.Profile != "bespoke" {
		t.Fatalf("profile = %q", m.Profile)
	}
	if m.Provider.Name != "Acme Weather" {
		t.Fatalf("provider = %q", m.Provider.Name)
	}
	svc := m.FindService("weather")
	if svc == nil {
		t.Fatal("weather service not found")
	}
	if got := svc.Resources[0].Pricing.PriceMsat; got != 21000 {
		t.Fatalf("price_msat = %d", got)
	}
}

func TestParseManifestOpenAPI(t *testing.T) {
	t.Parallel()

	m, err := ParseManifest([]byte(openAPIManifest))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if m.Profile != "openapi" {
		t.Fatalf("profile = %q", m.Profile)
	}
	if m.QuoteEndpoint != "/l402/quote" {
		t.Fatalf("quote_endpoint = %q", m.QuoteEndpoint)
	}
	svc := m.FindService("weather")
	if svc == nil {
		t.Fatal("weather service not found")
	}
	if !contains(svc.Capabilities, "forecast") {
		t.Fatalf("capabilities = %v", svc.Capabilities)
	}
	pr := svc.Resources[0].Pricing
	if pr.Model != "formula" || pr.BaseMsat != 1000 {
		t.Fatalf("pricing = %+v", pr)
	}
	// Formula computes locally.
	got := pr.ComputeMsat(map[string]int64{"forecast_monthly_requests": 100000})
	if got != 1000+10*100000 {
		t.Fatalf("ComputeMsat = %d", got)
	}
}

func TestFetchManifestWellKnown(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == ManifestPath {
				_, _ = w.Write([]byte(bespokeManifest))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		},
	))
	defer srv.Close()

	m, err := FetchManifest(context.Background(), srv.Client().Do, srv.URL)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if m.FindService("weather") == nil {
		t.Fatal("weather service missing")
	}
}

func TestFetchManifestLinkHeader(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/alt-manifest.json",
		func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(bespokeManifest))
		})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == ManifestPath {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Link",
			`</alt-manifest.json>; rel="l402-manifest"`)
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	m, err := FetchManifest(context.Background(), srv.Client().Do, srv.URL)
	if err != nil {
		t.Fatalf("fetch via link: %v", err)
	}
	if m.FindService("weather") == nil {
		t.Fatal("weather service missing")
	}
}

func TestFetchManifestNone(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		},
	))
	defer srv.Close()

	_, err := FetchManifest(context.Background(), srv.Client().Do, srv.URL)
	if err != ErrNoManifest {
		t.Fatalf("err = %v, want ErrNoManifest", err)
	}
}

func TestRequestQuoteSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"price_msat":21000,` +
				`"quote_expiry":1730000000,"macaroon":"AA==",` +
				`"invoice":"lnbc..."}`))
		},
	))
	defer srv.Close()

	resp, err := RequestQuote(
		context.Background(), srv.Client().Do, srv.URL,
		&QuoteRequest{Service: "weather"},
	)
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	if resp.PriceMsat != 21000 {
		t.Fatalf("price_msat = %d", resp.PriceMsat)
	}
}

func TestRequestQuoteError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"code":"unknown_service",` +
				`"message":"nope","field":"service"}}`))
		},
	))
	defer srv.Close()

	_, err := RequestQuote(
		context.Background(), srv.Client().Do, srv.URL,
		&QuoteRequest{Service: "nope"},
	)
	qe, ok := err.(*QuoteError)
	if !ok {
		t.Fatalf("err type = %T, want *QuoteError", err)
	}
	if qe.Code != "unknown_service" || qe.Status != http.StatusBadRequest {
		t.Fatalf("quote error = %+v", qe)
	}
}

func TestDecodeInvoice(t *testing.T) {
	t.Parallel()

	// A real mainnet invoice for 1500n BTC = 150 sats.
	const inv = "lnbc1500n1pw5kjhmpp5fu6xhthlt2vucmzkx6c7wtlh2r625r30" +
		"cyjsfqhu8rsx4xpz5lwqdpa2fjkzep6yptksct5yp5hxgrrv96hx6twvusy" +
		"cn3qv9jx7ur5d9hkugr5dusx6cqzpgxqr23s79ruapxc4j5uskt4htly2sa" +
		"lw4drq979d7rcela9wz02elhypmdzmzlnxuknpgfyfm86pntt8vvkvffma5q" +
		"c9n50h4mvqhngadqy3ngqjcym5a"

	decoded, err := DecodeInvoice(inv)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.AmountSat != 150 {
		t.Fatalf("amount = %d, want 150", decoded.AmountSat)
	}
}

// makeInvoice builds a signed regtest invoice committing to paymentHash for the
// given amount, returning the encoded invoice and the signer's pubkey hex.
func makeInvoice(t *testing.T, paymentHash [32]byte, amtSat int64) (string,
	string) {

	t.Helper()

	priv, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("priv key: %v", err)
	}

	inv, err := zpay32.NewInvoice(
		&chaincfg.RegressionNetParams, paymentHash,
		time.Unix(1_700_000_000, 0),
		zpay32.Amount(lnwire.MilliSatoshi(amtSat*1000)),
		zpay32.Description("lnget discovery test"),
	)
	if err != nil {
		t.Fatalf("new invoice: %v", err)
	}

	signer := zpay32.MessageSigner{
		SignCompact: func(msg []byte) ([]byte, error) {
			return ecdsa.SignCompact(
				priv, chainhash.HashB(msg), true,
			), nil
		},
	}
	encoded, err := inv.Encode(signer)
	if err != nil {
		t.Fatalf("encode invoice: %v", err)
	}

	pubHex := hexEncode(priv.PubKey().SerializeCompressed())
	return encoded, pubHex
}

func makeMacaroon(t *testing.T, paymentHash [32]byte,
	caveats ...string) string {

	t.Helper()

	var buf bytes.Buffer
	err := aperturel402.EncodeIdentifier(&buf, &aperturel402.Identifier{
		Version:     aperturel402.LatestVersion,
		PaymentHash: lntypes.Hash(paymentHash),
	})
	if err != nil {
		t.Fatalf("encode id: %v", err)
	}

	var rootKey [aperturel402.SecretSize]byte
	mac, err := macaroon.New(
		rootKey[:], buf.Bytes(), "lsat", macaroon.LatestVersion,
	)
	if err != nil {
		t.Fatalf("new macaroon: %v", err)
	}
	for _, c := range caveats {
		if err := mac.AddFirstPartyCaveat([]byte(c)); err != nil {
			t.Fatalf("add caveat: %v", err)
		}
	}

	raw, err := mac.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

func TestVerifyQuote(t *testing.T) {
	t.Parallel()

	var hash [32]byte
	copy(hash[:], bytes.Repeat([]byte{0x11}, 32))

	invoice, pubKey := makeInvoice(t, hash, 100)
	mac := makeMacaroon(t, hash,
		"services=weather:0",
		"weather_capabilities=forecast,historical",
		"weather_monthly_requests=500000",
	)

	req := &QuoteRequest{
		Service:      "weather",
		Capabilities: []string{"forecast"},
		Constraints: map[string]interface{}{
			"weather_monthly_requests": 500000,
		},
	}
	resp := &QuoteResponse{Macaroon: mac, Invoice: invoice}

	t.Run("happy path", func(t *testing.T) {
		err := VerifyQuote(resp, req, VerifyOptions{
			MaxCostSat: 1000,
			NodePubKey: pubKey,
		})
		if err != nil {
			t.Fatalf("verify: %v", err)
		}
	})

	t.Run("budget exceeded", func(t *testing.T) {
		err := VerifyQuote(resp, req, VerifyOptions{MaxCostSat: 10})
		if err == nil {
			t.Fatal("expected budget error")
		}
	})

	t.Run("wrong node", func(t *testing.T) {
		err := VerifyQuote(resp, req, VerifyOptions{
			NodePubKey: "02deadbeef",
		})
		if err == nil {
			t.Fatal("expected node pubkey error")
		}
	})

	t.Run("payment hash mismatch", func(t *testing.T) {
		var other [32]byte
		copy(other[:], bytes.Repeat([]byte{0x22}, 32))
		badMac := makeMacaroon(t, other, "services=weather:0")
		bad := &QuoteResponse{Macaroon: badMac, Invoice: invoice}
		if err := VerifyQuote(bad, req, VerifyOptions{}); err == nil {
			t.Fatal("expected hash mismatch error")
		}
	})

	t.Run("missing capability", func(t *testing.T) {
		weakMac := makeMacaroon(t, hash, "services=weather:0",
			"weather_capabilities=historical")
		weak := &QuoteResponse{Macaroon: weakMac, Invoice: invoice}
		if err := VerifyQuote(weak, req, VerifyOptions{}); err == nil {
			t.Fatal("expected capability error")
		}
	})

	t.Run("weaker constraint", func(t *testing.T) {
		weakMac := makeMacaroon(t, hash, "services=weather:0",
			"weather_capabilities=forecast,historical",
			"weather_monthly_requests=100000")
		weak := &QuoteResponse{Macaroon: weakMac, Invoice: invoice}
		if err := VerifyQuote(weak, req, VerifyOptions{}); err == nil {
			t.Fatal("expected weaker-constraint error")
		}
	})
}

// hexEncode is a tiny local hex encoder to avoid an extra import in tests.
func hexEncode(b []byte) string {
	const hexits = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, c := range b {
		out[i*2] = hexits[c>>4]
		out[i*2+1] = hexits[c&0x0f]
	}
	return string(out)
}
