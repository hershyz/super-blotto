package main

import (
	"context"
	"math/rand"
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

	InGame 				bool // True if player has joined the game or is currently playing in the game
	GameID				int
	Role					PlayerRole // Player 0 or 1 
}

// All in-memory game state. mu is used to protect accesses to everything
type GameState struct {
	mu          sync.RWMutex
	NextGameID	int
	
	Players     map[string]*Player // Token -> *Player
	WaitingPlayers map[*Player]struct{}
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
	WaitingPlayers: make(map[*Player]struct{}),
	Games: make(map[int]*Game),

	UsedUsernames: make(map[string]struct{}),
}


// Registers a player into the backend player store. The players username must 
// be unique, and the game phase must be in Lobby. The players are not automatically 
// added to the game, they must post a join request to be added to the 
// gs.WaitingPlayers set
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



// --- Handlers ---



// Lobby phase for people to join the game. I think it is nice to have a lobby
// because people who are AFK won't be matched. Could also add a heartbeat system
// and reuse the handleLeave function
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

// Starts a game. The game phase must be in Lobby and must have and even number
// of participants to start a game. This handler can only be called by an admin,
// so it should be wrapped by an adminOnly() call.
func (gs *GameState) handleStart() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		
		gs.mu.Lock()
		defer gs.mu.Unlock()

		if (gs.Phase != Lobby) { 
			http.Error(w, ErrGameInProgress.Error(), http.StatusBadRequest)
			return
		}

		if len(gs.WaitingPlayers) == 0 {
			http.Error(w, ErrNoParticipants.Error(), http.StatusBadRequest)
			return
		}

		if (len(gs.WaitingPlayers) % 2) != 0 { 
			http.Error(w, ErrOddParticipants.Error(), http.StatusBadRequest)
			return
		}

		if (gs.Round != 0) { panic(fmt.Sprint("Game round not initialized correctly")) }

		gs.Phase = Playing

		waitingPlayers := make([]*Player, 0, len(gs.WaitingPlayers))
		
		for p := range gs.WaitingPlayers {
			waitingPlayers = append(waitingPlayers, p)
		}

		clear(gs.WaitingPlayers)
	
		// Shuffle waiting players
		rand.Shuffle(len(waitingPlayers), func(i, j int) {
			waitingPlayers[i], waitingPlayers[j] = waitingPlayers[j], waitingPlayers[i]
		})

		for len(waitingPlayers) >= 2 {
			gs.NextGameID++

			p0 := waitingPlayers[0]
			p1 := waitingPlayers[1]
			waitingPlayers = waitingPlayers[2:]
			
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

// Handles a move from a player. Player moves include the round (so movees from 
// previous rounds don't affect the current round and are dropped), row, col, and 
// CommandPoints. Moves can only be made by registered players that are in the game.
// Thus, this handler should be wrapped with a validate() call. Validate will pass
// the player pointer into the function through the http.Request context with key
// playerKey{}.
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

// Handles a join from a player. Players that are registered are not automatically
// added to the game. They need to send a post request to the /join path to be added.
// Join requests can only be sent by players who have been registered. Thus, this 
// handler should be wrapped with a validate() call. Validate will pass the player 
// pointer into the function through the http.Request context with key playerKey{}.
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

		p.InGame = true
		gs.WaitingPlayers[p] = struct{}{}
	})
}

// Handles a leave request from a player. Removes the player from the gs.WaitingPlayers
// set. Leave requests can only be sent by players who have been registered. Thus, 
// this handler should be wrapped with a validate() call. Validate will pass the player 
// pointer into the function through the http.Request context with key playerKey{}.
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

		if p.InGame == false {
			// Maybe should error here?
			return
		}

		p.InGame = false
		delete(gs.WaitingPlayers, p)
	})
}

// TODO: 
// test :(
// Set up reseponses from all handlers
// check request types are correct for each request
