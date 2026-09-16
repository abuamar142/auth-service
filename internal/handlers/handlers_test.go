package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON_Success(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, "operation successful", map[string]string{"key": "value"})

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
	if resp.Message != "operation successful" {
		t.Errorf("expected message 'operation successful', got '%s'", resp.Message)
	}
	if resp.Error != nil {
		t.Error("expected no error in success response")
	}
}

func TestWriteJSON_WithNilData(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, "done", nil)

	var resp Response
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Data != nil {
		t.Error("expected data to be nil/omitted for nil input")
	}
	if resp.Success != true {
		t.Error("expected success=true")
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required", "field is empty")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var resp Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Success {
		t.Error("expected success=false")
	}
	if resp.Message != "name is required" {
		t.Errorf("expected message 'name is required', got '%s'", resp.Message)
	}
	if resp.Error == nil {
		t.Fatal("expected error body")
	}
	if resp.Error.Code != "VALIDATION_ERROR" {
		t.Errorf("expected error code 'VALIDATION_ERROR', got '%s'", resp.Error.Code)
	}
	if resp.Error.Details != "field is empty" {
		t.Errorf("expected error details 'field is empty', got '%s'", resp.Error.Details)
	}
}

func TestWriteError_EmptyDetails(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing token", "")

	var resp Response
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error.Details != "" {
		t.Errorf("expected empty details, got '%s'", resp.Error.Details)
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		wantErr  bool
	}{
		{"24h", "24h0m0s", false},
		{"7d", "168h0m0s", false},
		{"30d", "720h0m0s", false},
		{"1d", "24h0m0s", false},
		{"", "0s", false},
		{"10m", "10m0s", false},
		{"abc", "", true},
	}
	for _, tt := range tests {
		d, err := parseDuration(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseDuration(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && d.String() != tt.expected {
			t.Errorf("parseDuration(%q) = %v, want %v", tt.input, d, tt.expected)
		}
	}
}
