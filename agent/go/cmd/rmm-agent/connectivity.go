package main

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	mozillaProbeURL = "https://detectportal.firefox.com/success.txt"
	appleProbeURL   = "https://www.apple.com/library/test/success.html"
)

var connectivityHTTPClient = newConnectivityHTTPClient()

func newConnectivityHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &http.Client{
		Transport:     transport,
		Timeout:       4 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

func httpsProbeURLs(value string) []string {
	var targets []string
	seen := make(map[string]bool)
	for _, raw := range strings.Fields(value) {
		u, err := url.Parse(raw)
		if err != nil || len(raw) > 2048 || u.Scheme != "https" || u.Hostname() == "" ||
			u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || seen[u.String()] {
			continue
		}
		seen[u.String()] = true
		targets = append(targets, u.String())
		if len(targets) == 4 {
			break
		}
	}
	if len(targets) == 0 {
		return []string{mozillaProbeURL, appleProbeURL}
	}
	return targets
}

func probeHTTPSConnectivity(ctx context.Context, client *http.Client, targets []string) []map[string]any {
	checks := make([]map[string]any, len(targets))
	var wg sync.WaitGroup
	for i, target := range targets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			checks[i] = probeHTTPS(ctx, client, target)
		}()
	}
	wg.Wait()
	return checks
}

func probeHTTPS(ctx context.Context, client *http.Client, target string) map[string]any {
	result := map[string]any{"reachable": false, "method": "https"}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return result
	}
	result["target"] = req.URL.Host
	req.Header.Set("User-Agent", "OpenWrt-RMM connectivity probe")
	req.Header.Set("Cache-Control", "no-cache")
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return result
	}
	defer resp.Body.Close()
	result["http_status"] = resp.StatusCode
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result
	}

	var expected string
	switch target {
	case mozillaProbeURL:
		expected = "success"
	case appleProbeURL:
		expected = "<HTML><HEAD><TITLE>Success</TITLE></HEAD><BODY>Success</BODY></HTML>"
	}
	if expected != "" {
		body, err := io.ReadAll(io.LimitReader(resp.Body, 256))
		if err != nil || resp.StatusCode != http.StatusOK || strings.TrimSpace(string(body)) != expected {
			return result
		}
	}
	result["reachable"] = true
	result["response_time_ms"] = float64(time.Since(start).Microseconds()) / 1000
	return result
}
