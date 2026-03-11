package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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

		if err = getGameState().registerPlayer(token, req.Username); err != nil {
			encodeError(w, err)
			return
		}

		encode(w, http.StatusOK, registerResponse{Token: token})
	})
}

// validate checks the Authorization header for valid token before handling request
func validate(f func(*GameState) http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")

		gs := getGameState()
		p, exists := gs.playerFromToken(token)

		if exists == false {
			encodeError(w, ErrInvalidToken)
			return
		}

		ctx := context.WithValue(r.Context(), playerKey{}, p)

		f(gs).ServeHTTP(w, r.WithContext(ctx))
	})
}

func adminOnly(f func(*GameState) http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		
		gs := getGameState()
		p, exists := gs.playerFromToken(token)

		if exists == false || p.IsAdmin == false {
			http.NotFound(w, r)
			return
		}

		f(gs).ServeHTTP(w, r)
	})
}
