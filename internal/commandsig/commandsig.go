// SPDX-License-Identifier: MIT
package commandsig

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const payloadVersion = "rmm-command-v1"

type Envelope struct {
	ID        string
	DeviceID  string
	Type      string
	Args      json.RawMessage
	CreatedAt time.Time
	ExpiresAt time.Time
	Nonce     string
}

type canonicalEnvelope struct {
	Version   string          `json:"version"`
	ID        string          `json:"id"`
	DeviceID  string          `json:"device_id"`
	Type      string          `json:"type"`
	Args      json.RawMessage `json:"args"`
	CreatedAt string          `json:"created_at"`
	ExpiresAt string          `json:"expires_at"`
	Nonce     string          `json:"nonce"`
}

func Canonical(envelope Envelope) ([]byte, error) {
	if strings.TrimSpace(envelope.ID) == "" || strings.TrimSpace(envelope.DeviceID) == "" || strings.TrimSpace(envelope.Type) == "" || strings.TrimSpace(envelope.Nonce) == "" {
		return nil, errors.New("command envelope is incomplete")
	}
	if envelope.CreatedAt.IsZero() || envelope.ExpiresAt.IsZero() || !envelope.ExpiresAt.After(envelope.CreatedAt) {
		return nil, errors.New("command envelope time range is invalid")
	}
	args := envelope.Args
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	decoder := json.NewDecoder(bytes.NewReader(args))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, fmt.Errorf("invalid command args: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, err
	}
	compact, err := json.Marshal(decoded)
	if err != nil {
		return nil, fmt.Errorf("canonicalize command args: %w", err)
	}
	return json.Marshal(canonicalEnvelope{
		Version:   payloadVersion,
		ID:        strings.TrimSpace(envelope.ID),
		DeviceID:  strings.TrimSpace(envelope.DeviceID),
		Type:      strings.TrimSpace(envelope.Type),
		Args:      compact,
		CreatedAt: envelope.CreatedAt.UTC().Format(time.RFC3339Nano),
		ExpiresAt: envelope.ExpiresAt.UTC().Format(time.RFC3339Nano),
		Nonce:     strings.TrimSpace(envelope.Nonce),
	})
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return fmt.Errorf("invalid trailing command args: %w", err)
	}
	return errors.New("command args contain multiple JSON values")
}

func Sign(privateKey ed25519.PrivateKey, envelope Envelope) (string, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return "", errors.New("invalid Ed25519 private key")
	}
	payload, err := Canonical(envelope)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, payload)), nil
}

func Verify(publicKey ed25519.PublicKey, envelope Envelope, signature string) error {
	if len(publicKey) != ed25519.PublicKeySize {
		return errors.New("invalid Ed25519 public key")
	}
	payload, err := Canonical(envelope)
	if err != nil {
		return err
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(signature))
	if err != nil || len(raw) != ed25519.SignatureSize || !ed25519.Verify(publicKey, payload, raw) {
		return errors.New("command signature is invalid")
	}
	return nil
}

func KeyID(publicKey ed25519.PublicKey) string {
	digest := sha256.Sum256(publicKey)
	return hex.EncodeToString(digest[:8])
}

func EncodePublicKey(publicKey ed25519.PublicKey) string {
	return "ed25519:" + base64.RawStdEncoding.EncodeToString(publicKey)
}

func ParsePublicKey(value string) (ed25519.PublicKey, error) {
	rawValue, ok := strings.CutPrefix(strings.TrimSpace(value), "ed25519:")
	if !ok {
		return nil, errors.New("command public key must use the ed25519 prefix")
	}
	raw, err := base64.RawStdEncoding.DecodeString(rawValue)
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return nil, errors.New("command public key is invalid")
	}
	return ed25519.PublicKey(raw), nil
}

func NewNonce() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func LoadOrCreatePrivateKey(path string) (ed25519.PrivateKey, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("command signing key path is empty")
	}
	if data, err := os.ReadFile(path); err == nil {
		return parsePrivateKey(data)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, err
	}
	data := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	temporary := path + ".new"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return nil, err
	}
	if err := os.Chmod(temporary, 0o600); err != nil {
		_ = os.Remove(temporary)
		return nil, err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return nil, err
	}
	return privateKey, nil
}

func parsePrivateKey(data []byte) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, errors.New("command signing key is not PKCS#8 PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	privateKey, ok := parsed.(ed25519.PrivateKey)
	if !ok || len(privateKey) != ed25519.PrivateKeySize {
		return nil, errors.New("command signing key is not Ed25519")
	}
	return privateKey, nil
}
