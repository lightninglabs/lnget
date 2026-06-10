package discovery

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// httpMethods are the OpenAPI path-item keys that denote operations (as opposed
// to path-level fields like "parameters" or "summary").
var httpMethods = map[string]struct{}{
	"get": {}, "put": {}, "post": {}, "delete": {},
	"options": {}, "head": {}, "patch": {}, "trace": {},
}

// openAPIDoc is the subset of an OpenAPI 3.1 document we read, including the
// x-l402-* extension vocabulary.
type openAPIDoc struct {
	OpenAPI string `json:"openapi"`
	Info    struct {
		Title    string          `json:"title"`
		Version  string          `json:"x-l402-version"`
		Provider openAPIProvider `json:"x-l402-provider"`
	} `json:"info"`
	Servers []struct {
		URL string `json:"url"`
	} `json:"servers"`
	QuoteEndpoint string                                `json:"x-l402-quote-endpoint"`
	Caveats       map[string]CaveatSpec                 `json:"x-l402-caveats"`
	Paths         map[string]map[string]json.RawMessage `json:"paths"`
}

// openAPIProvider is the x-l402-provider extension.
type openAPIProvider struct {
	Name       string   `json:"name"`
	URI        string   `json:"uri"`
	NodePubKey string   `json:"node_pubkey"`
	Currencies []string `json:"currencies"`
}

// openAPIOp is the subset of an OpenAPI operation we read.
type openAPIOp struct {
	Service     string                `json:"x-l402-service"`
	Capability  string                `json:"x-l402-capability"`
	Tiers       []Tier                `json:"x-l402-tiers"`
	Pricing     *Pricing              `json:"x-l402-pricing"`
	Constraints map[string]Constraint `json:"x-l402-constraints"`
}

// parseOpenAPI parses an OpenAPI document with x-l402-* extensions into the
// common Manifest shape.
func parseOpenAPI(body []byte) (*Manifest, error) {
	var doc openAPIDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("invalid OpenAPI document: %w", err)
	}

	name := doc.Info.Provider.Name
	if name == "" {
		name = doc.Info.Title
	}
	uri := doc.Info.Provider.URI
	if uri == "" && len(doc.Servers) > 0 {
		uri = doc.Servers[0].URL
	}

	m := &Manifest{
		Version: doc.Info.Version,
		Provider: Provider{
			Name:       name,
			URI:        uri,
			NodePubKey: doc.Info.Provider.NodePubKey,
		},
		Currencies:    doc.Info.Provider.Currencies,
		QuoteEndpoint: doc.QuoteEndpoint,
		Caveats:       doc.Caveats,
		Profile:       "openapi",
	}

	// Group operations by their x-l402-service.
	byService := make(map[string]*ServiceEntry)
	for path, item := range doc.Paths {
		for method, raw := range item {
			if _, ok := httpMethods[strings.ToLower(method)]; !ok {
				continue
			}

			var op openAPIOp
			if err := json.Unmarshal(raw, &op); err != nil {
				continue
			}
			if op.Service == "" {
				continue
			}

			svc := byService[op.Service]
			if svc == nil {
				svc = &ServiceEntry{Name: op.Service}
				byService[op.Service] = svc
			}
			if len(op.Tiers) > 0 && len(svc.Tiers) == 0 {
				svc.Tiers = op.Tiers
			}
			if op.Capability != "" && !contains(svc.Capabilities, op.Capability) {
				svc.Capabilities = append(svc.Capabilities, op.Capability)
			}

			pricing := Pricing{}
			if op.Pricing != nil {
				pricing = *op.Pricing
			}
			svc.Resources = append(svc.Resources, ResourceEntry{
				Path:        path,
				Method:      strings.ToUpper(method),
				Capability:  op.Capability,
				Pricing:     pricing,
				Constraints: op.Constraints,
			})
		}
	}

	// Emit services in a deterministic order.
	names := make([]string, 0, len(byService))
	for name := range byService {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		m.Services = append(m.Services, *byService[name])
	}

	return m, nil
}

// contains reports whether s is in the slice.
func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
