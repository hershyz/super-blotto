package main

import (
	"fmt"
	"sync"
)

type PlayerData struct {
	Username string `json:"username"`
	Stats PlayerStats `json:"stats"`
	Playing bool 			`json:"playing"`
	CommandPoints int `json:"CommandPoints,omitempty"` // Empty if gamephase is Lobby
	Board [GridHeight][GridWidth]int `json:"board,omitempty"` // Empty if gamephase is Lobby
}

type PlayerStats struct {
	Wins        	int
	Losses        int
	Ties        	int
}

type Player struct {
	mu 						sync.RWMutex

	IsAdmin   		bool
	Token 				string
	Username 			string
	
	// General player state
	Stats 				PlayerStats
	Playing 			bool // True if player has joined the game or is currently playing in the game

	// Game state. Should be default initialized in lobby
	Opponent			*Player
	CommandPoints	int
	Board 				[GridHeight][GridWidth]int // This players point allocations
	VisibleCommandPoints	int 							// The CommandPoints that the opponent can see. This is update at the end of each round
	VisibleBoard [GridHeight][GridWidth]int // The board that the opponents can see. This is update at the end of each round
}

func (p *Player) getUsername() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.Username
}

func (p *Player) getLeaderboardData() PlayerData {
	p.mu.RLock()
	defer p.mu.RUnlock()

	data := PlayerData{
		Username: p.Username,
		Stats: p.Stats,
		Playing: p.Playing,
	}

	return data
}

func (p *Player) getGameData(op bool) PlayerData {
	p.mu.RLock()
	defer p.mu.RUnlock()

	data := PlayerData{
		Username: p.Username,
		Stats: p.Stats,
		Playing: p.Playing,
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

func (p *Player) getOpponent() *Player {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	return p.Opponent
}

// All functions are called assuming the caller holds the gs.mu lock
func (p *Player) generatePoints(row, col int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, d := range dirs {
		nr := row + d[0]
		nc := col + d[1]

		if nr < 0 || nr >= GridHeight ||
		nc < 0 || nc >= GridWidth {
			continue
		}
		p.CommandPoints += 5
	}
}

func (me *Player) endRound() {
	me.mu.Lock()
	defer me.mu.Unlock()

	op := me.Opponent
	op.mu.Lock()
	defer op.mu.Unlock()

	myBoard := me.Board
	opBoard := op.Board

	me.VisibleBoard = myBoard
	op.VisibleBoard = opBoard

	for row := 0; row < GridHeight; row++ {
		for col := 0; col < GridWidth; col++ {
			myPoints := myBoard[row][col]
			opPoints := opBoard[row][col]

			if myPoints > opPoints {
				me.generatePoints(row, col)
			} else if myPoints < opPoints {
				op.generatePoints(row, col)
			} else {
				continue
			}
		}
	}
}

func (me *Player) endGame() {
	me.mu.Lock()
	defer me.mu.Unlock()
	
	op := me.Opponent
	op.mu.Lock()
	defer op.mu.Unlock()

	myBoard := me.Board
	opBoard := op.Board

	me.VisibleBoard = myBoard
	op.VisibleBoard = opBoard

	myCount := 0
	opCount := 0

	for row := 0; row < GridHeight; row++ {
		for col := 0; col < GridWidth; col++ {
			myPoints := myBoard[row][col]
			opPoints := opBoard[row][col]

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
	p.mu.Lock()
	defer p.mu.Unlock()

	p.Playing = false

	p.Opponent = nil
	p.CommandPoints = InitialCommandPoints
	p.Board = [GridHeight][GridWidth]int{}
	p.VisibleCommandPoints = InitialCommandPoints
	p.VisibleBoard = [GridHeight][GridWidth]int{}
}

func (p *Player) start(op *Player) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.Playing == false { panic(fmt.Sprint("p.Playing is false when starting game")) }

	if p.Opponent != nil { panic(fmt.Sprint("p.Opponent is not initialized to nil")) }
	if p.CommandPoints != InitialCommandPoints { panic(fmt.Sprint("p.CommandPoints is not initialized to InitialCommandPoints")) }
	if p.Board != ([GridHeight][GridWidth]int{}) { panic(fmt.Sprint("p.Board is not initialized to all zero")) }
	if p.VisibleCommandPoints != InitialCommandPoints { panic(fmt.Sprint("p.OpponentCommandPoints is not initialized to InitialCommandPoints")) }
	if p.VisibleBoard != ([GridHeight][GridWidth]int{}) { panic(fmt.Sprint("p.OpponentBoard is not initialized to all zero")) }

	p.Opponent = op
}

// Row and Col must be validated before calling move
func (p *Player) move(row, col, reqCommandPoints int) (error) {
	if p.Playing == false 								{ return ErrNotInGame }
	if p.CommandPoints < reqCommandPoints { return ErrInsufficientCommandPoints }

	p.CommandPoints -= reqCommandPoints
	p.Board[row][col] += reqCommandPoints
	return nil
}

func (p *Player) join() (error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.Playing == true { return ErrAlreadyJoined }

	p.Playing = true
	return nil
}

func (p *Player) leave() (error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.Playing == false { return ErrNotJoined }

	p.Playing = false
	return nil
}

func (p *Player) state() (PlayerData) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return PlayerData{
		
	}
}
