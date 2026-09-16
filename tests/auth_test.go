package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

var baseURL string

func TestMain(m *testing.M) {
	baseURL = os.Getenv("AUTH_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	for range 30 {
		resp, err := http.Get(baseURL + "/api/health")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			os.Exit(m.Run())
		}
		time.Sleep(time.Second)
	}
	fmt.Println("auth service not ready, skipping tests")
	os.Exit(1)
}

// --- helpers ---

func post(t *testing.T, path string, body interface{}) *http.Response {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := http.Post(baseURL+path, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST %s failed: %v", path, err)
	}
	return resp
}

func authReq(t *testing.T, method, path, token string, body interface{}) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, baseURL+path, reader)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s failed: %v", method, path, err)
	}
	return resp
}

func decodeResp(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return result
}

func loginUser(t *testing.T, identifier, password string) (string, string) {
	t.Helper()
	resp := post(t, "/api/v1/auth/login", map[string]string{
		"identifier": identifier,
		"password":   password,
	})
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("login failed: %d %s", resp.StatusCode, body)
	}
	var result struct {
		Data struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	resp.Body.Close()
	return result.Data.AccessToken, result.Data.RefreshToken
}

// --- Health ---

func TestHealth(t *testing.T) {
	resp := post(t, "/api/health", nil) // GET, but post helper still works
	resp2, err := http.Get(baseURL + "/api/health")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp2.Body.Close()
	resp.Body.Close()

	if resp2.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
	var body struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	json.NewDecoder(resp2.Body).Decode(&body)
	if !body.Success || body.Message != "service healthy" {
		t.Fatalf("unexpected: success=%v message=%s", body.Success, body.Message)
	}
}

// --- Register ---

func TestRegister_Success(t *testing.T) {
	email := fmt.Sprintf("test_%d@example.com", time.Now().UnixNano())
	resp := post(t, "/api/v1/auth/register", map[string]string{
		"email":       email,
		"username":    fmt.Sprintf("user_%d", time.Now().UnixNano()),
		"password":    "securepass123",
		"display_name": "Test User",
	})
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
	}
	r := decodeResp(t, resp)
	if r["success"] != true || r["message"] != "user registered" {
		t.Fatalf("unexpected: %v", r)
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	email := fmt.Sprintf("dup_%d@example.com", time.Now().UnixNano())
	// First registration
	post(t, "/api/v1/auth/register", map[string]string{
		"email":    email,
		"password": "pass12345",
	}).Body.Close()

	// Duplicate
	resp := post(t, "/api/v1/auth/register", map[string]string{
		"email":    email,
		"password": "pass12345",
	})
	defer resp.Body.Close()
	if resp.StatusCode != 409 {
		t.Fatalf("expected 409 for duplicate email, got %d", resp.StatusCode)
	}
	r := decodeResp(t, resp)
	if r["success"] != false {
		t.Error("expected success=false for duplicate")
	}
}

func TestRegister_DuplicateUsername(t *testing.T) {
	username := fmt.Sprintf("dupuser_%d", time.Now().UnixNano())
	post(t, "/api/v1/auth/register", map[string]string{
		"username": username,
		"password": "pass12345",
	}).Body.Close()

	resp := post(t, "/api/v1/auth/register", map[string]string{
		"username": username,
		"password": "pass12345",
	})
	defer resp.Body.Close()
	if resp.StatusCode != 409 {
		t.Fatalf("expected 409 for duplicate username, got %d", resp.StatusCode)
	}
}

func TestRegister_MissingIdentifier(t *testing.T) {
	resp := post(t, "/api/v1/auth/register", map[string]string{
		"password": "pass12345",
	})
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for missing email/username, got %d", resp.StatusCode)
	}
	r := decodeResp(t, resp)
	err := r["error"].(map[string]interface{})
	if err["code"] != "VALIDATION_ERROR" {
		t.Errorf("expected VALIDATION_ERROR, got %s", err["code"])
	}
}

func TestRegister_ShortPassword(t *testing.T) {
	resp := post(t, "/api/v1/auth/register", map[string]string{
		"email":    fmt.Sprintf("short_%d@example.com", time.Now().UnixNano()),
		"password": "123",
	})
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for short password, got %d", resp.StatusCode)
	}
}

func TestRegister_MissingPassword(t *testing.T) {
	resp := post(t, "/api/v1/auth/register", map[string]string{
		"email": fmt.Sprintf("nopass_%d@example.com", time.Now().UnixNano()),
	})
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for missing password, got %d", resp.StatusCode)
	}
}

// --- Login ---

func TestLogin_Success_Email(t *testing.T) {
	email := fmt.Sprintf("login_%d@example.com", time.Now().UnixNano())
	post(t, "/api/v1/auth/register", map[string]string{
		"email":    email,
		"password": "pass12345",
	}).Body.Close()

	resp := post(t, "/api/v1/auth/login", map[string]string{
		"identifier": email,
		"password":   "pass12345",
	})
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	r := decodeResp(t, resp)
	if r["success"] != true || r["message"] != "login successful" {
		t.Fatalf("unexpected: %v", r)
	}
}

