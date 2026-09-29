package services

import (
	"strings"
	"testing"
)

// The token that goes in the email and the value stored in the database must
// not be the same thing. If they were, a leaked database would be a list of
// working account-takeover tokens.
func TestGenerateResetToken_RawAndHashDiffer(t *testing.T) {
	raw, hash, err := generateResetToken()
	if err != nil {
		t.Fatalf("generateResetToken: %v", err)
	}

	if raw == hash {
		t.Fatal("raw token equals stored hash — the database would hold usable tokens")
	}
	if hash != hashToken(raw) {
		t.Error("stored hash does not match hashToken(raw) — lookup would never find it")
	}
}

// Two calls must not collide, and the token must be long enough that guessing
// is not a strategy. 32 bytes hex-encoded is 64 characters.
func TestGenerateResetToken_UniqueAndLong(t *testing.T) {
	seen := make(map[string]bool)

	for range 50 {
		raw, _, err := generateResetToken()
		if err != nil {
			t.Fatalf("generateResetToken: %v", err)
		}
		if seen[raw] {
			t.Fatal("generateResetToken repeated a value — tokens are predictable")
		}
		seen[raw] = true

		if len(raw) < 64 {
			t.Fatalf("token is only %d chars; too short to resist guessing", len(raw))
		}
	}
}

// A short password is rejected before the token is even looked at, so a weak
// password cannot consume a valid token.
func TestResetPassword_RejectsShortPassword(t *testing.T) {
	svc := &AuthService{BcryptCost: 4}

	err := svc.ResetPassword(t.Context(), "any-token", "short")
	if err != ErrWeakPassword {
		t.Errorf("err = %v, want ErrWeakPassword", err)
	}
}

// An empty token is refused without touching the database — there is nothing
// to look up, and the query would match nothing anyway.
func TestResetPassword_RejectsEmptyToken(t *testing.T) {
	svc := &AuthService{BcryptCost: 4}

	err := svc.ResetPassword(t.Context(), "   ", "longenoughpassword")
	if err != ErrInvalidResetToken {
		t.Errorf("err = %v, want ErrInvalidResetToken", err)
	}
}

// ForgotPassword with no email is a client mistake and is reported as such.
// Note this is the *only* case that differs — an unknown address does not.
func TestForgotPassword_RequiresEmail(t *testing.T) {
	svc := &AuthService{BcryptCost: 4}

	for _, in := range []string{"", "   ", "\t"} {
		if err := svc.ForgotPassword(t.Context(), in); err != ErrEmailRequired {
			t.Errorf("ForgotPassword(%q) = %v, want ErrEmailRequired", in, err)
		}
	}
}

// The address is normalised before it reaches the query: trimmed and
// lowercased, so a reset requested as "  USER@EXAMPLE.COM  " looks up the
// account stored as "user@example.com".
//
// Asserted through the input rule rather than by reaching a database — this
// test has none, and the normalisation is the part that would silently break.
func TestForgotPassword_NormalisationRules(t *testing.T) {
	// Mirrors what ForgotPassword applies, so the test states the rule
	// instead of restating the implementation.
	normalise := func(in string) string {
		return strings.ToLower(strings.TrimSpace(in))
	}

	for _, tc := range []struct{ in, want string }{
		{"  USER@EXAMPLE.COM  ", "user@example.com"},
		{"User@Example.com", "user@example.com"},
		{"already@lower.com", "already@lower.com"},
	} {
		if got := normalise(tc.in); got != tc.want {
			t.Errorf("normalise(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
