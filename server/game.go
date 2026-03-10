package main

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"slices"
	"sync"
	"time"
)

// All in-memory game state. mu is used to protect accesses to everything
type GameState struct {
	mu         sync.RWMutex
	NextGameID int


	Phase          GamePhase
	WaitingPlayers []*Player
	// PlayerZeros are used as identifiers for games; 
	// chosen lexicographically to ensure 0 -> 1 locking order.
	PlayerZeros    []*Player
	Leaderboard    []*Player

	// Round metadata
	Round        int
	RoundEndTime time.Time

	Players       map[string]*Player // Token -> *Player
	UsedUsernames map[string]struct{}
}

func NewGameState() *GameState {
	return &GameState{
		Players: map[string]*Player{
			AdminToken: {
				IsAdmin:  true,
				Username: AdminUsername,
				Token:    AdminToken,
			},
		},
		WaitingPlayers: make([]*Player, 0),
		PlayerZeros:    make([]*Player, 0),
		Leaderboard:    make([]*Player, 0),

		UsedUsernames: map[string]struct{}{AdminUsername: {}},
	}
}

var gs = NewGameState()

// Registers a player into the backend player store. The players username must
// be unique. The players are not automatically added to the game, they must
// post a join request to be added to the gs.WaitingPlayers set
func (gs *GameState) registerPlayer(username, token string) error {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if _, exists := gs.UsedUsernames[username]; exists {
		return ErrUsernameTaken
	}

	p := &Player{
		Token:    token,
		Username: username,

		CommandPoints: InitialCommandPoints,
	}

	gs.Players[token] = p
	gs.Leaderboard = append(gs.Leaderboard, p)

	gs.UsedUsernames[username] = struct{}{}

	return nil
}

func (gs *GameState) startRound() time.Time {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	gs.Phase = Playing
	gs.Round++
	gs.RoundEndTime = time.Now().Add(time.Second * TimePerRound)

	return gs.RoundEndTime
}

func (gs *GameState) endRound() {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	gs.Phase = Resolving
	var wg sync.WaitGroup

	for _, p := range gs.PlayerZeros {
		wg.Add(1)
		go func(p *Player) {
			defer wg.Done()
			p.endRound()
		}(p)
	}

	wg.Wait()
}

func (gs *GameState) endGame() {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	gs.Phase = Finished
	var wg sync.WaitGroup

	for _, p := range gs.PlayerZeros {
		wg.Add(1)
		go func(p *Player) {
			defer wg.Done()
			p.endGame()
		}(p)
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
	type LobbyRequest struct{}
	type LobbyResponse struct{}

	validateLobby := func() error {
		// Can only go to lobby when the game is finished
		if gs.Phase == Lobby {
			return ErrInLobby
		}
		if gs.Phase == Playing || gs.Phase == Resolving {
			return ErrGameInProgress
		}

		return nil
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := decode[LobbyRequest](r, http.MethodPost); err != nil {
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

			if gs.Phase != Finished {
				panic(fmt.Sprint("impossible"))
			} // for local reasoning purposes only since validateLobby checks this is true
			if len(gs.WaitingPlayers) != 0 {
				panic(fmt.Sprint("gs.WaitingPlayers should be empty in the finished state"))
			}

			// Reinitialize game state
			gs.Phase = Lobby
			clear(gs.PlayerZeros)
			gs.Round = 0

			// Reinitialize player state
			for _, p := range gs.Players {
				p.gotoLobby()
			}

			gs.mu.Unlock()
		}

		encode(w, http.StatusOK, LobbyResponse{})
	})
}

