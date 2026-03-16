package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
)

func generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func handleRegister() http.Handler {
	type registerRequest struct {
		Username string `json:"username"`
	}

	type registerResponse struct {
		Token string `json:"token"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, err := decode[registerRequest](r, http.MethodPost)
		if err != nil {
			encodeError(w, err)
			return
		}

		if req.Username == "" {
			encodeError(w, ErrUsernameRequired)
			return
		}

		token, err := generateToken()
		if err != nil {
			encodeError(w, ErrGeneratingToken)
			return
		}

		if err = gs.registerPlayer(req.Username, token); err != nil {
			encodeError(w, err)
			return
		}

		log.Printf("player registered: username=%s token=%s", req.Username, token)
		encode(w, http.StatusOK, registerResponse{Token: token})
	})
}

// validate checks the Authorization header for valid token before handling request
func validate(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")

		p, exists := gs.playerFromToken(token)

		if exists == false {
			encodeError(w, ErrInvalidToken)
			return
		}

		ctx := context.WithValue(r.Context(), playerKey{}, p)

		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

func handleAdminStatus() http.Handler {
	type adminStatusResponse struct {
		Phase         string `json:"phase"`
		Round         int    `json:"round"`
		WaitingCount  int    `json:"waitingCount"`
		RegisteredCount int  `json:"registeredCount"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gs.mu.RLock()
		defer gs.mu.RUnlock()

		encode(w, http.StatusOK, adminStatusResponse{
			Phase:           phaseStr(gs.Phase),
			Round:           gs.Round,
			WaitingCount:    len(gs.WaitingPlayers),
			RegisteredCount: len(gs.Players),
		})
	})
}

func handleAdminPing() http.Handler {
	type adminPingResponse struct{}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		encode(w, http.StatusOK, adminPingResponse{})
	})
}

func adminOnly(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")

		if token != AdminToken {
			http.NotFound(w, r)
			return
		}

		h.ServeHTTP(w, r)
	})
}
