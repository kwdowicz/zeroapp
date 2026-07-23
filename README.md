# Todo list

A small Go web application and REST API for tasks with a title and one of three statuses: `new`, `doing`, or `done`. Data is kept in memory and resets when the server restarts.

## Run

```powershell
go run .
```

Open <http://localhost:8080>. Set `ADDR` to use a different address, for example `$env:ADDR = ":3000"`. `SHUTDOWN_TIMEOUT` controls how long the server waits for active requests after an interrupt or termination signal and defaults to `10s`.

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