// Starts a game. The game phase must be in Lobby and must have and even number
// of participants to start a game. This handler can only be called by an admin,
// so it should be wrapped by an adminOnly() call.
func (gs *GameState) handleStart() http.Handler {
	type StartRequest struct{}
	type StartResponse struct{}

	validateStart := func() error {
		if gs.Phase != Lobby {
			return ErrGameInProgress
		}
		if len(gs.WaitingPlayers) == 0 {
			return ErrNoParticipants
		}
		if len(gs.WaitingPlayers)%2 != 0 {
			return ErrOddParticipants
		}
		return nil
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := decode[StartRequest](r, http.MethodPost); err != nil {
			encodeError(w, err)
			return
		}

		// Setup work for games to start. Games don't really start until the timer starts in gs.startRound.
		// It is important that gs.Phase is not changes to Playing in this block so that move requests recieved
		// between the end of this block and the gs.startRound call are rejected.
		{
			gs.mu.Lock()

			if err := validateStart(); err != nil {
				gs.mu.Unlock()
				encodeError(w, err)
				return
			}

			if gs.Round != 0 {
				panic(fmt.Sprint("Game round not initialized correctly"))
			}
			if len(gs.PlayerZeros) != 0 {
				panic(fmt.Sprint("gs.PlayerZeros not initialized correctly in the lobby phase"))
			}

			// Shuffle waiting players
			rand.Shuffle(len(gs.WaitingPlayers), func(i, j int) {
				gs.WaitingPlayers[i], gs.WaitingPlayers[j] = gs.WaitingPlayers[j], gs.WaitingPlayers[i]
			})

			for len(gs.WaitingPlayers) >= 2 {
				gs.NextGameID++

				p0 := gs.WaitingPlayers[0]
				p1 := gs.WaitingPlayers[1]
				gs.WaitingPlayers = gs.WaitingPlayers[2:]

				// Assign p0 to the player with the lexicographically smaller username
				if p0.getUsername() > p1.getUsername() {
					p0, p1 = p1, p0
				}

				// Use p0 as the identifiers for games
				gs.PlayerZeros = append(gs.PlayerZeros, p0)

				p0.start(p1)
				p1.start(p0)
			}

			if len(gs.WaitingPlayers) != 0 {
				panic(fmt.Sprint("gs.WaitingPlayers is somehow not empty after pairing up players"))
			}

			gs.mu.Unlock()
		} // unlock gs.mu so setting up timers doesn't hold the lock

		// set up timer
		go func(gs *GameState) {
			for i := 1; i <= TotalRounds; i++ {
				endtime := gs.startRound()
				time.Sleep(time.Until(endtime))
				gs.endRound()
				// TODO: Maybe wait for 5 seconds before starting a new round?
			}
			gs.endGame()
		}(gs)

		encode(w, http.StatusAccepted, StartResponse{})
	})
}

// Handles a move from a player. Player moves include the round (so movees from
// previous rounds don't affect the current round and are dropped), row, col, and
// CommandPoints. Moves can only be made by registered players that are in the game.
// Thus, this handler should be wrapped with a validate() call. Validate will pass
// the player pointer into the function through the http.Request context with key
// playerKey{}.
func (gs *GameState) handleMove() http.Handler {
	type MoveRequest struct {
		Round         int `json:"round"`
		Row           int `json:"row"`
		Col           int `json:"col"`
		CommandPoints int `json:"commandPoints"`
	}
	type MoveResponse struct{}

	validateMove := func(req MoveRequest, p *Player) error {
		if gs.Phase != Playing {
			return ErrRoundEnded
		}
		if gs.Round != req.Round {
			return ErrIncorrectRound
		}
		if req.CommandPoints < 0 {
			return ErrNegativeCommandPoints
		}
		return nil
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, err := decode[MoveRequest](r, http.MethodPost)

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
		if !ok {
			panic(fmt.Sprintf("gs.playerFromContext failed in move request"))
		}

		{
			// To make moves more concurrent, only get the gs reader lock. 
			// This is fine becuase we are not changing any gs specific variables.
			gs.mu.RLock() 

			if err := validateMove(req, p); err != nil {
				gs.mu.RUnlock()
				encodeError(w, err)
				return
			}

			if err := p.move(row, col, req.CommandPoints); err != nil {
				gs.mu.RUnlock()
				encodeError(w, err)
				return
			}

			gs.mu.RUnlock()
		}

		encode(w, http.StatusOK, MoveResponse{})
	})
}

// Handles a join from a player. Players that are registered are not automatically
// added to the game. They need to send a post request to the /join path to be added.
// Join requests can only be sent by players who have been registered. Thus, this
// handler should be wrapped with a validate() call. Validate will pass the player
// pointer into the function through the http.Request context with key playerKey{}.
func (gs *GameState) handleJoin() http.Handler {
	type JoinRequest struct{}
	type JoinResponse struct{}

	validateLobby := func() error {
		if gs.Phase != Lobby {
			return ErrGameInProgress
		}

		return nil
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := decode[JoinRequest](r, http.MethodPost); err != nil {
			encodeError(w, err)
			return
		}

		p, ok := playerFromContext(r.Context())
		if !ok {
			panic(fmt.Sprintf("gs.playerFromContext failed"))
		}

		{
			gs.mu.Lock()

			if err := validateLobby(); err != nil {
				gs.mu.Unlock()
				encodeError(w, err)
				return
			}

			if err := p.join(); err != nil {
				gs.mu.Unlock()
				encodeError(w, err)
				return
			}
			gs.WaitingPlayers = append(gs.WaitingPlayers, p)
			gs.mu.Unlock()
		}

		encode(w, http.StatusOK, JoinResponse{})
	})
}

