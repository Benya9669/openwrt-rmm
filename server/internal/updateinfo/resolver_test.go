package updateinfo

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestResolverAcceptsOnlyVerifiedStableManifest(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	publicKeyData, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(t.TempDir(), "manifest.pem")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKeyData}), 0600); err != nil {
		t.Fatal(err)
	}

	manifest := []byte(`{"schema":1,"channel":"stable","agent":{"version":"0.6.9"}}`)
	digest := sha256.Sum256(manifest)
	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/manifest.json", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(manifest) })
	mux.HandleFunc("/manifest.sig", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(signature) })
	server := httptest.NewServer(mux)
	defer server.Close()

	resolver, err := NewResolver(server.URL+"/manifest.json", server.URL+"/manifest.sig", keyPath, "0.6.8")
	if err != nil {
		t.Fatal(err)
	}
	if err := resolver.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if resolver.Version() != "0.6.9" {
		t.Fatalf("unexpected verified version %q", resolver.Version())
	}

	manifest = []byte(`{"schema":1,"channel":"stable","agent":{"version":"9.9.9"}}`)
	if err := resolver.Refresh(context.Background()); err == nil {
		t.Fatal("tampered manifest was accepted")
	}
	if resolver.Version() != "0.6.9" {
		t.Fatalf("last verified version was lost: %q", resolver.Version())
	}
}
