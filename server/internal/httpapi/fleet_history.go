package httpapi

import (
	"encoding/json"
	"math"
	"net/http"
	"rmm-openwrt/server/internal/model"
	"rmm-openwrt/server/internal/store"
	"sort"
	"strconv"
	"strings"
	"time"
)

type fleetHistoryPoint struct {
	At            time.Time `json:"at"`
	Load          *float64  `json:"load,omitempty"`
	MemoryPercent *float64  `json:"memory_percent,omitempty"`
	WANRXBPS      *float64  `json:"wan_rx_bps,omitempty"`
	WANTXBPS      *float64  `json:"wan_tx_bps,omitempty"`
	WANIP         string    `json:"wan_ip,omitempty"`
}
type fleetHistoryEvent struct {
	At      time.Time `json:"at"`
	Type    string    `json:"type"`
	Message string    `json:"message"`
}
type fleetHistory struct {
	Points               []fleetHistoryPoint `json:"points"`
	Events               []fleetHistoryEvent `json:"events"`
	ObservedAvailability *float64            `json:"observed_availability_percent,omitempty"`
	AvailabilityMethod   string              `json:"availability_method"`
}

func finiteFloat(value string) *float64 {
	number, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) || number < 0 {
		return nil
	}
	return &number
}
func deriveFleetHistory(samples []model.MetricSample, now time.Time) fleetHistory {
	ordered := append([]model.MetricSample(nil), samples...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].CreatedAt.Before(ordered[j].CreatedAt) })
	result := fleetHistory{Points: []fleetHistoryPoint{}, Events: []fleetHistoryEvent{}, AvailabilityMethod: "heartbeat observed; gaps longer than 120 seconds are unavailable; absence of telemetry does not prove router downtime"}
	var previousUptime *float64
	var previousIP, previousIface string
	var previousAt time.Time
	var previousRX, previousTX *float64
	var covered time.Duration
	for _, sample := range ordered {
		if sample.CreatedAt.After(now) {
			continue
		}
		var inventory struct {
			WANIP string `json:"wan_ip"`
			Route string `json:"default_route"`
		}
		var metrics struct {
			Load   string `json:"loadavg"`
			Uptime string `json:"uptime"`
			Memory struct {
				Total float64 `json:"total_kb"`
				Used  float64 `json:"used_kb"`
			} `json:"memory"`
			Counters []struct {
				Name string  `json:"name"`
				RX   float64 `json:"rx_bytes"`
				TX   float64 `json:"tx_bytes"`
			} `json:"interface_counters"`
		}
		if json.Unmarshal(sample.Inventory, &inventory) != nil || json.Unmarshal(sample.Metrics, &metrics) != nil {
			continue
		}
		point := fleetHistoryPoint{At: sample.CreatedAt, WANIP: inventory.WANIP}
		if fields := strings.Fields(metrics.Load); len(fields) > 0 {
			point.Load = finiteFloat(fields[0])
		}
		var uptime *float64
		if fields := strings.Fields(metrics.Uptime); len(fields) > 0 {
			uptime = finiteFloat(fields[0])
		}
		if metrics.Memory.Total > 0 && metrics.Memory.Used >= 0 && metrics.Memory.Used <= metrics.Memory.Total {
			percent := metrics.Memory.Used / metrics.Memory.Total * 100
			point.MemoryPercent = &percent
		}
		reboot := previousUptime != nil && uptime != nil && *uptime < *previousUptime
		if reboot {
			result.Events = append(result.Events, fleetHistoryEvent{sample.CreatedAt, "reboot_observed", "Уменьшился uptime устройства; вероятная перезагрузка"})
		}
		if previousIP != "" && inventory.WANIP != "" && inventory.WANIP != previousIP {
			result.Events = append(result.Events, fleetHistoryEvent{sample.CreatedAt, "wan_ip_changed", previousIP + " → " + inventory.WANIP})
		}
		if !previousAt.IsZero() {
			gap := sample.CreatedAt.Sub(previousAt)
			covered += min(gap, 2*time.Minute)
			if gap > 2*time.Minute {
				result.Events = append(result.Events, fleetHistoryEvent{previousAt.Add(2 * time.Minute), "telemetry_gap", "Нет heartbeat"}, fleetHistoryEvent{sample.CreatedAt, "heartbeat_restored", "Получен heartbeat"})
			}
		}
		fields := strings.Fields(inventory.Route)
		iface := ""
		for i := 0; i+1 < len(fields); i++ {
			if fields[i] == "dev" {
				iface = fields[i+1]
				break
			}
		}
		var rx, tx *float64
		for _, counter := range metrics.Counters {
			if counter.Name == iface && iface != "" {
				rx = &counter.RX
				tx = &counter.TX
				break
			}
		}
		elapsed := sample.CreatedAt.Sub(previousAt).Seconds()
		if !reboot && iface != "" && iface == previousIface && rx != nil && tx != nil && previousRX != nil && previousTX != nil && elapsed > 0 && elapsed <= 120 && *rx >= *previousRX && *tx >= *previousTX {
			receive := (*rx - *previousRX) * 8 / elapsed
			transmit := (*tx - *previousTX) * 8 / elapsed
			point.WANRXBPS = &receive
			point.WANTXBPS = &transmit
		}
		result.Points = append(result.Points, point)
		previousUptime = uptime
		previousIP = inventory.WANIP
		previousIface = iface
		previousAt = sample.CreatedAt
		previousRX = rx
		previousTX = tx
	}
	if len(result.Points) > 0 && now.After(result.Points[0].At) {
		covered += min(now.Sub(previousAt), 2*time.Minute)
		if now.Sub(previousAt) > 2*time.Minute {
			result.Events = append(result.Events, fleetHistoryEvent{previousAt.Add(2 * time.Minute), "telemetry_gap", "Нет heartbeat"})
		}
		availability := float64(covered) / float64(now.Sub(result.Points[0].At)) * 100
		result.ObservedAvailability = &availability
	}
	sort.SliceStable(result.Events, func(i, j int) bool { return result.Events[i].At.Before(result.Events[j].At) })
	return result
}
func (a *App) handleFleetHistory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !a.authorizeOperatorDevice(w, r, id) {
		return
	}
	samples, found, err := a.store.ListMetricSamples(r.Context(), id, store.MetricHistoryOptions{Limit: 500})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load device history")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "device unavailable")
		return
	}
	history := deriveFleetHistory(samples, time.Now().UTC())
	events, err := a.store.ListAuditEvents(r.Context(), store.AuditListOptions{DeviceID: id, Limit: 100})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load device timeline")
		return
	}
	for _, event := range events {
		history.Events = append(history.Events, fleetHistoryEvent{event.CreatedAt, "audit", event.Action + " · " + event.Actor})
	}
	sort.SliceStable(history.Events, func(i, j int) bool { return history.Events[i].At.Before(history.Events[j].At) })
	writeJSON(w, http.StatusOK, history)
}