// Handles a leave request from a player. Removes the player from the gs.WaitingPlayers
// set. Leave requests can only be sent by players who have been registered. Thus,
// this handler should be wrapped with a validate() call. Validate will pass the player
// pointer into the function through the http.Request context with key playerKey{}.
func (gs *GameState) handleLeave() http.Handler {
	type LeaveRequest struct{}
	type LeaveResponse struct{}

	validateLeave := func() error {
		if gs.Phase != Lobby {
			return ErrGameInProgress
		}
		// TODO: maybe error if p.Participating == false

		return nil
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := decode[LeaveRequest](r, http.MethodPost); err != nil {
			encodeError(w, err)
			return
		}

		p, ok := playerFromContext(r.Context())
		if !ok {
			panic(fmt.Sprintf("gs.playerFromContext failed"))
		}

		{
			gs.mu.Lock()

			if err := validateLeave(); err != nil {
				gs.mu.Unlock()
				encodeError(w, err)
				return
			}

			if err := p.leave(); err != nil {
				gs.mu.Unlock()
				encodeError(w, err)
				return
			}

			// O(n) deletion. I wanted to use a slice for waiting players so that if we 
			// display the waiting players on the client side, the order will be consistent
			gs.WaitingPlayers = slices.DeleteFunc(gs.WaitingPlayers, func(wp *Player) bool {
				return wp == p
			})
			gs.mu.Unlock()
		}

		encode(w, http.StatusOK, LeaveResponse{})
	})
}

func (gs *GameState) handleGetState() http.Handler {
	type LobbyData struct {
		WaitingPlayers []string     `json:"waitingPlayers"`
		Leaderboard    []PlayerData `json:"leaderboard"`
		Me             PlayerData   `json:"me"`
	}

	type GameData struct {
		Round        int        `json:"round"`
		RoundEndTime time.Time  `json:"roundEndTime"`
		Opponent     PlayerData `json:"opponent"`
		Me           PlayerData `json:"me"`
	}

	type GetStateRequest struct{}
	type GetStateResponse struct {
		Phase GamePhase `json:"phase"`
		LobbyData LobbyData `json:"lobbyData,omitempty"` // Only send if Phase == Lobby
		GameData GameData `json:"gameData,omitempty"` // Only send if Phase != Lobby
	}

	validateGetState := func() error {
		return nil
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := decode[GetStateRequest](r, http.MethodGet); err != nil {
			encodeError(w, err)
			return
		}

		var response GetStateResponse
		p, ok := playerFromContext(r.Context())
		if !ok {
			panic(fmt.Sprintf("gs.playerFromContext failed"))
		}

		{
			gs.mu.RLock()

			if err := validateGetState(); err != nil {
				gs.mu.RUnlock()
				encodeError(w, err)
				return
			}

			response.Phase = gs.Phase

			if gs.Phase == Lobby {
				waitingUsernames := make([]string, 0, len(gs.WaitingPlayers))
				for _, wp := range gs.WaitingPlayers {
					waitingUsernames = append(waitingUsernames, wp.getUsername())
				}

				leaderboardData := make([]PlayerData, 0, len(gs.Leaderboard))
				for _, lp := range gs.Leaderboard {
					leaderboardData = append(leaderboardData, lp.getLeaderboardData())
				}

				response.LobbyData = LobbyData{
					WaitingPlayers: waitingUsernames,
					Leaderboard:    leaderboardData,
					Me:             p.getLeaderboardData(),
				}
			} else {
				// I think there are some concurrency issues with this. Since we only have a reader lock on gs,
				// and we don't hold both me and op mutex at the same time, a move from either me or op could
				// come in between our .getGameData requests. I don't think this is an issue because we don't
				// see opponent moves until after the round ends anyways.
				response.GameData = GameData{
					Round:        gs.Round,
					RoundEndTime: gs.RoundEndTime,
					Opponent:     p.getOpponent().getGameData(true),
					Me:           p.getGameData(false),
				}
			}

			gs.mu.RUnlock()
		}

		encode(w, http.StatusOK, response)
	})
}

// TODO:
// test :(
// update leaderboard.
// other handlers needed:
//		GET /leaderboard
// 		POST /reset (maybe)
// the getState handler might be too inefficient with all the locking. Might have to think of a different way to get the state of the game, or use caching/versioning to reduce the computation.
