// SPDX-License-Identifier: MIT
package fieldcrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const prefix = "enc:v1:"

type Cipher struct {
	aead  cipher.AEAD
	keyID string
}

func New(key []byte) (*Cipher, error) {
	if len(key) != 32 {
		return nil, errors.New("data encryption key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(key)
	return &Cipher{aead: aead, keyID: hex.EncodeToString(digest[:8])}, nil
}

func (c *Cipher) KeyID() string {
	if c == nil {
		return ""
	}
	return c.keyID
}

func LoadOrCreate(path string) (*Cipher, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("data encryption key path is empty")
	}
	if data, err := os.ReadFile(path); err == nil {
		raw, decodeErr := base64.RawStdEncoding.DecodeString(strings.TrimSpace(string(data)))
		if decodeErr != nil {
			return nil, fmt.Errorf("decode data encryption key: %w", decodeErr)
		}
		return New(raw)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	raw := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, raw); err != nil {
		return nil, err
	}
	temporary := path + ".new"
	if err := os.WriteFile(temporary, []byte(base64.RawStdEncoding.EncodeToString(raw)+"\n"), 0o600); err != nil {
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
	return New(raw)
}

func IsEncrypted(value string) bool {
	return strings.HasPrefix(strings.TrimSpace(value), prefix)
}

func (c *Cipher) Encrypt(value string) (string, error) {
	return c.EncryptWithContext("", value)
}

func (c *Cipher) EncryptWithContext(context, value string) (string, error) {
	if value == "" {
		return value, nil
	}
	if IsEncrypted(value) {
		if _, err := c.DecryptWithContext(context, value); err != nil {
			return "", err
		}
		return value, nil
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := c.aead.Seal(nonce, nonce, []byte(value), []byte(context))
	return prefix + base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (c *Cipher) EncryptBytes(value []byte) ([]byte, error) {
	return c.EncryptBytesWithContext("", value)
}

func (c *Cipher) EncryptBytesWithContext(context string, value []byte) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return c.aead.Seal(nonce, nonce, value, []byte(context)), nil
}

func (c *Cipher) DecryptBytes(value []byte) ([]byte, error) {
	return c.DecryptBytesWithContext("", value)
}

func (c *Cipher) DecryptBytesWithContext(context string, value []byte) ([]byte, error) {
	if len(value) < c.aead.NonceSize() {
		return nil, errors.New("encrypted data is malformed")
	}
	nonce, ciphertext := value[:c.aead.NonceSize()], value[c.aead.NonceSize():]
	plain, err := c.aead.Open(nil, nonce, ciphertext, []byte(context))
	if err != nil {
		return nil, errors.New("encrypted data authentication failed")
	}
	return plain, nil
}

func (c *Cipher) Decrypt(value string) (string, error) {
	return c.DecryptWithContext("", value)
}

func (c *Cipher) DecryptWithContext(context, value string) (string, error) {
	if value == "" || !IsEncrypted(value) {
		return value, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(strings.TrimSpace(value), prefix))
	if err != nil || len(raw) < c.aead.NonceSize() {
		return "", errors.New("encrypted field is malformed")
	}
	nonce, ciphertext := raw[:c.aead.NonceSize()], raw[c.aead.NonceSize():]
	plain, err := c.aead.Open(nil, nonce, ciphertext, []byte(context))
	if err != nil {
		return "", errors.New("encrypted field authentication failed")
	}
	return string(plain), nil
}
