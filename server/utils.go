package main

import (
	"errors"
)

var (
	ErrMethodNotAllowed = errors.New("method not allowed")
	ErrInvalidRequestBody = errors.New("invalid request body")
	ErrUsernameTaken = errors.New("username already taken")
	ErrOutOfBounds = errors.New("index is out of bounds")
	ErrGameInProgress = errors.New("game in progress, please wait until game has finished")
	ErrNotInGame = errors.New("player is not in a game")
	ErrInGame = errors.New("player is already in the game")
	ErrRoundEnded = errors.New("round ended")
	ErrIncorrectRound = errors.New("current round is different from request round")
	ErrInsufficientCommandPoints = errors.New("not enought command points")
	ErrNegativeCommandPoints = errors.New("cannot use negative command points")
)

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

