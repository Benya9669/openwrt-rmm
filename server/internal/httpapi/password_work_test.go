package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type resetLookupStore struct {
	Store
	valid bool
}

func (s resetLookupStore) PasswordResetValid(context.Context, string) (bool, error) {
	return s.valid, nil
}

func TestPasswordResetRejectsInvalidLinkBeforeReservingPasswordWork(t *testing.T) {
	app := App{loginSlots: make(chan struct{}, 1)}
	app.loginSlots <- struct{}{}
	for _, test := range []struct {
		valid  bool
		status int
	}{{false, http.StatusBadRequest}, {true, http.StatusTooManyRequests}} {
		app.store = resetLookupStore{valid: test.valid}
		body := `{"token":"fixture-token-of-sufficient-length-for-lookup","new_password":"synthetic-password"}`
		r := httptest.NewRequest(http.MethodPost, "/api/auth/password-reset/confirm", strings.NewReader(body))
		w := httptest.NewRecorder()
		app.handlePasswordResetConfirm(w, r)
		if w.Code != test.status {
			t.Fatalf("valid=%v: got %d, want %d", test.valid, w.Code, test.status)
		}
	}
}
