package authn

import (
	"strings"
	"testing"
)

func TestPasswordHashAndVerify(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("unexpected password hash format: %q", hash)
	}
	if !VerifyPassword(hash, "correct-horse-battery-staple") {
		t.Fatal("correct password was rejected")
	}
	if VerifyPassword(hash, "wrong-password") {
		t.Fatal("wrong password was accepted")
	}
	if VerifyPassword("$argon2id$v=19$m=999999999,t=2,p=1$c2FsdA$aGFzaA", "anything") {
		t.Fatal("unsafe Argon2 parameters were accepted")
	}
}

func TestPasswordLengthLimits(t *testing.T) {
	if _, err := HashPassword("too-short"); err == nil {
		t.Fatal("short password was accepted")
	}
	if _, err := HashPassword(strings.Repeat("x", 1025)); err == nil {
		t.Fatal("oversized password was accepted")
	}
}
