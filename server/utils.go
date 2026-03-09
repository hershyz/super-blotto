package main

import (
	"errors"
	"net/http"
	"encoding/json"
	"fmt"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

// Errors
var (
	ErrMethodNotAllowed = errors.New("method not allowed")

	ErrInvalidRequestBody = errors.New("invalid request body")
	ErrOutOfBounds = errors.New("index is out of bounds")
	ErrIncorrectRound = errors.New("current round is different from request round")
	ErrInsufficientCommandPoints = errors.New("not enought command points")
	ErrNegativeCommandPoints = errors.New("cannot use negative command points")

	ErrNotInGame = errors.New("player is not in a game")
	ErrInGame = errors.New("player is already in the game")

	ErrUsernameTaken = errors.New("username already taken")
	ErrGameInProgress = errors.New("game in progress, please wait until game has finished")
	ErrRoundEnded = errors.New("round ended")
	ErrNoParticipants = errors.New("no participants have joined yet")
	ErrOddParticipants = errors.New("game can only start if an even number of participants join")
	ErrInLobby = errors.New("game is already in lobby")
)

var ErrorToStatus = map[error]int{
	// 405 Method Not Allowed
	ErrMethodNotAllowed: http.StatusMethodNotAllowed,

	// 400 Bad Request (Client sent data that is logically wrong)
	ErrInvalidRequestBody:        http.StatusBadRequest,
	ErrOutOfBounds:               http.StatusBadRequest,
	ErrIncorrectRound:            http.StatusBadRequest,
	ErrInsufficientCommandPoints: http.StatusBadRequest,
	ErrNegativeCommandPoints:     http.StatusBadRequest,

	// 403 Forbidden (Authenticated, but not allowed to do this now)
	ErrNotInGame: http.StatusForbidden,
	ErrInGame:    http.StatusForbidden,

	// 409 Conflict (The request conflicts with the current server state)
	ErrUsernameTaken:   http.StatusConflict,
	ErrGameInProgress:  http.StatusConflict,
	ErrRoundEnded:      http.StatusConflict,
	ErrNoParticipants:  http.StatusConflict,
	ErrOddParticipants: http.StatusConflict,
	ErrInLobby:  				http.StatusConflict,
}

var (
	dirs = [4][2]int{
		{-1, 0}, // up
		{1, 0},  // down
		{0, -1}, // left
		{0, 1},  // right
	}
)

const (
	NullGameID = 0
	GridHeight = 10
	GridWidth = 10
	InitialCommandPoints = 1000
	TotalRounds = 10
	TimePerRound = 30
	AdminToken = "AdminToken"
)

type GamePhase int

const (
	Lobby GamePhase = iota
	Playing
	Resolving
	Finished
)

type PlayerRole int

const (
	None PlayerRole = iota-1
	Player0
	Player1
)

type playerKey struct{} // Key to get player pointer inside http request

func getStatus(err error) int {
	if status, ok := ErrorToStatus[err]; ok {
		return status
	}
	return http.StatusInternalServerError // Default to 500
}

func encode[T any](w http.ResponseWriter, status int, resp T) error {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    if err := json.NewEncoder(w).Encode(resp); err != nil {
        return fmt.Errorf("encode json: %w", err)
    }
    return nil
}

func encodeError(w http.ResponseWriter, err error) {
    status := getStatus(err)
    encode(w, status, ErrorResponse{Error: err.Error()})
}

func decode[T any](r *http.Request, method string) (T, error) {
    var req T
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        return req, fmt.Errorf("decode json: %w", err)
    }
    return req, nil
}
