package main

import (
	"fmt"
	"sync"
)

type PlayerData struct {
	Username      string                     `json:"username"`
	Stats         PlayerStats                `json:"stats"`
	Playing       bool                       `json:"playing"`
	CommandPoints int                        `json:"CommandPoints,omitempty"` // Empty if gamephase is Lobby
	Board         [GridHeight][GridWidth]int `json:"board,omitempty"`         // Empty if gamephase is Lobby
}

type PlayerStats struct {
	Wins   int
	Losses int
	Ties   int
}

type Player struct {
	mu sync.RWMutex

	IsAdmin  bool
	Token    string
	Username string

	// General player state
	Stats   PlayerStats
	Playing bool // True if player has joined the game or is currently playing in the game

	// Game state. Should be default initialized in lobby
	Opponent             *Player
	CommandPoints        int
	VisibleCommandPoints int                        // The CommandPoints that the opponent can see. This is updated at the end of each round
	Board                [GridHeight][GridWidth]int // This players point allocations
	VisibleBoard         [GridHeight][GridWidth]int // The board that the opponents can see. This is updated at the end of each round
}

func NewPlayer(token, username string) *Player {
	return &Player{
		Token:    token,
		Username: username,

		CommandPoints: 				InitialCommandPoints,
		VisibleCommandPoints: InitialCommandPoints,
	}
}

func NewAdminPlayer() *Player {
	return &Player{
		IsAdmin:  true,
		Username: AdminUsername,
		Token:    AdminToken,

		CommandPoints: 				InitialCommandPoints,
		VisibleCommandPoints: InitialCommandPoints,
	}
}

// --- gs.Lock() held ---

func generatePoints(row, col int, board *[GridHeight][GridWidth]int) {
	for _, d := range dirs {
		nr := row + d[0]
		nc := col + d[1]

		if nr < 0 || nr >= GridHeight ||
			nc < 0 || nc >= GridWidth {
			continue
		}

		board[nr][nc] += 5
	}
}

// Only ever called on p0
func (me *Player) endRound() {
	op := me.Opponent

	// myBoard and opBoard represent the next state of the board
	// Use me.Board and op.Board when determining which player
	// owns a cell so that newly generated points don't affect
	// later comparisons
	myBoard := me.Board
	opBoard := op.Board

	for row := 0; row < GridHeight; row++ {
		for col := 0; col < GridWidth; col++ {
			myPoints := me.Board[row][col]
			opPoints := op.Board[row][col]

			if myPoints > opPoints {
				generatePoints(row, col, &myBoard)
			} else if myPoints < opPoints {
				generatePoints(row, col, &opBoard)
			} else {
				continue
			}
		}
	}

	me.Board = myBoard
	op.Board = opBoard

	me.VisibleBoard = myBoard
	op.VisibleBoard = opBoard
	me.VisibleCommandPoints = me.CommandPoints
	op.VisibleCommandPoints = op.CommandPoints
}

// Only ever called on p0
func (me *Player) endGame() {
	op := me.Opponent

	myCount := 0
	opCount := 0

	for row := 0; row < GridHeight; row++ {
		for col := 0; col < GridWidth; col++ {
			myPoints := me.Board[row][col]
			opPoints := op.Board[row][col]

			if myPoints > opPoints {
				myCount++
			} else if myPoints < opPoints {
				opCount++
			} else {
				continue
			}
		}
	}

	if myCount > opCount {
		me.Stats.Wins++
		op.Stats.Losses++
	} else if myCount < opCount {
		me.Stats.Losses++
		op.Stats.Wins++
	} else {
		me.Stats.Ties++
		op.Stats.Ties++
	}
}

func (p *Player) gotoLobby() {
	p.Playing = false
	p.Opponent = nil

	p.CommandPoints 				= InitialCommandPoints
	p.VisibleCommandPoints 	= InitialCommandPoints

	p.Board 				= [GridHeight][GridWidth]int{}
	p.VisibleBoard 	= [GridHeight][GridWidth]int{}
}

func (p *Player) startGame(op *Player) {
	if p.Playing == false {
		panic(fmt.Sprint("p.Playing is false when starting game"))
	}
	if p.Opponent != nil {
		panic(fmt.Sprint("p.Opponent is not initialized to nil"))
	}
	if p.CommandPoints != InitialCommandPoints {
		panic(fmt.Sprint("p.CommandPoints is not initialized to InitialCommandPoints"))
	}
	if p.VisibleCommandPoints != InitialCommandPoints {
		panic(fmt.Sprint("p.OpponentCommandPoints is not initialized to InitialCommandPoints"))
	}
	if p.Board != ([GridHeight][GridWidth]int{}) {
		panic(fmt.Sprint("p.Board is not initialized to all zero"))
	}
	if p.VisibleBoard != ([GridHeight][GridWidth]int{}) {
		panic(fmt.Sprint("p.OpponentBoard is not initialized to all zero"))
	}

	p.Opponent = op
}

func (p *Player) join() error {
	if p.Playing == true {
		return ErrAlreadyJoined
	}

	p.Playing = true
	return nil
}

func (p *Player) leave() error {
	if p.Playing == false {
		return ErrNotJoined
	}

	p.Playing = false
	return nil
}


// --- gs.RLock() held ---


func (p *Player) getUsername() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.Username
}

func (p *Player) getOpponent() *Player {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.Opponent
}

func (p *Player) getGameData(op bool) PlayerData {
	p.mu.RLock()
	defer p.mu.RUnlock()

	data := PlayerData{
		Username: p.Username,
		Stats:    p.Stats,
		Playing:  p.Playing,
	}

	if op == true {
		data.CommandPoints = p.VisibleCommandPoints
		data.Board = p.VisibleBoard
	} else {
		data.CommandPoints = p.CommandPoints
		data.Board = p.Board
	}

	return data
}

// Row and Col must be validated before calling move
func (p *Player) move(row, col, reqCommandPoints int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Playing == false {
		return ErrNotInGame
	}
	if p.CommandPoints < reqCommandPoints {
		return ErrInsufficientCommandPoints
	}

	p.CommandPoints -= reqCommandPoints
	p.Board[row][col] += reqCommandPoints
	return nil
}

func (p *Player) getLeaderboardData() PlayerData {
	p.mu.RLock()
	defer p.mu.RUnlock()

	data := PlayerData{
		Username: p.Username,
		Stats:    p.Stats,
		Playing:  p.Playing,
	}

	return data
}
