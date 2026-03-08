package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.Handle("/register", handleRegister())
	http.Handle("/move", validate(handleMove()))
	http.Handle("/start", adminOnly(handleStart()))

	addr := ":8080"
	fmt.Printf("Server listening on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
