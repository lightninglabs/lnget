package api

import (
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"sort"

	"github.com/lightninglabs/lnget/client"
	"github.com/lightninglabs/lnget/l402"
)

// handleListTokens returns all cached L402 tokens.
// GET /api/tokens.
func (s *Server) handleListTokens(w http.ResponseWriter,
	r *http.Request) {

	tokens, err := s.tokenStore.AllTokens()
	if err != nil {
		log.Printf("error listing tokens: %v", err)
		writeError(w, http.StatusInternalServerError,
			"internal server error")

		return
	}

	var infos []client.TokenInfo
	for domain, token := range tokens {
		infos = append(infos, tokenToInfo(domain, token))
	}

	if infos == nil {
		infos = []client.TokenInfo{}
	}

	// Sort by domain for deterministic output.
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Domain < infos[j].Domain
	})

	writeJSON(w, http.StatusOK, infos)
}

// handleShowToken returns the token for a specific domain.
// GET /api/tokens/{domain}.
func (s *Server) handleShowToken(w http.ResponseWriter,
	r *http.Request) {

	domain := r.PathValue("domain")
	if domain == "" {
		writeError(w, http.StatusBadRequest, "domain is required")
		return
	}

	token, err := s.tokenStore.GetToken(domain)
	if err != nil {
		if errors.Is(err, l402.ErrNoToken) {
			writeError(w, http.StatusNotFound,
				"no token for domain")
		} else {
			//nolint:gosec // G706: domain is from URL path, err is internal.
			log.Printf("error getting token for %s: %v",
				domain, err)
			writeError(w, http.StatusInternalServerError,
				"internal server error")
		}

		return
	}

	writeJSON(w, http.StatusOK, tokenToInfo(domain, token))
}

// handleRemoveToken removes the token for a specific domain.
// DELETE /api/tokens/{domain}.
func (s *Server) handleRemoveToken(w http.ResponseWriter,
	r *http.Request) {

	domain := r.PathValue("domain")
	if domain == "" {
		writeError(w, http.StatusBadRequest, "domain is required")
		return
	}

	err := s.tokenStore.RemoveToken(domain)
	if err != nil {
		//nolint:gosec // G706: domain is from URL path, err is internal.
		log.Printf("error removing token for %s: %v", domain, err)
		writeError(w, http.StatusInternalServerError,
			"internal server error")

		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "removed",
		"domain": domain,
	})
}

// tokenToInfo converts a Token to a TokenInfo struct.
func tokenToInfo(domain string, token *l402.Token) client.TokenInfo {
	return client.TokenInfo{
		Domain:      domain,
		PaymentHash: hex.EncodeToString(token.PaymentHash[:]),
		AmountSat:   (int64(token.AmountPaid) + 500) / 1000,
		FeeSat:      (int64(token.RoutingFeePaid) + 500) / 1000,
		Created:     token.TimeCreated.Format("2006-01-02 15:04:05"),
		Pending:     l402.IsPending(token),
	}
}
