// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package twilio

import (
	"fmt"
	"net/http"
	"text/template"

	"eva-mind/internal/brainstem/config"
)

type TwimlHandlerFunc struct {
	cfg *config.Config
}

func NewTwimlHandler(cfg *config.Config) *TwimlHandlerFunc {
	return &TwimlHandlerFunc{cfg: cfg}
}

// ✅ TwimlHandler corrigido para usar ServiceDomain da config
func (h *TwimlHandlerFunc) TwimlHandler(w http.ResponseWriter, r *http.Request) {
	agID := r.URL.Query().Get("agendamento_id")
	if agID == "" {
		http.Error(w, "agendamento_id obrigatório", http.StatusBadRequest)
		return
	}

	// ✅ URL correta do WebSocket usando ServiceDomain da config
	streamURL := fmt.Sprintf("wss://%s/calls/stream/%s", h.cfg.ServiceDomain, agID)

	tmpl := `<?xml version="1.0" encoding="UTF-8"?>
<Response>
  <Connect>
    <Stream url="{{.StreamURL}}" />
  </Connect>
</Response>`

	t, err := template.New("twiml").Parse(tmpl)
	if err != nil {
		http.Error(w, "Erro interno", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	err = t.Execute(w, struct{ StreamURL string }{StreamURL: streamURL})
	if err != nil {
		http.Error(w, "Erro ao gerar TwiML", http.StatusInternalServerError)
	}
}
