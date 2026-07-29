package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	os.Exit(run())
}

func run() (exitCode int) {
	logger, shutdownLogging, otlpEnabled, err := setupLogger(context.Background(), os.Stdout)
	if err != nil {
		slog.New(slog.NewJSONHandler(os.Stderr, nil)).Error("initialize logging", "error", err)
		return 1
	}
	slog.SetDefault(logger)
	defer func() {
		shutdownContext, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
		defer cancel()
		if err := shutdownLogging(shutdownContext); err != nil {
			logger.Error("flush OpenTelemetry logs", "error", err)
			exitCode = 1
		}
	}()

	config, err := loadServerConfig()
	if err != nil {
		logger.Error("load server configuration", "error", err)
		return 1
	}
	health := NewHealth()
	handler := withRequestLogging(NewAppWithHealth(NewTaskStore(), health), logger)

	server := &http.Server{
		Addr:              config.address,
		Handler:           handler,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
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
		logger.Info("shutdown signal received", "shutdown_timeout", config.shutdownTimeout)

		shutdownContext, cancel := context.WithTimeout(context.Background(), config.shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
		}
	}()

	logger.Info("todo app listening", "address", config.address, "shutdown_timeout", config.shutdownTimeout, "otel_logs", otlpEnabled)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("HTTP server failed", "error", err)
		return 1
	}
	if shutdownSignal.Err() != nil {
		<-shutdownComplete
	}
	logger.Info("todo app stopped")
	return 0
}
