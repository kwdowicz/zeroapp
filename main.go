package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const defaultShutdownTimeout = 10 * time.Second

type serverConfig struct {
	address         string
	shutdownTimeout time.Duration
}

func loadServerConfig() (serverConfig, error) {
	config := serverConfig{
		address:         os.Getenv("ADDR"),
		shutdownTimeout: defaultShutdownTimeout,
	}
	if config.address == "" {
		config.address = ":8080"
	}

	if value := os.Getenv("SHUTDOWN_TIMEOUT"); value != "" {
		timeout, err := time.ParseDuration(value)
		if err != nil {
			return serverConfig{}, fmt.Errorf("parse SHUTDOWN_TIMEOUT: %w", err)
		}
		if timeout <= 0 {
			return serverConfig{}, errors.New("SHUTDOWN_TIMEOUT must be greater than zero")
		}
		config.shutdownTimeout = timeout
	}
	return config, nil
}

func main() {
	config, err := loadServerConfig()
	if err != nil {
		log.Fatal(err)
	}
	health := NewHealth()

	server := &http.Server{
		Addr:              config.address,
		Handler:           NewAppWithHealth(NewTaskStore(), health),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	shutdownSignal, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	shutdownComplete := make(chan struct{})
	go func() {
		defer close(shutdownComplete)
		<-shutdownSignal.Done()
		health.SetReady(false)
		log.Printf("shutdown signal received; allowing up to %s for active requests", config.shutdownTimeout)

		shutdownContext, cancel := context.WithTimeout(context.Background(), config.shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			log.Printf("graceful shutdown: %v", err)
		}
	}()

	log.Printf("todo app listening on http://localhost%s (shutdown timeout %s)", config.address, config.shutdownTimeout)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
	if shutdownSignal.Err() != nil {
		<-shutdownComplete
	}
	log.Printf("todo app stopped")
}
