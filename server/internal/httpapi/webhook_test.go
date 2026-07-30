package httpapi

import (
	"net"
	"testing"
)

func TestValidateWebhookEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "public HTTPS", value: "https://hooks.example.com/rmm"},
		{name: "HTTP rejected", value: "http://hooks.example.com/rmm", wantErr: true},
		{name: "credentials rejected", value: "https://user:pass@hooks.example.com/rmm", wantErr: true},
		{name: "fragment rejected", value: "https://hooks.example.com/rmm#internal", wantErr: true},
		{name: "loopback rejected", value: "https://127.0.0.1/rmm", wantErr: true},
		{name: "private IPv4 rejected", value: "https://10.0.0.8/rmm", wantErr: true},
		{name: "link local IPv6 rejected", value: "https://[fe80::1]/rmm", wantErr: true},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := validateWebhookEndpoint(test.value); (got != "") != test.wantErr {
				t.Fatalf("validateWebhookEndpoint(%q) = %q, wantErr=%v", test.value, got, test.wantErr)
			}
		})
	}
}

func TestPublicWebhookIP(t *testing.T) {
	t.Parallel()

	if !publicWebhookIP(net.ParseIP("203.0.113.10")) {
		t.Fatal("documentation-range public IP should pass local-address filtering")
	}
	for _, value := range []string{"127.0.0.1", "10.0.0.1", "169.254.1.1", "::1", "fe80::1"} {
		if publicWebhookIP(net.ParseIP(value)) {
			t.Fatalf("local address %s must be rejected", value)
		}
	}
}
