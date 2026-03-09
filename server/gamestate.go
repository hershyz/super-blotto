package main

import (
	"context"
	"math/rand"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Player struct {
	IsAdmin   		bool	
	Token 				string
	Username 			string

	Wins        	int
	Losses        int
	Ties        	int

	Participating bool // True if player has joined the game or is currently playing in the game
	GameID				int
	Role					PlayerRole // Player 0 or 1 
}

// All in-memory game state. mu is used to protect accesses to everything
type GameState struct {
	mu          sync.RWMutex
	NextGameID	int
	
	Players     map[string]*Player // Token -> *Player
	WaitingPlayers map[*Player]struct{}
	Games       map[int]*Game // GameID -> *Game TODO: Could make this a slice, since all games are happening at the same time so reusing gameIDs isn't really a problem
	Leaderboard []*Player

	// Round metadata
	Round 				int
	RoundEndTime	time.Time
	Phase 				GamePhase

	UsedUsernames map[string]struct{}
}

var gs = &GameState{
	Players: map[string]*Player{
			AdminToken: {
				IsAdmin:  true,
				Username: "admin",
				Token:    AdminToken,
			},
		},
	WaitingPlayers: make(map[*Player]struct{}),
	Games: make(map[int]*Game),

	UsedUsernames:  map[string]struct{}{"admin": {}},
}


// Registers a player into the backend player store. The players username must 
// be unique. The players are not automatically added to the game, they must 
// post a join request to be added to the gs.WaitingPlayers set
func (gs *GameState) registerPlayer(username, token string) (error) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

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
		go func(g *Game) { 
			defer wg.Done() 
			g.endRound() 
		}(g)
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
		go func(g *Game) { 
			defer wg.Done() 
			g.endGame() 
		}(g)
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
	type lobbyRequest struct {}
	type lobbyResponse struct {}

	validateLobby := func() (error) {
		// Can only go to lobby when the game is finished
		if (gs.Phase == Lobby)												 		{ return ErrInLobby }
		if (gs.Phase == Playing || gs.Phase == Resolving) { return ErrGameInProgress }

		return nil
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := decode[lobbyRequest](r, http.MethodPost); err != nil {
			encodeError(w, err)
			return
		}
		
		{
			gs.mu.Lock()

			if err := validateLobby(); err != nil {
				gs.mu.Unlock()
				encodeError(w, ErrGameInProgress)
				return
			}

			if gs.Phase != Finished 				{ panic(fmt.Sprint("impossible")) } // for local reasoning purposes only since validateLobby checks this is true
			if len(gs.WaitingPlayers) != 0 	{ panic(fmt.Sprint("gs.WaitingPlayers should be empty in the finished state")) }
			
			// Reinitialize game state
			clear(gs.Games)
			gs.Round = 0
			gs.Phase = Lobby
			
			// Reinitialize player state
			for _, p := range gs.Players {
				p.Participating = false
				p.GameID = NullGameID
				p.Role = None
			}

			gs.mu.Unlock()
		}

		encode(w, http.StatusOK, lobbyResponse{})
	})
}

// Starts a game. The game phase must be in Lobby and must have and even number
// of participants to start a game. This handler can only be called by an admin,
// so it should be wrapped by an adminOnly() call.
func (gs *GameState) handleStart() http.Handler {
	type startRequest struct {}
	type startResponse struct {}

	validateStart := func() error {
			if gs.Phase != Lobby { return ErrGameInProgress }
			if len(gs.WaitingPlayers) == 0 { return ErrNoParticipants }
			if len(gs.WaitingPlayers)%2 != 0 { return ErrOddParticipants }
			return nil
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := decode[startRequest](r, http.MethodPost); err != nil {
			encodeError(w, err)
			return
		}

		{
			gs.mu.Lock()

			if err := validateStart(); err != nil {
				gs.mu.Unlock()
				encodeError(w, err) // unlock before calling encode error so that sending messages and logging doesn't happen while holding a lock
				return
			}

			if (gs.Round != 0) { 
				panic(fmt.Sprint("Game round not initialized correctly"))
			}

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

			gs.mu.Unlock()
		} // unlock gs.mu so setting up timers doesn't hold the lock
		
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
				// TODO: Maybe wait for 5 seconds before starting a new round?
			}
		}(gs)

		encode(w, http.StatusAccepted, startResponse{})
	})
}

