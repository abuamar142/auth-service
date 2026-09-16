package services

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestHashToken_Deterministic(t *testing.T) {
	input := "test-token-123"
	h1 := hashToken(input)
	h2 := hashToken(input)
	if h1 != h2 {
		t.Errorf("hashToken not deterministic: %s != %s", h1, h2)
	}
}

func TestHashToken_IsHex(t *testing.T) {
	h := hashToken("anything")
	if _, err := hex.DecodeString(h); err != nil {
		t.Errorf("hashToken output is not valid hex: %s", h)
	}
}

func TestHashToken_DifferentInputs(t *testing.T) {
	h1 := hashToken("token-a")
	h2 := hashToken("token-b")
	if h1 == h2 {
		t.Error("hashToken produced same hash for different inputs")
	}
}

func TestRandomBytes_Length(t *testing.T) {
	b, err := randomBytes(32)
	if err != nil {
		t.Fatalf("randomBytes failed: %v", err)
	}
	if len(b) != 32 {
		t.Errorf("expected 32 bytes, got %d", len(b))
	}
}

func TestRandomBytes_Unique(t *testing.T) {
	b1, _ := randomBytes(32)
	b2, _ := randomBytes(32)
	if hex.EncodeToString(b1) == hex.EncodeToString(b2) {
		t.Error("randomBytes produced identical output twice")
	}
}

func TestRandomBytes_DifferentSizes(t *testing.T) {
	sizes := []int{16, 32, 64}
	for _, size := range sizes {
		b, err := randomBytes(size)
		if err != nil {
			t.Fatalf("randomBytes(%d) failed: %v", size, err)
		}
		if len(b) != size {
			t.Errorf("randomBytes(%d) returned %d bytes", size, len(b))
		}
	}
}

func TestAPIKeyPrefix(t *testing.T) {
	// Simulate what CreateAPIKey does for prefix
	raw, _ := randomBytes(32)
	keyStr := "ak_" + hex.EncodeToString(raw)
	prefix := keyStr[:12]

	if !strings.HasPrefix(prefix, "ak_") {
		t.Errorf("prefix should start with 'ak_', got %s", prefix)
	}
	if len(prefix) != 12 {
		t.Errorf("prefix should be 12 chars, got %d", len(prefix))
	}
}
