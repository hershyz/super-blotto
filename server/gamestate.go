package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type moveRequest struct {
	Round int `json:"round"`
	Row	int `json:"row"`
	Col int `json:"col"`
	CommandPoints int `json:"commandPoints"`
}

type Player struct {
	Token 				string
	Username 			string

	Wins        	int
	Losses        int
	Ties        	int

	InGame 				bool
	GameID				int
	Role					PlayerRole // Player 0 or 1 
}

// All in-memory game state. mu is used to protect accesses to everything
type GameState struct {
	mu          sync.RWMutex
	NextGameID	int
	
	Players     map[string]*Player // Token -> *Player
	WaitingPlayers []*Player
	Games       map[int]*Game // GameID -> *Game
	Leaderboard []*Player

	// Round metadata
	Round 				int
	RoundEndTime	time.Time
	Phase 				GamePhase

	UsedUsernames map[string]struct{}
}

var gs = &GameState{
	Players:   make(map[string]*Player),
	Games: make(map[int]*Game),

	UsedUsernames: make(map[string]struct{}),
}

func (gs *GameState) registerPlayer(username, token string) (error) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if gs.Phase != Lobby {
		return ErrGameInProgress
	}

	if _, exists := gs.UsedUsernames[username]; exists {
		return ErrUsernameTaken
	}

	p := &Player{
		Token: 					token,
		Username:      	username,
		Role:						None,
	}

	gs.Players[token] = p
	gs.Leaderboard = append(gs.Leaderboard, p)

	gs.UsedUsernames[username] = struct{}{}

	return nil
}

func (gs *GameState) startRound() (time.Time) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	
	gs.Round++
	gs.RoundEndTime = time.Now().Add(time.Second * TimePerRound)
	gs.Phase = Playing
	
	return gs.RoundEndTime
}

func (gs *GameState) endRound() {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	gs.Phase = Resolving
	var wg sync.WaitGroup
	
	for _, g := range gs.Games {
		// Theoretically, each endRound calculation should be disjoin for one another, so can do it in parallel. Could be wrong tho. Also, this part doesn't really need to be fast so whateva
		wg.Add(1)
		go func(g *Game) { g.endRound() }(g)
	}
	wg.Wait()
}

func (gs *GameState) endGame() {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	gs.Phase = Finished
	var wg sync.WaitGroup
	for _, g := range gs.Games {
		// Theoretically, each endRound calculation should be disjoin for one another, so can do it in parallel. Could be wrong tho. Also, this part doesn't really need to be fast so whateva
		wg.Add(1)
		go func(g *Game) { g.endGame() }(g)
	}

	wg.Wait()
}

func (gs *GameState) playerFromToken(token string) (*Player, bool) {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	p, exists := gs.Players[token]

	return p, exists 
}

func playerFromContext(ctx context.Context) (*Player, bool) {
	p, ok := ctx.Value(playerKey{}).(*Player)
	return p, ok 
}

// Lobby phase for people to join the game. I think it is nice to have a lobby
// because people who are AFK won't be matched 
func (gs *GameState) handleLobby() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gs.mu.Lock()
		defer gs.mu.Unlock()

		// Can only go to lobby when the game is finished
		if (gs.Phase != Lobby && gs.Phase != Finished) {
			http.Error(w, ErrGameInProgress.Error(), http.StatusBadRequest)
			return
		}
		
		// Reinitialize game state
		clear(gs.Games)
		gs.Round = 0
		gs.Phase = Lobby
		
		// Reinitialize player state
		for _, p := range gs.Players {
			p.InGame = false
			p.GameID = NullGameID
			p.Role = None
		}
	})
}

// --- Handlers ---

