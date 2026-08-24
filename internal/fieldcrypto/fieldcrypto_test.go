// SPDX-License-Identifier: MIT
package fieldcrypto

import (
	"bytes"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	c, err := New(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := c.Encrypt("sensitive")
	if err != nil || encrypted == "sensitive" || !IsEncrypted(encrypted) {
		t.Fatalf("unexpected encryption result %q: %v", encrypted, err)
	}
	plain, err := c.Decrypt(encrypted)
	if err != nil || plain != "sensitive" {
		t.Fatalf("unexpected plaintext %q: %v", plain, err)
	}
}

func TestDecryptLeavesLegacyPlaintextReadable(t *testing.T) {
	c, err := New(bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := c.Decrypt("legacy")
	if err != nil || plain != "legacy" {
		t.Fatalf("legacy value was not preserved: %q %v", plain, err)
	}
}

func TestCiphertextIsBoundToContext(t *testing.T) {
	c, _ := New(bytes.Repeat([]byte{4}, 32))
	encrypted, err := c.EncryptWithContext("user-1:webhook", "sensitive")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.DecryptWithContext("user-2:webhook", encrypted); err == nil {
		t.Fatal("ciphertext was accepted in a different context")
	}
}
