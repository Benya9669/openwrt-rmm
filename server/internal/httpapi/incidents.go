package httpapi

import (
	"errors"
	"net/http"
	"rmm-openwrt/server/internal/store"
)

func (a *App) handleIncidents(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFromContext(r.Context())
	if r.Method == http.MethodGet {
		incidents, err := a.store.ListIncidents(r.Context(), p.User.ID)
		if err != nil {
			writeError(w, 500, "failed to load incidents")
			return
		}
		writeJSON(w, 200, map[string]any{"incidents": incidents})
		return
	}
	var input struct {
		Action string `json:"action"`
		Body   string `json:"body"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := a.store.UpdateIncident(r.Context(), p.User.ID, r.PathValue("id"), input.Action, input.Body); err != nil {
		if errors.Is(err, store.ErrFleetAccess) {
			writeError(w, 404, "incident or assignee unavailable")
		} else {
			writeError(w, 400, "invalid incident action")
		}
		return
	}
	writeJSON(w, 200, map[string]string{"status": "updated"})
}