func (gs *GameState) handleStart() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		
		gs.mu.Lock()
		defer gs.mu.Unlock()

		if (gs.Phase != Lobby) { 
			http.Error(w, ErrGameInProgress.Error(), http.StatusBadRequest)
			return
		}

		if (gs.Round != 0) { panic(fmt.Sprint("Game round not initialized correctly")) }

		gs.Phase = Playing

		for len(gs.WaitingPlayers) >= 2 {
			gs.NextGameID++

			p0 := gs.WaitingPlayers[0]
			p1 := gs.WaitingPlayers[1]
			gs.WaitingPlayers = gs.WaitingPlayers[2:]
			
			gs.Games[gs.NextGameID] = &Game{
				ID: gs.NextGameID,
				Players: [2]*Player{p0, p1},
				CommandPoints: [2]int{InitialCommandPoints, InitialCommandPoints},
			}

			p0.GameID = gs.NextGameID
			p1.GameID = gs.NextGameID
			p0.Role = Player0
			p1.Role = Player1
		}
		
		// set up timer
		go func(gs *GameState) {
			for i := 1; i <= TotalRounds; i++ {
				endtime := gs.startRound()

				time.Sleep(time.Until(endtime))

				if i == TotalRounds {
					gs.endGame()
				} else {
					gs.endRound()
				}
			}
		}(gs)
	})
}

func (gs *GameState) handleMove() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, ErrMethodNotAllowed.Error(), http.StatusMethodNotAllowed)
			return
		}
		
		var req moveRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, ErrInvalidRequestBody.Error(), http.StatusBadRequest)
			return
		}
		
		row := req.Row
		col := req.Col

		// --- game state agnostic checks ---

		if row < 0 || row >= GridWidth || 
		col < 0 || col >= GridHeight {
			http.Error(w, ErrOutOfBounds.Error(), http.StatusBadRequest)
			return
		}

		// --- game state specific checks ---

		p, ok := playerFromContext(r.Context())

		if !ok { panic(fmt.Sprintf("gs.playerFromContext failed")) }

		gs.mu.Lock()
		defer gs.mu.Unlock()

		if gs.Phase != Playing {
			http.Error(w, ErrRoundEnded.Error(), http.StatusBadRequest)
			return
		}

		if gs.Round != req.Round {
			http.Error(w, ErrIncorrectRound.Error(), http.StatusBadRequest)
			return
		}

		if p.InGame == false {
			if (p.GameID != NullGameID) { panic(fmt.Sprintf("GameID is not NullGameID when player is not in game")) }
			http.Error(w, ErrNotInGame.Error(), http.StatusBadRequest)
			return
		}
	
		if (p.GameID == NullGameID) { panic(fmt.Sprintf("GameID is NullGameID when player is in game")) }

		g, exists := gs.Games[p.GameID]

		if (exists == false) { panic(fmt.Sprintf("Game does not exist")) }
		
		err := g.move(row, col, req.CommandPoints, p.Role)
		if err != nil {
			http.Error(w, ErrNotInGame.Error(), http.StatusBadRequest)
			return
		}
	})
}

func (gs *GameState) handleJoin() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, ErrMethodNotAllowed.Error(), http.StatusMethodNotAllowed)
			return
		}
		
		p, ok := playerFromContext(r.Context())

		if !ok { panic(fmt.Sprintf("gs.playerFromContext failed")) }

		gs.mu.Lock()
		defer gs.mu.Unlock()

		if gs.Phase != Lobby {
			http.Error(w, ErrGameInProgress.Error(), http.StatusBadRequest)
			return
		}

		if p.InGame == true {
			// Maybe should error here?
			return
		}

		gs.WaitingPlayers = append(gs.WaitingPlayers, p)
	})
}

func (gs *GameState) handleLeave() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, ErrMethodNotAllowed.Error(), http.StatusMethodNotAllowed)
			return
		}
		
		p, ok := playerFromContext(r.Context())

		if !ok { panic(fmt.Sprintf("gs.playerFromContext failed")) }

		gs.mu.Lock()
		defer gs.mu.Unlock()
		
		if gs.Phase != Lobby {
			http.Error(w, ErrGameInProgress.Error(), http.StatusBadRequest)
			return
		}

		if p.InGame == true {
			// Maybe should error here?
			return
		}

		gs.WaitingPlayers = append(gs.WaitingPlayers, p)
	})
}

// TODO: 
// test :(
// Set up reseponses from all handlers
// 
