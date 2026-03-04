package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func resetTokenStore() {
	storeMu.Lock()
	tokenStore = make(map[string]string)
	storeMu.Unlock()
}

func TestRegisterSuccess(t *testing.T) {
	resetTokenStore()

	body := `{"username":"alice"}`
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handleRegister(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp registerResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Token == "" {
		t.Fatal("expected non-empty token")
	}

	storeMu.RLock()
	stored := tokenStore["alice"]
	storeMu.RUnlock()
	if stored != resp.Token {
		t.Fatalf("token in store (%s) does not match response (%s)", stored, resp.Token)
	}
}

func TestRegisterMissingUsername(t *testing.T) {
	resetTokenStore()

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handleRegister(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestRegisterWrongMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/register", nil)
	w := httptest.NewRecorder()

	handleRegister(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestRegisterOverwritesToken(t *testing.T) {
	resetTokenStore()

	// Register once
	body := `{"username":"bob"}`
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	handleRegister(w, req)

	var first registerResponse
	json.NewDecoder(w.Body).Decode(&first)

	// Register again
	req = httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(body))
	w = httptest.NewRecorder()
	handleRegister(w, req)

	var second registerResponse
	json.NewDecoder(w.Body).Decode(&second)

	if first.Token == second.Token {
		t.Fatal("expected different token on re-register")
	}

	storeMu.RLock()
	stored := tokenStore["bob"]
	storeMu.RUnlock()
	if stored != second.Token {
		t.Fatal("store should hold the latest token")
	}
}

func TestValidateToken(t *testing.T) {
	resetTokenStore()

	// Register
	body := `{"username":"carol"}`
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	handleRegister(w, req)

	var resp registerResponse
	json.NewDecoder(w.Body).Decode(&resp)

	// Valid token
	authReq := httptest.NewRequest(http.MethodGet, "/", nil)
	authReq.Header.Set("Authorization", resp.Token)
	username, ok := validateToken(authReq)
	if !ok || username != "carol" {
		t.Fatalf("expected valid token for carol, got ok=%v username=%s", ok, username)
	}

	// Invalid token
	authReq = httptest.NewRequest(http.MethodGet, "/", nil)
	authReq.Header.Set("Authorization", "bogus")
	_, ok = validateToken(authReq)
	if ok {
		t.Fatal("expected invalid token")
	}

	// Missing header
	authReq = httptest.NewRequest(http.MethodGet, "/", nil)
	_, ok = validateToken(authReq)
	if ok {
		t.Fatal("expected invalid when no header")
	}
}
