// to run: cd server && go run .

package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.Handle("/register", handleRegister())

	http.Handle("/state", validate(gs.handleState()))
	http.Handle("/move", validate(gs.handleMove()))
	http.Handle("/join", validate(gs.handleJoin()))
	http.Handle("/leave", validate(gs.handleLeave()))

	http.Handle("/start", adminOnly(gs.handleStart()))
	http.Handle("/lobby", adminOnly(gs.handleLobby()))

	addr := ":8080"
	fmt.Printf("Server listening on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
