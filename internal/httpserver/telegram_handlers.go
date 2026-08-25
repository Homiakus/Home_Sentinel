package httpserver

import (
	"context"
	"net/http"
	"time"
)

func (s *Server) telegramStatus(w http.ResponseWriter, r *http.Request) {
	if s.app.Telegram == nil {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": false})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	bot, err := s.app.Telegram.Client.GetMe(ctx)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": true, "reachable": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"enabled": true, "reachable": true, "bot": map[string]any{"id": bot.ID, "username": bot.Username, "first_name": bot.FirstName}})
}
func (s *Server) telegramPairing(w http.ResponseWriter, r *http.Request) {
	if s.app.Telegram == nil {
		writeProblem(w, r, http.StatusServiceUnavailable, "TELEGRAM_DISABLED", "Telegram integration is not enabled")
		return
	}
	p, _ := principalFrom(r.Context())
	code, exp, err := s.app.Telegram.Pairings.Create(r.Context(), p.User.ID, 10*time.Minute)
	if err != nil {
		writeProblem(w, r, http.StatusInternalServerError, "TELEGRAM_PAIRING_FAILED", "Unable to create Telegram pairing code")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"code": code, "expires_at": exp, "command": "/start " + code})
}