func TestLogin_Success_Username(t *testing.T) {
	username := fmt.Sprintf("loguser_%d", time.Now().UnixNano())
	post(t, "/api/v1/auth/register", map[string]string{
		"username": username,
		"password": "pass12345",
	}).Body.Close()

	resp := post(t, "/api/v1/auth/login", map[string]string{
		"identifier": username,
		"password":   "pass12345",
	})
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	email := fmt.Sprintf("wrong_%d@example.com", time.Now().UnixNano())
	post(t, "/api/v1/auth/register", map[string]string{
		"email":    email,
		"password": "pass12345",
	}).Body.Close()

	resp := post(t, "/api/v1/auth/login", map[string]string{
		"identifier": email,
		"password":   "wrongpassword",
	})
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401 for wrong password, got %d", resp.StatusCode)
	}
	r := decodeResp(t, resp)
	if r["success"] != false {
		t.Error("expected success=false")
	}
}

func TestLogin_NonexistentUser(t *testing.T) {
	resp := post(t, "/api/v1/auth/login", map[string]string{
		"identifier": "nobody@example.com",
		"password":   "pass12345",
	})
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401 for nonexistent user, got %d", resp.StatusCode)
	}
}

func TestLogin_MissingFields(t *testing.T) {
	resp := post(t, "/api/v1/auth/login", map[string]string{
		"identifier": "",
		"password":   "",
	})
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for missing fields, got %d", resp.StatusCode)
	}
}

// --- Me ---

func TestMe_Authenticated(t *testing.T) {
	email := fmt.Sprintf("me_%d@example.com", time.Now().UnixNano())
	post(t, "/api/v1/auth/register", map[string]string{
		"email":       email,
		"password":    "pass12345",
		"display_name": "Me User",
	}).Body.Close()

	token, _ := loginUser(t, email, "pass12345")
	resp := authReq(t, "GET", "/api/v1/auth/me", token, nil)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	r := decodeResp(t, resp)
	if r["success"] != true || r["message"] != "user retrieved" {
		t.Fatalf("unexpected: %v", r)
	}
}

func TestMe_Unauthenticated(t *testing.T) {
	resp, err := http.Get(baseURL + "/api/v1/auth/me")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestMe_InvalidToken(t *testing.T) {
	resp := authReq(t, "GET", "/api/v1/auth/me", "invalid.jwt.token", nil)
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401 for invalid token, got %d", resp.StatusCode)
	}
}

// --- Refresh ---

func TestRefresh_Success(t *testing.T) {
	email := fmt.Sprintf("refresh_%d@example.com", time.Now().UnixNano())
	post(t, "/api/v1/auth/register", map[string]string{
		"email":    email,
		"password": "pass12345",
	}).Body.Close()

	_, refreshToken := loginUser(t, email, "pass12345")

	resp := post(t, "/api/v1/auth/refresh", map[string]string{
		"refresh_token": refreshToken,
	})
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	r := decodeResp(t, resp)
	if r["success"] != true || r["message"] != "token refreshed" {
		t.Fatalf("unexpected: %v", r)
	}
}

