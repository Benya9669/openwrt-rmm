package httpapi

import (
	"errors"
	"log"
	"net/http"
	"rmm-openwrt/server/internal/store"
)

func (a *App) handleFleetAssets(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFromContext(r.Context())
	if r.Method == http.MethodGet {
		assets, err := a.store.ListFleetAssets(r.Context(), principal.User.ID, r.URL.Query().Get("q"), r.URL.Query().Get("site"))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load equipment")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"assets": assets})
		return
	}
	var asset store.FleetAsset
	if !decodeJSON(w, r, &asset) {
		return
	}
	asset.DeviceID = r.PathValue("id")
	if err := a.store.SaveFleetAsset(r.Context(), principal.User.ID, asset); err != nil {
		if errors.Is(err, store.ErrFleetAccess) {
			writeError(w, http.StatusNotFound, "device not found")
			return
		}
		writeError(w, http.StatusBadRequest, "equipment details are invalid")
		return
	}
	if _, err := a.store.AddAuditEvent(r.Context(), actorName(r), "fleet.asset_update", asset.DeviceID, "", mustJSON(map[string]string{"request_id": requestID(r.Context())})); err != nil {
		log.Printf("equipment audit failed: %v", err)
	}
	a.events.publish("devices")
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}
