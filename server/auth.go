package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
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

func handleRegister() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		err = registerPlayer(req.Username, token)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(registerResponse{Token: token})
	})
}

// validateToken checks the Authorization header and returns the username if valid.
func validate(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")

		gs.mu.RLock()
		_, exists := gs.TokensToUsername[token]
		gs.mu.RUnlock()

		if exists == false {
			http.Error(w, "token is invalid", http.StatusInternalServerError)
			return
		} 		

		h.ServeHTTP(w, r)
	})
}

func adminOnly(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: Admin auth logic

		// If current user is not admin {
		//		http.NotFound(w, r)
		//		return
		// }
		h.ServeHTTP(w, r)
	})
}
