// SPDX-License-Identifier: MIT
package commandsig

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestCommandSignatureBindsEnvelope(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	envelope := Envelope{ID: "cmd_1", DeviceID: "dev_1", Type: "ping", Args: json.RawMessage(`{"target":"1.1.1.1"}`), CreatedAt: now, ExpiresAt: now.Add(time.Minute), Nonce: "nonce"}
	signature, err := Sign(privateKey, envelope)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(publicKey, envelope, signature); err != nil {
		t.Fatal(err)
	}
	wireEquivalent := envelope
	wireEquivalent.Args = json.RawMessage(`{ "target": "1.1.1.1" }`)
	if err := Verify(publicKey, wireEquivalent, signature); err != nil {
		t.Fatalf("equivalent JSON wire representation changed the signature: %v", err)
	}
	envelope.DeviceID = "dev_2"
	if err := Verify(publicKey, envelope, signature); err == nil {
		t.Fatal("signature accepted a different device")
	}
}

func TestCommandSignatureNormalizesHTMLEscaping(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now().UTC().Truncate(time.Second)
	envelope := Envelope{ID: "cmd_html", DeviceID: "dev_1", Type: "uci_set", Args: json.RawMessage(`{"value":"<tag>"}`), CreatedAt: now, ExpiresAt: now.Add(time.Minute), Nonce: "nonce"}
	signature, err := Sign(privateKey, envelope)
	if err != nil {
		t.Fatal(err)
	}
	envelope.Args = json.RawMessage(`{"value":"\u003ctag\u003e"}`)
	if err := Verify(publicKey, envelope, signature); err != nil {
		t.Fatalf("HTML-escaped JSON changed the signature: %v", err)
	}
}

func TestLoadOrCreatePrivateKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "command-signing.pem")
	first, err := LoadOrCreatePrivateKey(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadOrCreatePrivateKey(path)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Equal(second) {
		t.Fatal("persisted command signing key changed")
	}
}
