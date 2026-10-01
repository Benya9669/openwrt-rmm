package httpapi

import (
	"errors"
	"net/http"
	"rmm-openwrt/server/internal/store"
)

func (a *App) handleFleetProfiles(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFromContext(r.Context())
	if r.Method == http.MethodGet {
		profiles, err := a.store.ListFleetProfiles(r.Context(), principal.User.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load profiles")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"profiles": profiles})
		return
	}
	var profile store.FleetProfile
	if !decodeJSON(w, r, &profile) {
		return
	}
	profile.ID = r.PathValue("id")
	result, err := a.store.SaveFleetProfile(r.Context(), principal.User.ID, profile)
	if errors.Is(err, store.ErrFleetAccess) {
		writeError(w, http.StatusNotFound, "profile unavailable")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid profile section, option or value; keep credentials in the credential workflow")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) handleFleetProfileOperation(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFromContext(r.Context())
	var input store.FleetProfileOperation
	if !decodeJSON(w, r, &input) {
		return
	}
	operation, err := a.store.CreateFleetProfileOperation(r.Context(), principal.User.ID, input)
	if errors.Is(err, store.ErrFleetFeatureUnavailable) {
		writeError(w, http.StatusConflict, "update agent before using UCI profiles")
		return
	}
	if errors.Is(err, store.ErrFleetAccess) {
		writeError(w, http.StatusNotFound, "profile or device unavailable")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "profile operation unavailable; verify a recent successful preview for every target")
		return
	}
	writeJSON(w, http.StatusAccepted, operation)
}
