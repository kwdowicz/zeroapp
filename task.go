package main

import (
	"errors"
	"strings"
	"sync"
)

type Status string

const (
	StatusNew   Status = "new"
	StatusDoing Status = "doing"
	StatusDone  Status = "done"
)

var (
	ErrInvalidTitle  = errors.New("title is required")
	ErrInvalidStatus = errors.New("status must be new, doing, or done")
	ErrTaskNotFound  = errors.New("task not found")
)

type Task struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	Status Status `json:"status"`
}

func (s Status) Valid() bool {
	return s == StatusNew || s == StatusDoing || s == StatusDone
}

type TaskStore struct {
	mu     sync.RWMutex
	nextID int64
	tasks  map[int64]Task
}

func NewTaskStore() *TaskStore {
	return &TaskStore{nextID: 1, tasks: make(map[int64]Task)}
}

func (s *TaskStore) List() []Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]Task, 0, len(s.tasks))
	for id := int64(1); id < s.nextID; id++ {
		if task, ok := s.tasks[id]; ok {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

func (s *TaskStore) Get(id int64) (Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, ok := s.tasks[id]
	if !ok {
		return Task{}, ErrTaskNotFound
	}
	return task, nil
}

func (s *TaskStore) Create(title string, status Status) (Task, error) {
	title, err := validateTask(title, status)
	if err != nil {
		return Task{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	task := Task{ID: s.nextID, Title: title, Status: status}
	s.tasks[task.ID] = task
	s.nextID++
	return task, nil
}

func (s *TaskStore) Update(id int64, title string, status Status) (Task, error) {
	title, err := validateTask(title, status)
	if err != nil {
		return Task{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.tasks[id]; !ok {
		return Task{}, ErrTaskNotFound
	}
	task := Task{ID: id, Title: title, Status: status}
	s.tasks[id] = task
	return task, nil
}

func (s *TaskStore) Delete(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.tasks[id]; !ok {
		return ErrTaskNotFound
	}
	delete(s.tasks, id)
	return nil
}

func validateTask(title string, status Status) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", ErrInvalidTitle
	}
	if !status.Valid() {
		return "", ErrInvalidStatus
	}
	return title, nil
}
