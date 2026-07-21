package httpapi

import (
	"errors"
	"net/http"
	"net/netip"
	"strconv"
	"strings"

	"rmm-openwrt/server/internal/model"
)

var nonPublicDNSPrefixes = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"),
}

type dnsRecordUpdateRequest struct {
	DNSLabel *string `json:"dns_label"`
	TTL      *int    `json:"ttl"`
	Enabled  *bool   `json:"enabled"`
}

type agentDNSUpdateRequest struct {
	DeviceID string `json:"device_id"`
	IPv4     string `json:"ipv4"`
	IPv6     string `json:"ipv6"`
}

func (a *App) handleListDNSRecords(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFromContext(r.Context())
	records, err := a.store.ListDNSRecordsForUser(r.Context(), principal.User.ID, principal.IsAdmin())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list DNS records")
		return
	}
	a.decorateDNSRecords(records)
	writeJSON(w, http.StatusOK, map[string]any{"records": records})
}

func (a *App) handleGetDeviceDNS(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 4 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	record, found, err := a.store.GetDNSRecord(r.Context(), parts[2])
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load DNS record")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "DNS record not found")
		return
	}
	a.decorateDNSRecord(&record)
	writeJSON(w, http.StatusOK, record)
}

func (a *App) handleUpdateDeviceDNS(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 4 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	var req dnsRecordUpdateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.DNSLabel == nil && req.TTL == nil && req.Enabled == nil {
		writeError(w, http.StatusBadRequest, "dns_label, ttl or enabled is required")
		return
	}
	if req.DNSLabel != nil {
		label := strings.ToLower(strings.TrimSpace(*req.DNSLabel))
		if !dnsLabelPattern.MatchString(label) {
			writeError(w, http.StatusBadRequest, "dns_label must be a valid single DNS label")
			return
		}
		req.DNSLabel = &label
	}
	if req.TTL != nil && (*req.TTL < 30 || *req.TTL > 86400) {
		writeError(w, http.StatusBadRequest, "ttl must be between 30 and 86400 seconds")
		return
	}
	record, found, err := a.store.UpdateDNSRecordSettings(r.Context(), parts[2], req.DNSLabel, req.TTL, req.Enabled)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") || strings.Contains(strings.ToLower(err.Error()), "reserved") {
			writeError(w, http.StatusConflict, "dns label is already in use")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update DNS record")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	a.decorateDNSRecord(&record)
	principal, _ := principalFromContext(r.Context())
	_, _ = a.store.AddAuditEvent(r.Context(), principal.User.Username, "device.dns_update", record.DeviceID, "", mustJSON(map[string]any{
		"dns_label":  req.DNSLabel,
		"ttl":        req.TTL,
		"enabled":    req.Enabled,
		"request_id": requestID(r.Context()),
	}))
	a.events.publish("devices")
	writeJSON(w, http.StatusOK, record)
}

func (a *App) handleListDeviceDNSHistory(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 5 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	history, found, err := a.store.ListDNSAddressHistory(r.Context(), parts[2], limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load DNS address history")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"history": history})
}

func (a *App) handleAgentDNSUpdate(w http.ResponseWriter, r *http.Request) {
	var req agentDNSUpdateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := a.authorizeDevice(r, req.DeviceID); err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	limitKey := strings.TrimSpace(req.DeviceID)
	if !a.dnsUpdateLimiter.Allow(limitKey) {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "DNS update rate limit exceeded")
		return
	}
	a.dnsUpdateLimiter.Fail(limitKey)
	ipv4, err := normalizePublicDNSAddress(req.IPv4, true)
	if err != nil {
		writeError(w, http.StatusBadRequest, "ipv4 must be a public IPv4 address")
		return
	}
	ipv6, err := normalizePublicDNSAddress(req.IPv6, false)
	if err != nil {
		writeError(w, http.StatusBadRequest, "ipv6 must be a public IPv6 address")
		return
	}
	if ipv4 == "" && ipv6 == "" {
		writeError(w, http.StatusBadRequest, "at least one public IP address is required")
		return
	}
	record, found, changed, err := a.store.UpdateDNSAddresses(r.Context(), req.DeviceID, ipv4, ipv6, "agent")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update DNS addresses")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	a.decorateDNSRecord(&record)
	if changed {
		_, _ = a.store.AddAuditEvent(r.Context(), "agent", "agent.dns_update", record.DeviceID, "", mustJSON(map[string]any{
			"has_ipv4":   ipv4 != "",
			"has_ipv6":   ipv6 != "",
			"request_id": requestID(r.Context()),
		}))
		a.events.publish("devices")
	}
	writeJSON(w, http.StatusOK, map[string]any{"record": record, "changed": changed})
}

func (a *App) handleInternalDNSRecords(w http.ResponseWriter, r *http.Request) {
	if a.dnsSyncToken == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	token, ok := bearerToken(r)
	if !ok || !constantTimeEqual(token, a.dnsSyncToken) {
		writeError(w, http.StatusUnauthorized, "invalid DNS sync credentials")
		return
	}
	records, err := a.store.ListPublishableDNSRecords(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list publishable DNS records")
		return
	}
	a.decorateDNSRecords(records)
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"records": records})
}

func (a *App) decorateDNSRecords(records []model.DNSRecord) {
	for index := range records {
		a.decorateDNSRecord(&records[index])
	}
}

func (a *App) decorateDNSRecord(record *model.DNSRecord) {
	if a.deviceDomain != "" && record.DNSLabel != "" {
		record.DomainName = record.DNSLabel + "." + a.deviceDomain
	}
}

func normalizePublicDNSAddress(raw string, wantIPv4 bool) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	address, err := netip.ParseAddr(raw)
	if err != nil {
		return "", err
	}
	address = address.Unmap()
	if address.Is4() != wantIPv4 || !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() || address.IsMulticast() || address.IsUnspecified() {
		return "", errors.New("address is not publicly routable")
	}
	for _, prefix := range nonPublicDNSPrefixes {
		if prefix.Contains(address) {
			return "", errors.New("address is reserved")
		}
	}
	return address.String(), nil
}
