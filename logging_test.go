package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSetupLoggerDefaultsToJSONStdoutOnly(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_LOGS_ENDPOINT", "")

	var output bytes.Buffer
	logger, shutdown, otlpEnabled, err := setupLogger(context.Background(), &output)
	if err != nil {
		t.Fatal(err)
	}
	if otlpEnabled {
		t.Fatal("OTLP logging enabled without an endpoint")
	}
	logger.Info("test message", "answer", 42)
	if err := shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("decode log record: %v; output = %q", err, output.String())
	}
	if record["msg"] != "test message" || record["answer"] != float64(42) {
		t.Fatalf("log record = %#v, want message and structured attribute", record)
	}
}

func TestSetupLoggerExportsOTLP(t *testing.T) {
	requests := make(chan struct {
		path        string
		contentType string
		bodySize    int
	}, 1)
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := new(bytes.Buffer)
		_, _ = body.ReadFrom(r.Body)
		requests <- struct {
			path        string
			contentType string
			bodySize    int
		}{r.URL.Path, r.Header.Get("Content-Type"), body.Len()}
		w.WriteHeader(http.StatusOK)
	}))
	defer receiver.Close()

	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_LOGS_ENDPOINT", receiver.URL+"/v1/logs")
	t.Setenv("OTEL_EXPORTER_OTLP_LOGS_INSECURE", "true")
	t.Setenv("OTEL_EXPORTER_OTLP_LOGS_HEADERS", "")
	t.Setenv("OTEL_EXPORTER_OTLP_LOGS_CERTIFICATE", "")

	var output bytes.Buffer
	logger, shutdown, otlpEnabled, err := setupLogger(context.Background(), &output)
	if err != nil {
		t.Fatal(err)
	}
	if !otlpEnabled {
		t.Fatal("OTLP logging is disabled with a logs endpoint configured")
	}
	logger.InfoContext(context.Background(), "export me", "kind", "smoke-test")
	shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := shutdown(shutdownContext); err != nil {
		t.Fatal(err)
	}

	select {
	case request := <-requests:
		if request.path != "/v1/logs" {
			t.Errorf("OTLP request path = %q, want /v1/logs", request.path)
		}
		if request.contentType != "application/x-protobuf" {
			t.Errorf("OTLP Content-Type = %q, want application/x-protobuf", request.contentType)
		}
		if request.bodySize == 0 {
			t.Error("OTLP request body is empty")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for OTLP log export")
	}
}

func TestRequestLogging(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /widgets/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	})
	handler := withRequestLogging(mux, logger)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/widgets/7", nil))

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("decode request log: %v; output = %q", err, output.String())
	}
	want := map[string]any{
		"msg":                       "http request completed",
		"http.request.method":       http.MethodGet,
		"url.path":                  "/widgets/7",
		"http.route":                "GET /widgets/{id}",
		"http.response.status_code": float64(http.StatusCreated),
		"http.response.body.size":   float64(2),
	}
	for key, value := range want {
		if record[key] != value {
			t.Errorf("log attribute %q = %#v, want %#v", key, record[key], value)
		}
	}
	if _, ok := record["duration"]; !ok {
		t.Error("request log is missing duration")
	}
}
