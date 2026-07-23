package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTaskAPIWorkflow(t *testing.T) {
	t.Parallel()
	handler := NewApp(NewTaskStore())

	response := serve(handler, http.MethodGet, "/api/tasks", "")
	assertStatus(t, response, http.StatusOK)
	var tasks []Task
	decodeResponse(t, response, &tasks)
	if tasks == nil || len(tasks) != 0 {
		t.Fatalf("initial task list = %#v, want non-nil empty list", tasks)
	}

	response = serve(handler, http.MethodPost, "/api/tasks", `{"title":"First task"}`)
	assertStatus(t, response, http.StatusCreated)
	if got := response.Header().Get("Location"); got != "/api/tasks/1" {
		t.Fatalf("Location = %q, want /api/tasks/1", got)
	}
	var created Task
	decodeResponse(t, response, &created)
	if want := (Task{ID: 1, Title: "First task", Status: StatusNew}); created != want {
		t.Fatalf("created task = %#v, want %#v", created, want)
	}

	response = serve(handler, http.MethodGet, "/api/tasks/1", "")
	assertStatus(t, response, http.StatusOK)
	var fetched Task
	decodeResponse(t, response, &fetched)
	if fetched != created {
		t.Fatalf("fetched task = %#v, want %#v", fetched, created)
	}

	response = serve(handler, http.MethodPut, "/api/tasks/1", `{"title":"First task updated","status":"doing"}`)
	assertStatus(t, response, http.StatusOK)
	var updated Task
	decodeResponse(t, response, &updated)
	if want := (Task{ID: 1, Title: "First task updated", Status: StatusDoing}); updated != want {
		t.Fatalf("updated task = %#v, want %#v", updated, want)
	}

	response = serve(handler, http.MethodGet, "/api/tasks", "")
	assertStatus(t, response, http.StatusOK)
	decodeResponse(t, response, &tasks)
	if len(tasks) != 1 || tasks[0] != updated {
		t.Fatalf("task list = %#v, want updated task", tasks)
	}

	response = serve(handler, http.MethodDelete, "/api/tasks/1", "")
	assertStatus(t, response, http.StatusNoContent)
	if response.Body.Len() != 0 {
		t.Fatalf("DELETE response body = %q, want empty", response.Body.String())
	}

	response = serve(handler, http.MethodGet, "/api/tasks/1", "")
	assertError(t, response, http.StatusNotFound, "task not found")
}

func TestCreateTaskValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		body    string
		status  int
		message string
	}{
		{"empty body", "", http.StatusBadRequest, "invalid JSON"},
		{"malformed JSON", `{"title":`, http.StatusBadRequest, "invalid JSON"},
		{"unknown field", `{"title":"task","extra":true}`, http.StatusBadRequest, "unknown field"},
		{"multiple objects", `{"title":"one"} {"title":"two"}`, http.StatusBadRequest, "one object"},
		{"missing title", `{}`, http.StatusBadRequest, "title is required"},
		{"blank title", `{"title":"  "}`, http.StatusBadRequest, "title is required"},
		{"invalid status", `{"title":"task","status":"pending"}`, http.StatusBadRequest, "status must be new, doing, or done"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			response := serve(NewApp(NewTaskStore()), http.MethodPost, "/api/tasks", test.body)
			assertError(t, response, test.status, test.message)
		})
	}
}

func TestUpdateTaskValidation(t *testing.T) {
	t.Parallel()
	handler := NewApp(NewTaskStore())
	serve(handler, http.MethodPost, "/api/tasks", `{"title":"task"}`)

	tests := []struct {
		name    string
		path    string
		body    string
		status  int
		message string
	}{
		{"invalid id text", "/api/tasks/nope", `{"title":"x","status":"done"}`, http.StatusBadRequest, "positive integer"},
		{"invalid id zero", "/api/tasks/0", `{"title":"x","status":"done"}`, http.StatusBadRequest, "positive integer"},
		{"missing task", "/api/tasks/99", `{"title":"x","status":"done"}`, http.StatusNotFound, "task not found"},
		{"missing status", "/api/tasks/1", `{"title":"x"}`, http.StatusBadRequest, "status must be new, doing, or done"},
		{"missing title", "/api/tasks/1", `{"status":"done"}`, http.StatusBadRequest, "title is required"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := serve(handler, http.MethodPut, test.path, test.body)
			assertError(t, response, test.status, test.message)
		})
	}
}

func TestDeleteTaskErrors(t *testing.T) {
	t.Parallel()
	handler := NewApp(NewTaskStore())
	assertError(t, serve(handler, http.MethodDelete, "/api/tasks/-1", ""), http.StatusBadRequest, "positive integer")
	assertError(t, serve(handler, http.MethodDelete, "/api/tasks/1", ""), http.StatusNotFound, "task not found")
}

func TestRoutesAndWebPage(t *testing.T) {
	t.Parallel()
	handler := NewApp(NewTaskStore())

	response := serve(handler, http.MethodGet, "/", "")
	assertStatus(t, response, http.StatusOK)
	if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/html") {
		t.Fatalf("index Content-Type = %q, want text/html", contentType)
	}
	if body := response.Body.String(); !strings.Contains(body, "Todo list") || !strings.Contains(body, "/api/tasks") {
		t.Fatalf("index does not contain expected application content")
	}

	assertStatus(t, serve(handler, http.MethodGet, "/missing", ""), http.StatusNotFound)
	assertStatus(t, serve(handler, http.MethodPatch, "/api/tasks/1", `{}`), http.StatusMethodNotAllowed)
}

func TestJSONResponsesHaveContentType(t *testing.T) {
	t.Parallel()
	response := serve(NewApp(NewTaskStore()), http.MethodGet, "/api/tasks", "")
	if got := response.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want application/json; charset=utf-8", got)
	}
}

func TestHealthEndpoints(t *testing.T) {
	t.Parallel()
	health := NewHealth()
	handler := NewAppWithHealth(NewTaskStore(), health)

	assertStatus(t, serve(handler, http.MethodGet, "/health/live", ""), http.StatusNoContent)
	assertStatus(t, serve(handler, http.MethodGet, "/health/ready", ""), http.StatusNoContent)

	health.SetReady(false)
	assertStatus(t, serve(handler, http.MethodGet, "/health/ready", ""), http.StatusServiceUnavailable)
	assertStatus(t, serve(handler, http.MethodGet, "/health/live", ""), http.StatusNoContent)
}

func serve(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func assertStatus(t *testing.T, response *httptest.ResponseRecorder, want int) {
	t.Helper()
	if response.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, want, response.Body.String())
	}
}

func assertError(t *testing.T, response *httptest.ResponseRecorder, status int, messagePart string) {
	t.Helper()
	assertStatus(t, response, status)
	var body struct {
		Error string `json:"error"`
	}
	decodeResponse(t, response, &body)
	if !strings.Contains(body.Error, messagePart) {
		t.Fatalf("error = %q, want it to contain %q", body.Error, messagePart)
	}
}

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if err := json.NewDecoder(response.Body).Decode(dst); err != nil && err != io.EOF {
		t.Fatalf("decode response: %v", err)
	}
}
