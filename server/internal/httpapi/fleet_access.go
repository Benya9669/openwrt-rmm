package httpapi

import (
	"errors"
	"net/http"
	"rmm-openwrt/server/internal/store"
)

func (a *App) handleFleetAccessPolicy(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFromContext(r.Context())
	if r.Method == http.MethodGet {
		policy, err := a.store.GetFleetAccessPolicy(r.Context(), principal.User.ID, r.PathValue("id"))
		if errors.Is(err, store.ErrFleetAccess) {
			writeError(w, http.StatusNotFound, "device unavailable")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load access policy")
			return
		}
		if !principal.IsAdmin() {
			policy.UserIDs = nil
		}
		writeJSON(w, http.StatusOK, policy)
		return
	}
	var policy store.FleetAccessPolicy
	if !decodeJSON(w, r, &policy) {
		return
	}
	policy.DeviceID = r.PathValue("id")
	if err := a.store.SaveFleetAccessPolicy(r.Context(), principal.User.ID, policy); err != nil {
		if errors.Is(err, store.ErrFleetAccess) {
			writeError(w, http.StatusNotFound, "device unavailable")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid access policy or user list")
		return
	}
	a.events.publish("devices")
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}
