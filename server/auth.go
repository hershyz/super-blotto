package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"
)

var (
	tokenStore = make(map[string]string) // username -> token
	storeMu    sync.RWMutex
)

type registerRequest struct {
	Username string `json:"username"`
}

type registerResponse struct {
	Token string `json:"token"`
}

func generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Username == "" {
		http.Error(w, "username is required", http.StatusBadRequest)
		return
	}

	token, err := generateToken()
	if err != nil {
		http.Error(w, "failed to generate token", http.StatusInternalServerError)
		return
	}

	storeMu.Lock()
	tokenStore[req.Username] = token
	storeMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(registerResponse{Token: token})
}

// validateToken checks the Authorization header and returns the username if valid.
func validateToken(r *http.Request) (string, bool) {
	token := r.Header.Get("Authorization")
	if token == "" {
		return "", false
	}

	storeMu.RLock()
	defer storeMu.RUnlock()
	for username, t := range tokenStore {
		if t == token {
			return username, true
		}
	}
	return "", false
}
