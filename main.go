package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	db, err := InitDB("contacts.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	h := NewHandlers(db)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	addr := ":8080"
	fmt.Println("  ☎  You Never Call")
	fmt.Println("  Server running at http://localhost" + addr)
	fmt.Println()
	log.Fatal(http.ListenAndServe(addr, mux))
}
