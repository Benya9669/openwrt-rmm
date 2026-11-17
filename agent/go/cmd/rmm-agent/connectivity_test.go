package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

func TestHTTPSProbeRejectsRedirectsAndUntrustedTLS(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			hits.Add(1)
			w.WriteHeader(http.StatusNoContent)
		case "/redirect":
			http.Redirect(w, r, "/ok", http.StatusFound)
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer server.Close()

	ctx := context.Background()
	client := newConnectivityHTTPClient()
	if result := probeHTTPS(ctx, client, server.URL+"/ok"); result["reachable"] != false {
		t.Fatalf("untrusted certificate accepted: %v", result)
	}
	client.Transport = server.Client().Transport
	results := probeHTTPSConnectivity(ctx, client, []string{server.URL + "/ok", server.URL + "/redirect", server.URL + "/unavailable"})
	if results[0]["reachable"] != true || results[0]["http_status"] != 204 || results[0]["response_time_ms"] == nil {
		t.Fatalf("valid HTTPS response rejected: %v", results[0])
	}
	if results[1]["reachable"] != false || results[1]["http_status"] != 302 || hits.Load() != 1 {
		t.Fatalf("redirect followed or accepted: %v", results[1])
	}
	if results[2]["reachable"] != false || results[2]["http_status"] != 503 || results[2]["response_time_ms"] != nil {
		t.Fatalf("failed probe reported as successful: %v", results[2])
	}
}

func TestHTTPSProbeURLsIgnoreLegacyAndInvalidTargets(t *testing.T) {
	if got := httpsProbeURLs("1.1.1.1 8.8.8.8 http://unsafe.test https://user:pass@unsafe.test"); !reflect.DeepEqual(got, []string{mozillaProbeURL, appleProbeURL}) {
		t.Fatalf("expected safe defaults, got %v", got)
	}
	if got := httpsProbeURLs("https://example.test/health https://example.test/health https://example.org/ready https://other.test/?token=test"); !reflect.DeepEqual(got, []string{"https://example.test/health", "https://example.org/ready"}) {
		t.Fatalf("expected distinct valid targets, got %v", got)
	}
}

func TestConnectivityFailureCountsProbeCyclesOnly(t *testing.T) {
	var sampler connectivitySampler
	now := time.Unix(1000, 0)
	targets := []string{mozillaProbeURL, appleProbeURL}
	recovered := false
	probe := func([]string) []map[string]any {
		return []map[string]any{{"reachable": false}, {"reachable": recovered}}
	}
	for _, step := range []struct{ elapsed, want int }{{0, 1}, {30, 1}, {300, 2}, {330, 2}} {
		sampler.current(now.Add(time.Duration(step.elapsed)*time.Second), 5*time.Minute, targets, probe)
		if sampler.failedCycles != step.want {
			t.Fatalf("elapsed %d: got %d failed cycles, want %d", step.elapsed, sampler.failedCycles, step.want)
		}
	}
	recovered = true
	sampler.current(now.Add(10*time.Minute), 5*time.Minute, targets, probe)
	if sampler.failedCycles != 0 {
		t.Fatal("one successful target must clear failed cycles")
	}
}