// Handles a move from a player. Player moves include the round (so movees from 
// previous rounds don't affect the current round and are dropped), row, col, and 
// CommandPoints. Moves can only be made by registered players that are in the game.
// Thus, this handler should be wrapped with a validate() call. Validate will pass
// the player pointer into the function through the http.Request context with key
// playerKey{}.
func (gs *GameState) handleMove() http.Handler {
	type moveRequest struct {
		Round int `json:"round"`
		Row	int `json:"row"`
		Col int `json:"col"`
		CommandPoints int `json:"commandPoints"`
	}
	type moveResponse struct {}

	validateMove := func(req moveRequest, p *Player) (error) {
		if (p.Participating == false && p.GameID != NullGameID) { panic(fmt.Sprintf("GameID is not NullGameID when player is not in game")) }
		if (p.GameID == NullGameID) 														{ panic(fmt.Sprintf("GameID is NullGameID when player is in game")) }

		if gs.Phase != Playing 			{ return ErrRoundEnded }
		if gs.Round != req.Round 		{ return ErrIncorrectRound }
		if p.Participating == false { return ErrNotInGame }
		if req.CommandPoints < 0 		{ return ErrNegativeCommandPoints }
		return nil
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, err := decode[moveRequest](r, http.MethodPost)

		if err != nil {
			encodeError(w, err)
			return
		} 
		
		row := req.Row
		col := req.Col

		// --- game state agnostic checks ---

		if row < 0 || row >= GridWidth || 
		col < 0 || col >= GridHeight {
			encodeError(w, ErrOutOfBounds)
			return
		}

		// --- game state specific checks ---

		p, ok := playerFromContext(r.Context())
		if !ok { panic(fmt.Sprintf("gs.playerFromContext failed in move request")) }

		{
			gs.mu.Lock()
			
			if err := validateMove(req, p); err != nil {
				gs.mu.Unlock()
				encodeError(w, err)
				return
			}

			g, exists := gs.Games[p.GameID]
			if (exists == false) { panic(fmt.Sprintf("Game does not exist")) }
			
			if err := g.move(row, col, req.CommandPoints, p.Role); err != nil {
				gs.mu.Unlock()
				encodeError(w, err)
				return
			}

			gs.mu.Unlock()
		}

		encode(w, http.StatusOK, moveResponse{})
	})
}

// Handles a join from a player. Players that are registered are not automatically
// added to the game. They need to send a post request to the /join path to be added.
// Join requests can only be sent by players who have been registered. Thus, this 
// handler should be wrapped with a validate() call. Validate will pass the player 
// pointer into the function through the http.Request context with key playerKey{}.
func (gs *GameState) handleJoin() http.Handler {
	type joinRequest struct {}
	type joinResponse struct {}

	validateLobby := func() (error) {
		if gs.Phase != Lobby { return ErrGameInProgress }
		// TODO: maybe should return an error if p.Participating == true

		return nil
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := decode[joinRequest](r, http.MethodPost); err != nil {
			encodeError(w, err)
			return
		}
		
		p, ok := playerFromContext(r.Context())
		if !ok { panic(fmt.Sprintf("gs.playerFromContext failed")) }

		{
			gs.mu.Lock()

			if err := validateLobby(); err != nil {
				gs.mu.Unlock()
				encodeError(w, err)
				return
			}

			p.Participating = true
			gs.WaitingPlayers[p] = struct{}{}

			gs.mu.Unlock()
		}

		encode(w, http.StatusOK, joinResponse{})
	})
}

// Handles a leave request from a player. Removes the player from the gs.WaitingPlayers
// set. Leave requests can only be sent by players who have been registered. Thus, 
// this handler should be wrapped with a validate() call. Validate will pass the player 
// pointer into the function through the http.Request context with key playerKey{}.
func (gs *GameState) handleLeave() http.Handler {
	type leaveRequest struct {}
	type leaveResponse struct {}

	validateLeave := func() (error) {
		if gs.Phase != Lobby { return ErrGameInProgress }
		// TODO: maybe error if p.Participating == false

		return nil
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := decode[leaveRequest](r, http.MethodPost); err != nil {
			encodeError(w, err)
			return
		}
		
		p, ok := playerFromContext(r.Context())
		if !ok { panic(fmt.Sprintf("gs.playerFromContext failed")) }
	
		{
			gs.mu.Lock()

			if err := validateLeave(); err != nil {
				gs.mu.Unlock()
				encodeError(w, err)
				return
			}

			p.Participating = false
			delete(gs.WaitingPlayers, p)

			gs.mu.Unlock()
		}
		
		encode(w, http.StatusOK, leaveResponse{})
	})
}

// TODO: 
// test :(
// update leaderboard.
// other handlers needed: 
// 		GET /state
//		GET /leaderboard 
// 		POST /reset (maybe)

