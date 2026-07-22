package main

import (
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	address := os.Getenv("ADDR")
	if address == "" {
		address = ":8080"
	}

	server := &http.Server{
		Addr:              address,
		Handler:           NewApp(NewTaskStore()),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("todo app listening on http://localhost%s", address)
	log.Fatal(server.ListenAndServe())
}
