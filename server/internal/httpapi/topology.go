package httpapi

import (
	"encoding/json"
	"net"
	"net/http"
	"net/netip"
	"rmm-openwrt/server/internal/model"
	"sort"
	"strings"
	"time"
)

type topologyNode struct {
	ID         string              `json:"id"`
	Label      string              `json:"label"`
	DeviceID   string              `json:"device_id,omitempty"`
	Address    string              `json:"address,omitempty"`
	Kind       string              `json:"kind"`
	ObservedAt *time.Time          `json:"observed_at,omitempty"`
	Stale      bool                `json:"stale"`
	Interfaces []map[string]string `json:"interfaces,omitempty"`
}
type topologyLink struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Interface string `json:"interface"`
	Address   string `json:"address"`
	MAC       string `json:"mac,omitempty"`
	State     string `json:"state"`
	Evidence  string `json:"evidence"`
	Ambiguous bool   `json:"ambiguous"`
	Stale     bool   `json:"stale"`
}
type topologyMap struct {
	Nodes     []topologyNode `json:"nodes"`
	Links     []topologyLink `json:"links"`
	Truncated bool           `json:"truncated"`
}

func deriveTopology(devices []model.Device, now time.Time) topologyMap {
	out := topologyMap{Nodes: []topologyNode{}, Links: []topologyLink{}}
	if len(devices) > 500 {
		devices = devices[:500]
		out.Truncated = true
	}
	type reported struct {
		Interfaces []map[string]string `json:"interfaces"`
		Neighbors  []map[string]string `json:"neighbors"`
	}
	reports := map[string]reported{}
	addresses := map[string][]string{}
	for _, d := range devices {
		var report reported
		if json.Unmarshal(d.Inventory, &report) != nil {
			report = reported{}
		}
		if len(report.Interfaces) > 128 {
			report.Interfaces = report.Interfaces[:128]
			out.Truncated = true
		}
		reports[d.ID] = report
		stale := d.LastSeenAt == nil || now.Sub(*d.LastSeenAt) > 2*time.Minute
		out.Nodes = append(out.Nodes, topologyNode{ID: d.ID, Label: d.Hostname, DeviceID: d.ID, Kind: "router", ObservedAt: d.LastSeenAt, Stale: stale, Interfaces: report.Interfaces})
		seen := map[string]bool{}
		for _, iface := range report.Interfaces {
			prefix, err := netip.ParsePrefix(iface["address"])
			if err != nil {
				continue
			}
			ip := prefix.Addr().Unmap()
			if ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() {
				continue
			}
			key := ip.String()
			if !seen[key] {
				addresses[key] = append(addresses[key], d.ID)
				seen[key] = true
			}
		}
	}
	seen := map[string]bool{}
	for _, d := range devices {
		report := reports[d.ID]
		if len(report.Neighbors) > 256 {
			report.Neighbors = report.Neighbors[:256]
			out.Truncated = true
		}
		stale := d.LastSeenAt == nil || now.Sub(*d.LastSeenAt) > 2*time.Minute
		for _, neighbor := range report.Neighbors {
			ip, err := netip.ParseAddr(neighbor["ip"])
			if err != nil {
				continue
			}
			ip = ip.Unmap()
			if ip.IsUnspecified() || ip.IsMulticast() || ip.IsLoopback() {
				continue
			}
			iface := neighbor["interface"]
			if iface == "" {
				iface = neighbor["dev"]
			}
			if len(iface) > 64 {
				continue
			}
			key := d.ID + "|" + iface + "|" + ip.String()
			if seen[key] {
				continue
			}
			seen[key] = true
			targets := addresses[ip.String()]
			target := ""
			if len(targets) == 1 && targets[0] != d.ID {
				target = targets[0]
			}
			if target == "" {
				target = "neighbor:" + key
				out.Nodes = append(out.Nodes, topologyNode{ID: target, Label: ip.String(), Address: ip.String(), Kind: "neighbor", ObservedAt: d.LastSeenAt, Stale: stale})
			}
			mac := ""
			if parsed, err := net.ParseMAC(neighbor["mac"]); err == nil {
				mac = parsed.String()
			}
			state := strings.ToUpper(neighbor["state"])
			if state == "" {
				state = "UNKNOWN"
			}
			if len(state) > 32 {
				state = "UNKNOWN"
			}
			out.Links = append(out.Links, topologyLink{From: d.ID, To: target, Interface: iface, Address: ip.String(), MAC: mac, State: state, Evidence: "neighbor table; IP match is inferred, not a physical link", Ambiguous: len(targets) > 1, Stale: stale})
		}
	}
	sort.Slice(out.Links, func(i, j int) bool { return out.Links[i].From+out.Links[i].To < out.Links[j].From+out.Links[j].To })
	return out
}

func (a *App) handleTopology(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFromContext(r.Context())
	devices, err := a.store.ListDevicesForUser(r.Context(), p.User.ID, p.IsAdmin())
	if err != nil {
		writeError(w, 500, "failed to load network map")
		return
	}
	writeJSON(w, 200, deriveTopology(devices, time.Now().UTC()))
}
