package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"
)

func TestFleetReactionsConsecutiveCooldownRecoveryAndDailyCap(t *testing.T) {
	ctx := context.Background()
	s, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "rules.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	user, err := s.EnsureBootstrapUser(ctx, "rules-admin", "test-hash")
	if err != nil {
		t.Fatal(err)
	}
	device, err := s.EnrollDevice(ctx, "test-router", "24.10")
	if err != nil {
		t.Fatal(err)
	}
	rule, err := s.SaveFleetRule(ctx, user.ID, FleetRule{Title: "RAM remediation", DeviceIDs: []string{device.DeviceID}, Kind: "memory_high", Threshold: 80, Consecutive: 2, CooldownSeconds: 300, MaxActionsPerDay: 2, Action: "service_restart", Service: "dnsmasq", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	heartbeat := func(raw string) {
		t.Helper()
		if _, err = s.SaveHeartbeat(ctx, device.DeviceID, json.RawMessage(`{}`), json.RawMessage(raw)); err != nil {
			t.Fatal(err)
		}
	}
	high := `{"memory":{"total_kb":100,"used_kb":90}}`
	low := `{"memory":{"total_kb":100,"used_kb":20}}`
	advance := func() {
		t.Helper()
		if err := s.AdvanceFleetRules(ctx); err != nil {
			t.Fatal(err)
		}
	}
	events := func() []map[string]string {
		t.Helper()
		items, err := s.ListFleetRuleEvents(ctx, user.ID)
		if err != nil {
			t.Fatal(err)
		}
		return items
	}
	heartbeat(high)
	advance()
	advance()
	if len(events()) != 0 {
		t.Fatal("same sample counted twice")
	}
	heartbeat(`{}`)
	advance()
	heartbeat(high)
	advance()
	if len(events()) != 0 {
		t.Fatal("unknown observation did not reset sequence")
	}
	heartbeat(high)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.AdvanceFleetRules(ctx); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	first := events()
	if len(first) != 1 || first[0]["status"] != "queued" {
		t.Fatalf("duplicate or missing reaction: %v", first)
	}
	heartbeat(low)
	advance()
	if events()[0]["status"] != "cancelled" {
		t.Fatal("recovery did not cancel pending restart")
	}
	heartbeat(high)
	advance()
	heartbeat(high)
	advance()
	if len(events()) != 1 {
		t.Fatal("cooldown ignored")
	}
	if _, err = s.db.ExecContext(ctx, "UPDATE fleet_rule_states SET next_action_at='' WHERE rule_id=?", rule.ID); err != nil {
		t.Fatal(err)
	}
	heartbeat(high)
	advance()
	if len(events()) != 2 {
		t.Fatal("reaction not retried within cap")
	}
	heartbeat(low)
	advance()
	if _, err = s.db.ExecContext(ctx, "UPDATE fleet_rule_states SET next_action_at='' WHERE rule_id=?", rule.ID); err != nil {
		t.Fatal(err)
	}
	heartbeat(high)
	advance()
	heartbeat(high)
	advance()
	if len(events()) != 2 {
		t.Fatal("daily cap ignored")
	}
	rule.Enabled = false
	if _, err = s.SaveFleetRule(ctx, user.ID, rule); err != nil {
		t.Fatal(err)
	}
	heartbeat(high)
	advance()
	if len(events()) != 2 {
		t.Fatal("disabled rule executed")
	}
	rule.Service = "network"
	if _, err = s.SaveFleetRule(ctx, user.ID, rule); err == nil {
		t.Fatal("unsafe automatic service accepted")
	}
}

func TestReactionRejectsMissingAndInvalidTelemetry(t *testing.T) {
	rule := FleetRule{Kind: "load_high", Threshold: 2}
	for _, raw := range []string{`{}`, `{"loadavg":"NaN"}`, `{"loadavg":"-1"}`, `{"loadavg":"garbage"}`} {
		if _, known := reactionViolation(rule, json.RawMessage(raw)); known {
			t.Fatalf("invalid sample %s", raw)
		}
	}
	rule.Kind = "wan_unreachable"
	if _, known := reactionViolation(rule, json.RawMessage(`{"connectivity_checks":[{}]}`)); known {
		t.Fatal("missing reachability treated as a failure")
	}
	if failed, known := reactionViolation(rule, json.RawMessage(`{"connectivity_checks":[{"reachable":false}]}`)); !known || !failed {
		t.Fatal("failed WAN check not detected")
	}
}
