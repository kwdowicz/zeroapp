package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

//go:embed static/index.html
var staticFiles embed.FS

type App struct {
	store *TaskStore
}

func NewApp(store *TaskStore) http.Handler {
	app := &App{store: store}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/tasks", app.listTasks)
	mux.HandleFunc("POST /api/tasks", app.createTask)
	mux.HandleFunc("GET /api/tasks/{id}", app.getTask)
	mux.HandleFunc("PUT /api/tasks/{id}", app.updateTask)
	mux.HandleFunc("DELETE /api/tasks/{id}", app.deleteTask)
	mux.HandleFunc("GET /", app.index)
	return mux
}

func (a *App) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	page, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "failed to load application", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(page)
}

func (a *App) listTasks(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, a.store.List())
}

func (a *App) getTask(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	task, err := a.store.Get(id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}

type taskInput struct {
	Title  string `json:"title"`
	Status Status `json:"status"`
}

func (a *App) createTask(w http.ResponseWriter, r *http.Request) {
	var input taskInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if input.Status == "" {
		input.Status = StatusNew
	}
	task, err := a.store.Create(input.Title, input.Status)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	w.Header().Set("Location", fmt.Sprintf("/api/tasks/%d", task.ID))
	writeJSON(w, http.StatusCreated, task)
}

func (a *App) updateTask(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	var input taskInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	task, err := a.store.Update(id, input.Title, input.Status)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (a *App) deleteTask(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	if err := a.store.Delete(id); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func taskID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "task id must be a positive integer")
		return 0, false
	}
	return id, true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("invalid JSON: request body must contain one object")
	}
	return nil
}

func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrTaskNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrInvalidTitle), errors.Is(err, ErrInvalidStatus):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
