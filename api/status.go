package api

import (
	"log"
	"net/http"

	"github.com/lightninglabs/lnget/client"
)

// handleStatus returns the Lightning backend connection status.
// GET /api/status.
func (s *Server) handleStatus(w http.ResponseWriter,
	r *http.Request) {

	status := client.BackendStatus{
		Type:      string(s.cfg.LN.Mode),
		Connected: false,
	}

	if s.backend != nil {
		info, err := s.backend.GetInfo(r.Context())
		if err != nil {
			log.Printf("error getting backend info: %v", err)
			status.Error = "backend unavailable"
		} else {
			status.Connected = true
			status.NodePubKey = info.NodePubKey
			status.Alias = info.Alias
			status.Network = info.Network
			status.SyncedToChain = info.SyncedToChain
			status.BalanceSat = info.Balance
		}
	}

	writeJSON(w, http.StatusOK, status)
}

// configResponse is the redacted config sent to the dashboard.
type configResponse struct {
	LNMode         string `json:"ln_mode"`
	MaxCostSats    int64  `json:"max_cost_sats"`
	MaxFeeSats     int64  `json:"max_fee_sats"`
	PaymentTimeout string `json:"payment_timeout"`
	AutoPay        bool   `json:"auto_pay"`
	EventsEnabled  bool   `json:"events_enabled"`
	TokenDir       string `json:"token_dir"`
}

// handleConfig returns the current config with sensitive fields redacted.
// GET /api/config.
func (s *Server) handleConfig(w http.ResponseWriter,
	r *http.Request) {

	resp := configResponse{
		LNMode:         string(s.cfg.LN.Mode),
		MaxCostSats:    s.cfg.L402.MaxCostSats,
		MaxFeeSats:     s.cfg.L402.MaxFeeSats,
		PaymentTimeout: s.cfg.L402.PaymentTimeout.String(),
		AutoPay:        s.cfg.L402.AutoPay,
		EventsEnabled:  s.cfg.Events.Enabled,
		TokenDir:       s.cfg.Tokens.Dir,
	}

	writeJSON(w, http.StatusOK, resp)
}
