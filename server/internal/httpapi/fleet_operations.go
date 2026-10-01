package httpapi

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"rmm-openwrt/server/internal/store"
)

func (a *App) handleFleetOperations(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFromContext(r.Context())
	if r.Method == http.MethodGet {
		operations, err := a.store.ListFleetOperations(r.Context(), principal.User.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load fleet operations")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"operations": operations})
		return
	}
	var req struct {
		store.FleetOperationInput
		Group string `json:"group"`
		Tag   string `json:"tag"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(req.DeviceIDs) == 0 && (req.Group != "" || req.Tag != "") {
		devices, err := a.store.ListDevicesForUser(r.Context(), principal.User.ID, principal.IsAdmin())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to resolve operation scope")
			return
		}
		for _, device := range devices {
			if req.Group != "" && device.Group != req.Group {
				continue
			}
			matches := req.Tag == ""
			for _, tag := range device.Tags {
				if tag == req.Tag {
					matches = true
				}
			}
			if matches {
				req.DeviceIDs = append(req.DeviceIDs, device.ID)
			}
		}
	}
	req.Type = strings.TrimSpace(req.Type)
	if req.Type == "agent_update" {
		writeError(w, http.StatusBadRequest, "use managed update schedules or the existing rollout workflow")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	operation, err := a.store.CreateFleetOperation(r.Context(), principal.User.ID, req.FleetOperationInput)
	if errors.Is(err, store.ErrFleetFeatureUnavailable) {
		writeError(w, http.StatusConflict, "update the agent before using this fleet feature")
		return
	}
	if errors.Is(err, store.ErrFleetAccess) {
		writeError(w, http.StatusNotFound, "operation device is unavailable")
		return
	}
	if errors.Is(err, store.ErrFleetConflict) {
		writeError(w, http.StatusConflict, "request key already belongs to another operation")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "operation could not be created; check limits, targets and arguments")
		return
	}
	_, err = a.store.AddAuditEvent(r.Context(), actorName(r), "fleet.operation_create", "", "", mustJSON(map[string]string{"operation_id": operation.ID, "request_id": requestID(r.Context())}))
	if err != nil {
		log.Printf("fleet operation audit failed: %v", err)
	}
	a.events.publish("devices")
	writeJSON(w, http.StatusAccepted, operation)
}

func (a *App) handleFleetOperationAction(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFromContext(r.Context())
	if err := a.store.SetFleetOperationAction(r.Context(), principal.User.ID, r.PathValue("id"), r.PathValue("action")); err != nil {
		if errors.Is(err, store.ErrFleetAccess) {
			writeError(w, http.StatusNotFound, "operation not found")
			return
		}
		writeError(w, http.StatusConflict, "operation action is unavailable")
		return
	}
	if _, err := a.store.AddAuditEvent(r.Context(), actorName(r), "fleet.operation_"+r.PathValue("action"), "", "", mustJSON(map[string]string{"operation_id": r.PathValue("id"), "request_id": requestID(r.Context())})); err != nil {
		log.Printf("fleet action audit failed: %v", err)
	}
	a.events.publish("devices")
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func (a *App) fleetOperationsLoop(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		workCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		if err := a.store.AdvanceFleetRules(workCtx); err != nil && ctx.Err() == nil {
			log.Printf("advance fleet reactions failed: %v", err)
		}
		if err := a.store.AdvanceFleetSchedules(workCtx); err != nil && ctx.Err() == nil {
			log.Printf("advance fleet schedules failed: %v", err)
		}
		if err := a.store.AdvanceGuardedRollouts(workCtx); err != nil && ctx.Err() == nil {
			log.Printf("advance guarded rollout failed: %v", err)
		}
		err := a.store.AdvanceFleetOperations(workCtx)
		cancel()
		if err != nil && ctx.Err() == nil {
			log.Printf("advance fleet operations failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
