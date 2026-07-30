package updateinfo

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

const maxManifestSize = 1 << 20

var stableVersionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)

type Manifest struct {
	Schema  int    `json:"schema"`
	Channel string `json:"channel"`
	Agent   struct {
		Version string `json:"version"`
	} `json:"agent"`
}

type Resolver struct {
	manifestURL  string
	signatureURL string
	publicKey    *ecdsa.PublicKey
	client       *http.Client

	mu      sync.RWMutex
	version string
}

func NewResolver(manifestURL, signatureURL, publicKeyPath, fallback string) (*Resolver, error) {
	publicKeyData, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read update manifest public key: %w", err)
	}
	block, _ := pem.Decode(publicKeyData)
	if block == nil {
		return nil, errors.New("decode update manifest public key: PEM block is missing")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse update manifest public key: %w", err)
	}
	publicKey, ok := parsed.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("update manifest public key is not ECDSA")
	}
	manifestURL = strings.TrimSpace(manifestURL)
	signatureURL = strings.TrimSpace(signatureURL)
	if manifestURL == "" || signatureURL == "" {
		return nil, errors.New("update manifest and signature URLs are required")
	}
	return &Resolver{
		manifestURL: manifestURL, signatureURL: signatureURL, publicKey: publicKey,
		version: fallback,
		client:  &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func (r *Resolver) Version() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.version
}

func (r *Resolver) Refresh(ctx context.Context) error {
	manifestData, err := r.fetch(ctx, r.manifestURL)
	if err != nil {
		return fmt.Errorf("download update manifest: %w", err)
	}
	signature, err := r.fetch(ctx, r.signatureURL)
	if err != nil {
		return fmt.Errorf("download update manifest signature: %w", err)
	}
	digest := sha256.Sum256(manifestData)
	if !ecdsa.VerifyASN1(r.publicKey, digest[:], signature) {
		return errors.New("update manifest signature is invalid")
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return fmt.Errorf("decode update manifest: %w", err)
	}
	version := strings.TrimSpace(manifest.Agent.Version)
	if manifest.Schema != 1 || manifest.Channel != "stable" || !stableVersionPattern.MatchString(version) {
		return errors.New("update manifest schema, channel, or agent version is invalid")
	}
	r.mu.Lock()
	r.version = version
	r.mu.Unlock()
	return nil
}

func (r *Resolver) Run(ctx context.Context, interval time.Duration, report func(error)) {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.Refresh(ctx); err != nil && report != nil {
				report(err)
			}
		}
	}
}

func (r *Resolver) fetch(ctx context.Context, endpoint string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	response, err := r.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP status %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxManifestSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || len(data) > maxManifestSize {
		return nil, errors.New("response is empty or exceeds the size limit")
	}
	return data, nil
}
