package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/register", handleRegister)

	addr := ":8080"
	fmt.Printf("Server listening on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
