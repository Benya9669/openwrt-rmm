package main

import "testing"

func TestDiagnosticServiceDoesNotTreatUnknownAsStopped(t *testing.T) {
	for _, test := range []struct{ json, status string }{
		{`{}`, "warning"},
		{`{"dnsmasq":{"instances":{}}}`, "warning"},
		{`{"dnsmasq":{"instances":{"main":{}}}}`, "warning"},
		{`{"dnsmasq":{"instances":{"main":{"running":false}}}}`, "failed"},
		{`{"dnsmasq":{"instances":{"main":{"running":true}}}}`, "passed"},
	} {
		status, _ := diagnosticServiceStatus(test.json, 0, "dnsmasq")
		if status != test.status {
			t.Fatalf("%s: %s", test.json, status)
		}
	}
}
