// Package discovery implements the client side of the L402 discovery layer:
// fetching and parsing a provider's manifest (both the bespoke JSON and the
// self-describing OpenAPI profile), requesting a quote for a bundle, and
// verifying the returned challenge before paying. See the L402 discovery
// specification for the wire format.
package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	// ManifestPath is the well-known path the manifest is served at.
	ManifestPath = "/.well-known/l402.json"

	// manifestLinkRel is the Link relation a provider may use to advertise
	// a manifest at a non-default location.
	manifestLinkRel = "l402-manifest"

	// maxManifestBytes bounds the manifest body we will read.
	maxManifestBytes = 1 << 20
)

// ErrNoManifest is returned when a provider exposes no discovery manifest.
var ErrNoManifest = errors.New("no L402 discovery manifest found")

// DoFunc performs an HTTP request. It is satisfied by a thin wrapper around the
// lnget payment-aware client, and by httptest in tests.
type DoFunc func(req *http.Request) (*http.Response, error)

// Provider identifies the manifest's provider.
type Provider struct {
	Name       string `json:"name"`
	URI        string `json:"uri,omitempty"`
	NodePubKey string `json:"node_pubkey,omitempty"`
}

// Manifest is the parsed discovery manifest. It is the common shape both the
// bespoke and OpenAPI profiles are parsed into.
type Manifest struct {
	Version        string                `json:"version"`
	Provider       Provider              `json:"provider"`
	Currencies     []string              `json:"currencies,omitempty"`
	PaymentMethods []string              `json:"payment_methods,omitempty"`
	QuoteEndpoint  string                `json:"quote_endpoint,omitempty"`
	OpenAPI        string                `json:"openapi,omitempty"`
	Services       []ServiceEntry        `json:"services"`
	Caveats        map[string]CaveatSpec `json:"caveats,omitempty"`

	// Profile records which encoding the manifest was parsed from
	// ("bespoke" or "openapi"). It is not part of the wire format.
	Profile string `json:"-"`
}

// ServiceEntry describes one service.
type ServiceEntry struct {
	Name         string          `json:"name"`
	Description  string          `json:"description,omitempty"`
	Tiers        []Tier          `json:"tiers,omitempty"`
	Capabilities []string        `json:"capabilities,omitempty"`
	Resources    []ResourceEntry `json:"resources,omitempty"`
}

// Tier is one service tier.
type Tier struct {
	Tier int    `json:"tier"`
	Name string `json:"name,omitempty"`
}

// ResourceEntry is a priced resource of a service.
type ResourceEntry struct {
	Path        string                `json:"path,omitempty"`
	Method      string                `json:"method,omitempty"`
	Capability  string                `json:"capability,omitempty"`
	Pricing     Pricing               `json:"pricing"`
	Constraints map[string]Constraint `json:"constraints,omitempty"`
}

// Pricing describes how a resource is priced.
type Pricing struct {
	Model      string             `json:"model"`
	PriceMsat  int64              `json:"price_msat,omitempty"`
	BaseMsat   int64              `json:"base_msat,omitempty"`
	Components []FormulaComponent `json:"components,omitempty"`
}

// FormulaComponent is one per-unit charge in a formula price.
type FormulaComponent struct {
	Constraint       string `json:"constraint"`
	PriceMsatPerUnit int64  `json:"price_msat_per_unit"`
	Unit             int64  `json:"unit,omitempty"`
}

// Constraint describes the permissible bounds of a constraint.
type Constraint struct {
	Type   string   `json:"type"`
	Max    *int64   `json:"max,omitempty"`
	Min    *int64   `json:"min,omitempty"`
	Values []string `json:"values,omitempty"`
}

// CaveatSpec describes a caveat condition the provider enforces.
type CaveatSpec struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Attenuation string `json:"attenuation,omitempty"`
}

// ComputeMsat computes the price in millisatoshis for the given integer
// constraint values, matching the server's formula computation. For a fixed
// price it returns PriceMsat; for a dynamic model it returns 0, since the price
// is only known via a quote.
func (p Pricing) ComputeMsat(values map[string]int64) int64 {
	if p.Model != "formula" {
		return p.PriceMsat
	}

	price := p.BaseMsat
	for _, c := range p.Components {
		value, ok := values[c.Constraint]
		if !ok || value <= 0 {
			continue
		}
		unit := c.Unit
		if unit <= 0 {
			unit = 1
		}
		units := (value + unit - 1) / unit
		price += c.PriceMsatPerUnit * units
	}
	return price
}

// FetchManifest fetches and parses a provider's discovery manifest. It tries
// the well-known path first, then a Link: rel="l402-manifest" header on the
// base URL. It returns ErrNoManifest if neither is found, so the caller can
// fall back to the reactive 402 flow.
func FetchManifest(ctx context.Context, do DoFunc, baseURL string) (*Manifest,
	error) {

	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Try the well-known path.
	wellKnown := (&url.URL{
		Scheme: base.Scheme,
		Host:   base.Host,
		Path:   ManifestPath,
	}).String()

	body, status, _, err := fetch(ctx, do, http.MethodGet, wellKnown, nil)
	if err == nil && status == http.StatusOK {
		return ParseManifest(body)
	}

	// Fall back to a Link header on the base URL.
	_, _, header, err := fetch(ctx, do, http.MethodGet, baseURL, nil)
	if err == nil {
		if link := manifestLink(header, base); link != "" {
			body, status, _, err := fetch(
				ctx, do, http.MethodGet, link, nil,
			)
			if err == nil && status == http.StatusOK {
				return ParseManifest(body)
			}
		}
	}

	return nil, ErrNoManifest
}

// ParseManifest parses a manifest body, detecting the bespoke or OpenAPI
// profile automatically.
func ParseManifest(body []byte) (*Manifest, error) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(body, &probe); err != nil {
		return nil, fmt.Errorf("invalid manifest JSON: %w", err)
	}

	// An OpenAPI document has top-level "openapi" and "paths" members; the
	// bespoke manifest has "services".
	_, hasOpenAPI := probe["openapi"]
	_, hasPaths := probe["paths"]
	if hasOpenAPI && hasPaths {
		return parseOpenAPI(body)
	}

	var m Manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}
	m.Profile = "bespoke"
	return &m, nil
}

// FindService returns the service entry with the given name, or nil.
func (m *Manifest) FindService(name string) *ServiceEntry {
	for i := range m.Services {
		if m.Services[i].Name == name {
			return &m.Services[i]
		}
	}
	return nil
}

// fetch performs a request and returns the body, status, and headers.
func fetch(ctx context.Context, do DoFunc, method, rawURL string,
	body io.Reader) ([]byte, int, http.Header, error) {

	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := do(req)
	if err != nil {
		return nil, 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxManifestBytes))
	if err != nil {
		return nil, resp.StatusCode, resp.Header, err
	}

	return data, resp.StatusCode, resp.Header, nil
}

// manifestLink extracts a manifest URL from a Link header, resolving it against
// the base URL.
func manifestLink(header http.Header, base *url.URL) string {
	for _, link := range header.Values("Link") {
		if !strings.Contains(link, manifestLinkRel) {
			continue
		}

		// Link header form: <url>; rel="l402-manifest".
		start := strings.Index(link, "<")
		end := strings.Index(link, ">")
		if start < 0 || end <= start {
			continue
		}

		ref, err := url.Parse(link[start+1 : end])
		if err != nil {
			continue
		}
		return base.ResolveReference(ref).String()
	}
	return ""
}
