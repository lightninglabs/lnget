package api

import (
	"log"
	"net/http"
	"strconv"

	"github.com/lightninglabs/lnget/events"
)

// MaxPageSize is the maximum number of events that can be returned in a
// single list request to prevent unbounded queries.
const MaxPageSize = 1000

// handleListEvents returns a paginated list of payment events.
// GET /api/events?limit=50&offset=0&domain=example.com&status=success.
func (s *Server) handleListEvents(w http.ResponseWriter,
	r *http.Request) {
	opts := events.ListOpts{
		Limit:  50,
		Domain: r.URL.Query().Get("domain"),
		Status: r.URL.Query().Get("status"),
	}

	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			opts.Limit = n
		}
	}

	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			opts.Offset = n
		}
	}

	// Clamp the limit to prevent unbounded queries.
	if opts.Limit > MaxPageSize {
		opts.Limit = MaxPageSize
	}

	evts, err := s.eventStore.ListEvents(r.Context(), opts)
	if err != nil {
		log.Printf("error listing events: %v", err)
		writeError(w, http.StatusInternalServerError,
			"internal server error")

		return
	}

	if evts == nil {
		evts = []*events.Event{}
	}

	writeJSON(w, http.StatusOK, evts)
}

// handleEventStats returns aggregate spending statistics.
// GET /api/events/stats.
func (s *Server) handleEventStats(w http.ResponseWriter,
	r *http.Request) {
	stats, err := s.eventStore.GetStats(r.Context())
	if err != nil {
		log.Printf("error getting stats: %v", err)
		writeError(w, http.StatusInternalServerError,
			"internal server error")

		return
	}

	// Include active token count from the token store.
	tokens, err := s.tokenStore.AllTokens()
	if err == nil {
		stats.ActiveTokens = len(tokens)
	}

	writeJSON(w, http.StatusOK, stats)
}

// handleDomainSpending returns per-domain spending breakdowns.
// GET /api/events/domains.
func (s *Server) handleDomainSpending(w http.ResponseWriter,
	r *http.Request) {
	domains, err := s.eventStore.GetSpendingByDomain(r.Context())
	if err != nil {
		log.Printf("error getting domain spending: %v", err)
		writeError(w, http.StatusInternalServerError,
			"internal server error")

		return
	}

	if domains == nil {
		domains = []*events.DomainSpending{}
	}

	writeJSON(w, http.StatusOK, domains)
}
