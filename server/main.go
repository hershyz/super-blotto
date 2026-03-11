package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.Handle("/register", handleRegister())

	http.Handle("/state", validate((*GameState).handleGetState))
	http.Handle("/move", validate((*GameState).handleMove))
	http.Handle("/join", validate((*GameState).handleJoin))
	http.Handle("/leave", validate((*GameState).handleLeave))

	http.Handle("/start", adminOnly((*GameState).handleStart))
	http.Handle("/lobby", adminOnly((*GameState).handleLobby))
	http.Handle("/reset", adminOnly((*GameState).handleReset))

	addr := ":8080"
	fmt.Printf("Server listening on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
