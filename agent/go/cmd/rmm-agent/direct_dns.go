package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

var nonPublicAddressPrefixes = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"),
}

type directDNSUpdateState struct {
	nextAttempt time.Time
}

func (s directDNSUpdateState) due(now time.Time) bool {
	return s.nextAttempt.IsZero() || !now.Before(s.nextAttempt)
}

func (s *directDNSUpdateState) schedule(now time.Time, interval time.Duration, updateErr error) {
	if interval < time.Minute {
		interval = 5 * time.Minute
	}
	if updateErr != nil && interval > time.Minute {
		interval = time.Minute
	}
	s.nextAttempt = now.Add(interval)
}

func updateDirectDNS(ctx context.Context, client *http.Client, cfg config) error {
	discoveryContext, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	type result struct {
		address string
		ipv4    bool
		err     error
	}
	results := make(chan result, 2)
	requests := 0
	for _, request := range []struct {
		endpoint string
		ipv4     bool
	}{{cfg.PublicIPv4URL, true}, {cfg.PublicIPv6URL, false}} {
		if strings.TrimSpace(request.endpoint) == "" {
			continue
		}
		requests++
		go func(endpoint string, ipv4 bool) {
			address, err := fetchPublicIPAddress(discoveryContext, client, endpoint, ipv4)
			results <- result{address: address, ipv4: ipv4, err: err}
		}(request.endpoint, request.ipv4)
	}

	var ipv4, ipv6 string
	var discoveryErrors []string
	for range requests {
		result := <-results
		if result.err != nil {
			discoveryErrors = append(discoveryErrors, result.err.Error())
			continue
		}
		if result.ipv4 {
			ipv4 = result.address
		} else {
			ipv6 = result.address
		}
	}
	if ipv4 == "" && ipv6 == "" {
		if len(discoveryErrors) == 0 {
			return errors.New("no public IP discovery service is configured")
		}
		return fmt.Errorf("public IP discovery failed: %s", strings.Join(discoveryErrors, "; "))
	}

	body := map[string]string{
		"device_id": cfg.DeviceID,
		"ipv4":      ipv4,
		"ipv6":      ipv6,
	}
	return postJSON(ctx, client, cfg.ServerURL+"/api/agent/dns/update", cfg.DeviceToken, body, nil)
}

func fetchPublicIPAddress(ctx context.Context, client *http.Client, endpoint string, wantIPv4 bool) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return "", errors.New("public IP discovery URL must use HTTPS without credentials")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "text/plain")
	discoveryClient := *client
	discoveryClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	response, err := discoveryClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return "", fmt.Errorf("public IP discovery returned HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 129))
	if err != nil {
		return "", err
	}
	if len(data) > 128 {
		return "", errors.New("public IP discovery response is too large")
	}
	address, err := normalizePublicAddress(string(data), wantIPv4)
	if err != nil {
		return "", err
	}
	return address, nil
}

func normalizePublicAddress(raw string, wantIPv4 bool) (string, error) {
	address, err := netip.ParseAddr(strings.TrimSpace(raw))
	if err != nil {
		return "", errors.New("public IP discovery returned an invalid address")
	}
	address = address.Unmap()
	if address.Is4() != wantIPv4 || !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() || address.IsMulticast() || address.IsUnspecified() {
		return "", errors.New("public IP discovery returned a non-public address")
	}
	for _, prefix := range nonPublicAddressPrefixes {
		if prefix.Contains(address) {
			return "", errors.New("public IP discovery returned a reserved address")
		}
	}
	return address.String(), nil
}
