package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/abuamar142/auth-service/internal/response"
	"github.com/abuamar142/auth-service/internal/services"
)

// Every error the admin endpoints can return has to reach the caller as a
// readable status. The `default` branch answers 500, so a sentinel error
// added without a case would turn "you cannot delete your own account" into
// "internal server error" — a real answer the UI could show as a fault.
func TestWriteServiceError_MapsEverySentinel(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
		code string
	}{
		{"not found", services.ErrUserNotFound, http.StatusNotFound, "NOT_FOUND"},
		{"email taken", services.ErrEmailExists, http.StatusConflict, "IDENTIFIER_TAKEN"},
		{"username taken", services.ErrUsernameExists, http.StatusConflict, "IDENTIFIER_TAKEN"},
		{"no identifier", services.ErrIdentifierRequired, http.StatusBadRequest, "VALIDATION_ERROR"},
		{"password too short", services.ErrPasswordTooShort, http.StatusBadRequest, "VALIDATION_ERROR"},
		{"delete self", services.ErrCannotDeleteSelf, http.StatusConflict, "CANNOT_DELETE_SELF"},
		{"demote self", services.ErrCannotDemoteSelf, http.StatusConflict, "CANNOT_DEMOTE_SELF"},
		{"last admin", services.ErrLastAdmin, http.StatusConflict, "LAST_ADMIN"},
		{"nothing to update", services.ErrNothingToUpdate, http.StatusBadRequest, "VALIDATION_ERROR"},
		{"missing id", services.ErrUserIDRequired, http.StatusBadRequest, "VALIDATION_ERROR"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeServiceError(rec, tc.err)

			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}

			var envelope response.Response
			if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
				t.Fatalf("decoding response: %v", err)
			}
			if envelope.Success {
				t.Error("success = true for an error response")
			}
			if envelope.Error == nil || envelope.Error.Code != tc.code {
				t.Errorf("error code = %v, want %s", envelope.Error, tc.code)
			}
		})
	}
}

// Unrecognised errors stay a 500 rather than leaking their message: a
// database error can carry a table name or a constraint.
func TestWriteServiceError_UnknownStaysOpaque(t *testing.T) {
	rec := httptest.NewRecorder()
	writeServiceError(rec, errors.New("pq: relation \"users\" does not exist"))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	var envelope response.Response
	if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if envelope.Error == nil || envelope.Error.Code != "INTERNAL_ERROR" {
		t.Errorf("error code = %v, want INTERNAL_ERROR", envelope.Error)
	}
	if envelope.Error != nil && envelope.Error.Details != "" {
		t.Errorf("details = %q, want empty", envelope.Error.Details)
	}
}
