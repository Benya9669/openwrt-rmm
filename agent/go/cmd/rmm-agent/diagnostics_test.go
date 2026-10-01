package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDiagnosticsReadOnlyBoundedAndExplicit(t *testing.T) {
	var calls []string
	runner := func(ctx context.Context, timeout time.Duration, name string, args ...string) (string, int) {
		if timeout > 20*time.Second {
			t.Fatal("unbounded diagnostic")
		}
		calls = append(calls, name+" "+strings.Join(args, " "))
		if name == "ping" {
			return "ICMP blocked", 1
		}
		return "test result", 0
	}
	output, code := runDiagnostics(context.Background(), map[string]string{"target": "127.0.0.1", "dns_name": "localhost"}, runner)
	if code != 1 {
		t.Fatal("unsuccessful ping was reported healthy")
	}
	var report diagnosticReport
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Checks) != 8 {
		t.Fatalf("missing diagnostic checks: %d", len(report.Checks))
	}
	if report.Checks[len(report.Checks)-1].Status != "unknown" {
		t.Fatal("NTP status was inferred from wall clock")
	}
	for _, call := range calls {
		if strings.Contains(call, "restart") || strings.Contains(call, "sh ") {
			t.Fatalf("diagnostic changed router: %s", call)
		}
	}
	calls = nil
	if _, code := runDiagnostics(context.Background(), map[string]string{"services": "../../bin/sh"}, runner); code != 2 || len(calls) != 0 {
		t.Fatal("invalid service was not rejected before execution")
	}
}
