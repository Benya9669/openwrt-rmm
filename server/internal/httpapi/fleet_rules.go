package httpapi

import (
	"errors"
	"net/http"
	"rmm-openwrt/server/internal/store"
)

func (a *App) handleFleetRules(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFromContext(r.Context())
	if r.Method == http.MethodGet {
		rules, err := a.store.ListFleetRules(r.Context(), principal.User.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load reaction rules")
			return
		}
		events, err := a.store.ListFleetRuleEvents(r.Context(), principal.User.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load reaction events")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"rules": rules, "events": events})
		return
	}
	var rule store.FleetRule
	if !decodeJSON(w, r, &rule) {
		return
	}
	rule.ID = r.PathValue("id")
	result, err := a.store.SaveFleetRule(r.Context(), principal.User.ID, rule)
	if errors.Is(err, store.ErrFleetConflict) {
		writeError(w, http.StatusConflict, "request key already belongs to another rule definition")
		return
	}
	if errors.Is(err, store.ErrFleetAccess) {
		writeError(w, http.StatusNotFound, "rule or device unavailable")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid condition, scope or safety limits; automatic restart allows only dnsmasq and uhttpd")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
