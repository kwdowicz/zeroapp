# Todo list

A small Go web application and REST API for tasks with a title and one of three statuses: `new`, `doing`, or `done`. Data is kept in memory and resets when the server restarts.

## Run

```powershell
go run .
```

Open <http://localhost:8080>. Set `ADDR` to use a different address, for example `$env:ADDR = ":3000"`. `SHUTDOWN_TIMEOUT` controls how long the server waits for active requests after an interrupt or termination signal and defaults to `10s`.

## Logging

The app writes structured JSON logs to stdout. Each completed HTTP request logs its method, route, path, status, response size, and duration; task titles and request bodies are not logged.

Set an OTLP/HTTP endpoint to export the same records to an OpenTelemetry Collector in batches:

```powershell
$env:OTEL_SERVICE_NAME = "zeroapp"
$env:OTEL_EXPORTER_OTLP_ENDPOINT = "http://localhost:4318"
go run .
```

Use `OTEL_EXPORTER_OTLP_LOGS_ENDPOINT` when logs have a signal-specific URL such as `http://localhost:4318/v1/logs`. The standard `OTEL_EXPORTER_OTLP_HEADERS`, `OTEL_EXPORTER_OTLP_LOGS_HEADERS`, timeout, compression, TLS, and certificate environment variables are handled by the OpenTelemetry exporter. Without either endpoint variable, the app remains stdout-only and does not try to contact a collector. Pending OTLP logs are flushed during shutdown.

## REST API

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/tasks` | List tasks |
| `POST` | `/api/tasks` | Create a task; `status` is optional and defaults to `new` |
| `GET` | `/api/tasks/{id}` | Get one task |
| `PUT` | `/api/tasks/{id}` | Replace a task's title and status |
| `DELETE` | `/api/tasks/{id}` | Delete a task |
| `GET` | `/health/live` | Report that the HTTP process is alive |
| `GET` | `/health/ready` | Report whether the process should receive traffic |

Create a task:

```powershell
Invoke-RestMethod -Method Post -Uri http://localhost:8080/api/tasks `
  -ContentType application/json -Body '{"title":"Write documentation"}'
```

Update it:

```powershell
Invoke-RestMethod -Method Put -Uri http://localhost:8080/api/tasks/1 `
  -ContentType application/json -Body '{"title":"Write documentation","status":"done"}'
```

## Test

```powershell
go test -race ./...
```

## Container

Build the small, non-root Linux image:

```powershell
docker build -t zeroapp:latest .
```

Run it locally:

```powershell
docker run --rm -p 8080:8080 --name zeroapp zeroapp:latest
```

The image exposes port `8080`, listens on all container interfaces, and is suitable for Docker or Kubernetes. Kubernetes probes should use `GET /health/live` and `GET /health/ready` on port `8080`. On SIGTERM, the server marks itself unready and gracefully drains active requests before exiting.

Task storage is in memory. Container restarts erase all tasks, and multiple replicas do not share data. Add persistent storage before using this application for durable or multi-replica workloads.
