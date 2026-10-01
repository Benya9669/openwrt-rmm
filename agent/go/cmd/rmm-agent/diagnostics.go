package main

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"time"
)

type diagnosticCheck struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Summary    string `json:"summary"`
	Output     string `json:"output,omitempty"`
	DurationMS int64  `json:"duration_ms"`
}

type diagnosticReport struct {
	StartedAt  string            `json:"started_at"`
	FinishedAt string            `json:"finished_at"`
	Target     string            `json:"target"`
	Checks     []diagnosticCheck `json:"checks"`
}

type diagnosticRunner func(context.Context, time.Duration, string, ...string) (string, int)

func runDiagnostics(ctx context.Context, args map[string]string, run diagnosticRunner) (string, int) {
	target := commandTarget(args, "1.1.1.1")
	domain := valueDefault(args["dns_name"], "openwrt.org")
	if !safeHostName(target) || !safeHostName(domain) {
		return "diagnostic target is invalid\n", 2
	}
	services := []string{"dnsmasq", "uhttpd"}
	if value := strings.TrimSpace(args["services"]); value != "" {
		services = strings.Split(value, ",")
		if len(services) > 5 {
			return "too many diagnostic services\n", 2
		}
	}
	for _, service := range services {
		if !safeServiceName(strings.TrimSpace(service)) {
			return "diagnostic service is not allowlisted\n", 2
		}
	}
	report := diagnosticReport{StartedAt: time.Now().UTC().Format(time.RFC3339), Target: target, Checks: []diagnosticCheck{}}
	start := time.Now()
	dnsCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	addresses, err := net.DefaultResolver.LookupHost(dnsCtx, domain)
	cancel()
	dns := diagnosticCheck{Name: "dns", Status: "passed", Summary: "DNS name resolved", DurationMS: time.Since(start).Milliseconds()}
	if err != nil {
		dns.Status = "failed"
		dns.Summary = "DNS name could not be resolved"
	} else {
		dns.Output = strings.Join(addresses, "\n")
	}
	report.Checks = append(report.Checks, dns)
	checks := []struct {
		name, command, summary string
		timeout                time.Duration
		args                   []string
	}{
		{"ping", "ping", "ICMP reachability", 12 * time.Second, []string{"-c", "3", "-W", "2", target}},
		{"route", "ip", "Routing table", 5 * time.Second, []string{"route", "show"}},
		{"interfaces", "ip", "Interface addresses", 5 * time.Second, []string{"-o", "addr", "show"}},
		{"traceroute", "traceroute", "Network path", 20 * time.Second, []string{"-m", "8", "-w", "1", target}},
	}
	for _, check := range checks {
		start := time.Now()
		output, code := run(ctx, check.timeout, check.command, check.args...)
		status := "passed"
		summary := check.summary + " checked"
		if code != 0 {
			status = "failed"
			summary = check.summary + " unavailable or unsuccessful"
		}
		if len(output) > 16384 {
			output = output[:16384] + "\n[output truncated]"
		}
		report.Checks = append(report.Checks, diagnosticCheck{Name: check.name, Status: status, Summary: summary, Output: output, DurationMS: time.Since(start).Milliseconds()})
	}
	for _, service := range services {
		service = strings.TrimSpace(service)
		if !safeServiceName(service) {
			return "diagnostic service is not allowlisted\n", 2
		}
		start := time.Now()
		query, _ := json.Marshal(map[string]string{"name": service})
		output, code := run(ctx, 5*time.Second, "ubus", "call", "service", "list", string(query))
		status, summary := diagnosticServiceStatus(output, code, service)
		// procd responses may include process arguments and environment credentials.
		report.Checks = append(report.Checks, diagnosticCheck{Name: "service:" + service, Status: status, Summary: summary, DurationMS: time.Since(start).Milliseconds()})
	}
	report.Checks = append(report.Checks, diagnosticCheck{Name: "time", Status: "unknown", Summary: "Router UTC time; NTP synchronization is not inferred", Output: time.Now().UTC().Format(time.RFC3339) + "\nuptime: " + strings.TrimSpace(readFileString("/proc/uptime"))})
	report.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.Marshal(report)
	if err != nil {
		return "diagnostic report encoding failed\n", 1
	}
	code := 0
	for _, check := range report.Checks {
		if check.Status == "failed" {
			code = 1
		}
	}
	return string(data), code
}

func diagnosticServiceStatus(output string, code int, service string) (string, string) {
	var services map[string]struct {
		Instances map[string]struct {
			Running *bool `json:"running"`
		} `json:"instances"`
	}
	if code != 0 || json.Unmarshal([]byte(output), &services) != nil || len(services[service].Instances) == 0 {
		return "warning", "Service state unavailable"
	}
	unknown := false
	for _, instance := range services[service].Instances {
		if instance.Running == nil {
			unknown = true
		} else if *instance.Running {
			return "passed", "Service reports running"
		}
	}
	if unknown {
		return "warning", "Service state unavailable"
	}
	return "failed", "Service reports no running instances"
}
