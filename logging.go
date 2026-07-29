package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

const instrumentationName = "zeroapp"

// setupLogger always writes structured logs to output. When an OTLP endpoint
// is configured, it also batches and exports the same records over OTLP/HTTP.
func setupLogger(ctx context.Context, output io.Writer) (*slog.Logger, func(context.Context) error, bool, error) {
	stdoutHandler := slog.NewJSONHandler(output, nil)
	if !otlpEndpointConfigured() {
		return slog.New(stdoutHandler), func(context.Context) error { return nil }, false, nil
	}

	exporter, err := otlploghttp.New(ctx)
	if err != nil {
		return nil, nil, false, err
	}

	serviceName := strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME"))
	if serviceName == "" {
		serviceName = instrumentationName
	}
	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceName(serviceName)),
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithTelemetrySDK(),
	)
	if err != nil {
		_ = exporter.Shutdown(ctx)
		return nil, nil, false, err
	}

	provider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
		sdklog.WithResource(res),
	)
	otelHandler := otelslog.NewHandler(instrumentationName, otelslog.WithLoggerProvider(provider))
	return slog.New(multiHandler{stdoutHandler, otelHandler}), provider.Shutdown, true, nil
}

func otlpEndpointConfigured() bool {
	return strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_LOGS_ENDPOINT")) != "" ||
		strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")) != ""
}

type multiHandler []slog.Handler

func (h multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h multiHandler) Handle(ctx context.Context, record slog.Record) error {
	for _, handler := range h {
		if handler.Enabled(ctx, record.Level) {
			if err := handler.Handle(ctx, record.Clone()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make(multiHandler, len(h))
	for i, handler := range h {
		handlers[i] = handler.WithAttrs(attrs)
	}
	return handlers
}

func (h multiHandler) WithGroup(name string) slog.Handler {
	handlers := make(multiHandler, len(h))
	for i, handler := range h {
		handlers[i] = handler.WithGroup(name)
	}
	return handlers
}

func withRequestLogging(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		response := &loggingResponseWriter{ResponseWriter: w}
		next.ServeHTTP(response, r)

		status := response.status
		if status == 0 {
			status = http.StatusOK
		}
		level := slog.LevelInfo
		if status >= http.StatusInternalServerError {
			level = slog.LevelError
		} else if status >= http.StatusBadRequest {
			level = slog.LevelWarn
		}

		attrs := []slog.Attr{
			slog.String("http.request.method", r.Method),
			slog.String("url.path", r.URL.Path),
			slog.Int("http.response.status_code", status),
			slog.Int64("http.response.body.size", response.bytes),
			slog.Duration("duration", time.Since(started)),
		}
		if r.Pattern != "" {
			attrs = append(attrs, slog.String("http.route", r.Pattern))
		}
		logger.LogAttrs(r.Context(), level, "http request completed", attrs...)
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (w *loggingResponseWriter) WriteHeader(status int) {
	if status >= 200 && w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *loggingResponseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(data)
	w.bytes += int64(n)
	return n, err
}

func (w *loggingResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
