package main

import (
	"errors"
	"net/http"
	"sync"
	"time"
)

var ErrUsernameTaken = errors.New("username already taken")

const (
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

type CellData struct {
	PointsA int
	PointsB int
}
 
type Game struct {
	mu 						sync.RWMutex
	ID 						int
	PlayerA 			*Player
	PlayerB 			*Player
	Board 				[GridHeight][GridWidth]CellData
	Round 				int
	RoundEndTime	time.Time
	Phase 				GamePhase
}

type Player struct {
	Token 				string
	Username 			string
	CommandPoints int
	GameID				int
}

// All in-memory game state. mu is used to protect accesses to the
// Players, TokenStore, and Leaderboard maps, and is used to protect
// surface level accesses to the Game map. Further access to specific 
// games require grabbing individual game locks.
type GameState struct {
	mu          sync.RWMutex
	Started 		bool
	NextGameID	int

	Players     map[string]*Player // Token -> *Player
	WaitingPlayers []*Player // Players Waiting to be paired
	Games       map[int]*Game // GameID -> *Game
	Leaderboard []*Player

	UsernameToTokens map[string]string  // Username 	-> Token
	TokensToUsername map[string]string  // Token 		-> Username
}

var gs = &GameState{
	Players:   make(map[string]*Player), 
	UsernameToTokens: make(map[string]string),
	TokensToUsername: make(map[string]string),
}

func registerPlayer(username, token string) (error) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if _, exists := gs.UsernameToTokens[username]; exists {
		return ErrUsernameTaken
	}

	p := &Player{
		Token: 					token,
		Username:      	username,
		CommandPoints: 	InitialCommandPoints,
	}

	gs.Players[token] = p
	gs.WaitingPlayers = append(gs.WaitingPlayers, p)
	gs.Leaderboard = append(gs.Leaderboard, p)

	gs.TokensToUsername[token] = username
	gs.UsernameToTokens[username] = token

	return nil
}

func (g *Game) startRound() (time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	g.RoundEndTime = time.Now().Add(time.Second * TimePerRound)
	


	return g.RoundEndTime
}

func (g *Game) endRound() {
	g.mu.Lock()
	defer g.mu.Unlock()

	// TODO: Round end logic
}

func handleStart() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gs.mu.Lock()
		defer gs.mu.Unlock()

		for i := 0; i + 1 < len(gs.WaitingPlayers); i += 2 {
			gs.NextGameID++

			pa := gs.WaitingPlayers[i]
			pb := gs.WaitingPlayers[i+1]
			gs.WaitingPlayers = gs.WaitingPlayers[2:]
			
			gs.Games[gs.NextGameID] = &Game{
				ID: gs.NextGameID,
				PlayerA: pa,
				PlayerB: pb,
				Phase: Lobby,
			}

			pa.GameID = gs.NextGameID
			pb.GameID = gs.NextGameID

			// set up timer
			go func(g *Game) {
				for i := 0; i < 10; i++ {

					time.Sleep(time.Until(endtime))

					g.endRound()
				}
			}(gs.Games[gs.NextGameID])
		}
	})
}

func handleMove() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		
	})
}
