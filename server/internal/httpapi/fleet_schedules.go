package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"rmm-openwrt/server/internal/store"
)

func (a *App) handleFleetSchedules(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFromContext(r.Context())
	if r.Method == http.MethodGet {
		schedules, err := a.store.ListFleetSchedules(r.Context(), principal.User.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load schedules")
			return
		}
		runs, err := a.store.ListFleetScheduleRuns(r.Context(), principal.User.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load schedule runs")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"schedules": schedules, "runs": runs})
		return
	}
	var input store.FleetSchedule
	if !decodeJSON(w, r, &input) {
		return
	}
	input.ID = r.PathValue("id")
	if input.Operation.Type == "agent_update" {
		if a.compatibleAgentFeed == nil {
			writeError(w, http.StatusConflict, "managed update feed unavailable")
			return
		}
		resolved := store.FleetUpdateArgs{Targets: map[string]map[string]string{}}
		for _, id := range input.Operation.DeviceIDs {
			if !a.authorizeOperatorDevice(w, r, id) {
				return
			}
			device, found, err := a.store.GetDevice(r.Context(), id)
			if err != nil || !found {
				writeError(w, http.StatusNotFound, "device unavailable")
				return
			}
			var inventory struct {
				Runtime string `json:"agent_runtime"`
				Package string `json:"agent_package"`
				Version string `json:"agent_version"`
				Release string `json:"openwrt_release"`
				Target  string `json:"target"`
				Manager string `json:"package_manager"`
			}
			if json.Unmarshal(device.Inventory, &inventory) != nil || inventory.Runtime != "go" || inventory.Package != "rmm-agent-go-production" || compareSemver(inventory.Version, "0.6.10") < 0 {
				writeError(w, http.StatusConflict, "signed managed updates require a compatible agent")
				return
			}
			feed, ok := a.compatibleAgentFeed(inventory.Release, inventory.Target, inventory.Manager)
			if !ok {
				writeError(w, http.StatusConflict, "compatible immutable update unavailable")
				return
			}
			if !agentSupportsFeed(device.Inventory, feed) {
				writeError(w, http.StatusConflict, "agent requires a native package update for OpenWrt 25.12 release-line support")
				return
			}
			args := agentPackageCommandArgs(feed, inventory.Manager, inventory.Version)
			args["expected_release"] = inventory.Release
			args["expected_target"] = inventory.Target
			resolved.Targets[id] = args
		}
		input.Operation.Args = mustJSON(resolved)
		input.Operation.StopOnFailure = true
	}
	schedule, err := a.store.SaveFleetSchedule(r.Context(), principal.User.ID, input)
	if errors.Is(err, store.ErrFleetConflict) {
		writeError(w, http.StatusConflict, "request key already belongs to another schedule definition")
		return
	}
	if errors.Is(err, store.ErrFleetAccess) {
		writeError(w, http.StatusNotFound, "schedule or device unavailable")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid schedule, timezone, window or operation")
		return
	}
	writeJSON(w, http.StatusOK, schedule)
}
