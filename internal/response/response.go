package response

import (
	"encoding/json"
	"net/http"
)

// Response is the standard API response envelope.
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Error   *ErrorBody  `json:"error,omitempty"`
}

// ErrorBody is the error detail in a response.
type ErrorBody struct {
	Code    string `json:"code"`
	Details string `json:"details,omitempty"`
}

// JSON writes a success JSON response.
func JSON(w http.ResponseWriter, status int, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Response{
		Success: status >= 200 && status < 300,
		Message: message,
		Data:    data,
	})
}

// Error writes a standard JSON error response. Usable from middleware and handlers.
func Error(w http.ResponseWriter, status int, code, message, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	errBody := &ErrorBody{Code: code}
	if details != "" {
		errBody.Details = details
	}
	json.NewEncoder(w).Encode(Response{
		Success: false,
		Message: message,
		Error:   errBody,
	})
}