func TestRefresh_InvalidToken(t *testing.T) {
	resp := post(t, "/api/v1/auth/refresh", map[string]string{
		"refresh_token": "nonexistent-token",
	})
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestRefresh_RotationInvalidatesOld(t *testing.T) {
	email := fmt.Sprintf("rotate_%d@example.com", time.Now().UnixNano())
	post(t, "/api/v1/auth/register", map[string]string{
		"email":    email,
		"password": "pass12345",
	}).Body.Close()

	_, oldRefresh := loginUser(t, email, "pass12345")

	// First refresh — should succeed
	resp := post(t, "/api/v1/auth/refresh", map[string]string{
		"refresh_token": oldRefresh,
	})
	if resp.StatusCode != 200 {
		t.Fatalf("first refresh: expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Second refresh with same old token — should fail
	resp2 := post(t, "/api/v1/auth/refresh", map[string]string{
		"refresh_token": oldRefresh,
	})
	defer resp2.Body.Close()
	if resp2.StatusCode != 401 {
		t.Fatalf("second refresh: expected 401, got %d", resp2.StatusCode)
	}
}

func TestRefresh_MissingToken(t *testing.T) {
	resp := post(t, "/api/v1/auth/refresh", map[string]string{})
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// --- Logout ---

func TestLogout_Success(t *testing.T) {
	email := fmt.Sprintf("logout_%d@example.com", time.Now().UnixNano())
	post(t, "/api/v1/auth/register", map[string]string{
		"email":    email,
		"password": "pass12345",
	}).Body.Close()

	token, refreshToken := loginUser(t, email, "pass12345")

	resp := authReq(t, "POST", "/api/v1/auth/logout", token, map[string]string{
		"refresh_token": refreshToken,
	})
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	r := decodeResp(t, resp)
	if r["success"] != true || r["message"] != "logged out" {
		t.Fatalf("unexpected: %v", r)
	}
}

func TestLogout_Unauthenticated(t *testing.T) {
	resp := post(t, "/api/v1/auth/logout", map[string]string{
		"refresh_token": "some-token",
	})
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestLogout_MissingRefreshToken(t *testing.T) {
	email := fmt.Sprintf("logoutmiss_%d@example.com", time.Now().UnixNano())
	post(t, "/api/v1/auth/register", map[string]string{
		"email":    email,
		"password": "pass12345",
	}).Body.Close()

	token, _ := loginUser(t, email, "pass12345")

	resp := authReq(t, "POST", "/api/v1/auth/logout", token, map[string]string{})
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// --- API Keys ---

func TestAPIKey_CreateListDelete(t *testing.T) {
	email := fmt.Sprintf("apikey_%d@example.com", time.Now().UnixNano())
	post(t, "/api/v1/auth/register", map[string]string{
		"email":    email,
		"password": "pass12345",
	}).Body.Close()

	token, _ := loginUser(t, email, "pass12345")

	// Create
	resp := authReq(t, "POST", "/api/v1/api-keys", token, map[string]string{
		"name": "test-key",
	})
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("create: expected 201, got %d: %s", resp.StatusCode, body)
	}
	r := decodeResp(t, resp)
	if r["success"] != true || r["message"] != "api key created" {
		t.Fatalf("unexpected: %v", r)
	}

	// List
	resp2 := authReq(t, "GET", "/api/v1/api-keys", token, nil)
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Fatalf("list: expected 200, got %d", resp2.StatusCode)
	}

	// Delete (need to extract ID from create response)
	data := r["data"].(map[string]interface{})
	apiKey := data["api_key"].(map[string]interface{})
	keyID := apiKey["id"].(string)

	resp3 := authReq(t, "DELETE", "/api/v1/api-keys/"+keyID, token, nil)
	defer resp3.Body.Close()
	if resp3.StatusCode != 200 {
		t.Fatalf("delete: expected 200, got %d", resp3.StatusCode)
	}
	r3 := decodeResp(t, resp3)
	if r3["success"] != true || r3["message"] != "api key deleted" {
		t.Fatalf("unexpected: %v", r3)
	}
}

func TestAPIKey_Unauthenticated(t *testing.T) {
	resp, err := http.Get(baseURL + "/api/v1/api-keys")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAPIKey_CreateMissingName(t *testing.T) {
	email := fmt.Sprintf("apikey2_%d@example.com", time.Now().UnixNano())
	post(t, "/api/v1/auth/register", map[string]string{
		"email":    email,
		"password": "pass12345",
	}).Body.Close()
	token, _ := loginUser(t, email, "pass12345")

	resp := authReq(t, "POST", "/api/v1/api-keys", token, map[string]string{})
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for missing name, got %d", resp.StatusCode)
	}
}

// --- Full lifecycle ---

func TestFullLifecycle_RegisterLoginMeRefreshReuseLogoutRefresh(t *testing.T) {
	email := fmt.Sprintf("full_%d@example.com", time.Now().UnixNano())
	username := fmt.Sprintf("fulluser_%d", time.Now().UnixNano())

	// Register
	resp := post(t, "/api/v1/auth/register", map[string]string{
		"email":        email,
		"username":     username,
		"password":     "securepass123",
		"display_name": "Full User",
	})
	if resp.StatusCode != 201 {
		t.Fatalf("register: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Login
	token, refreshToken := loginUser(t, email, "securepass123")
	if token == "" || refreshToken == "" {
		t.Fatal("login: missing tokens")
	}

	// Me
	resp2 := authReq(t, "GET", "/api/v1/auth/me", token, nil)
	if resp2.StatusCode != 200 {
		t.Fatalf("me: %d", resp2.StatusCode)
	}
	resp2.Body.Close()

	// Refresh
	resp3 := post(t, "/api/v1/auth/refresh", map[string]string{"refresh_token": refreshToken})
	if resp3.StatusCode != 200 {
		t.Fatalf("refresh: %d", resp3.StatusCode)
	}
	var refreshData struct {
		Data struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	json.NewDecoder(resp3.Body).Decode(&refreshData)
	resp3.Body.Close()

	// Reuse old refresh → 401
	resp4 := post(t, "/api/v1/auth/refresh", map[string]string{"refresh_token": refreshToken})
	if resp4.StatusCode != 401 {
		t.Fatalf("reuse old refresh: expected 401, got %d", resp4.StatusCode)
	}
	resp4.Body.Close()

	// Logout
	resp5 := authReq(t, "POST", "/api/v1/auth/logout", refreshData.Data.AccessToken, map[string]string{
		"refresh_token": refreshData.Data.RefreshToken,
	})
	if resp5.StatusCode != 200 {
		t.Fatalf("logout: %d", resp5.StatusCode)
	}
	resp5.Body.Close()

	// Refresh after logout → 401
	resp6 := post(t, "/api/v1/auth/refresh", map[string]string{"refresh_token": refreshData.Data.RefreshToken})
	if resp6.StatusCode != 401 {
		t.Fatalf("refresh after logout: expected 401, got %d", resp6.StatusCode)
	}
	resp6.Body.Close()
}
