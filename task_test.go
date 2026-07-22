package main

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestStatusValid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status Status
		want   bool
	}{
		{StatusNew, true},
		{StatusDoing, true},
		{StatusDone, true},
		{"", false},
		{"pending", false},
		{"DONE", false},
	}
	for _, test := range tests {
		if got := test.status.Valid(); got != test.want {
			t.Errorf("Status(%q).Valid() = %v, want %v", test.status, got, test.want)
		}
	}
}

func TestTaskStoreCRUD(t *testing.T) {
	t.Parallel()
	store := NewTaskStore()

	if got := store.List(); len(got) != 0 {
		t.Fatalf("new store List() returned %d tasks, want 0", len(got))
	}

	first, err := store.Create("  write tests  ", StatusNew)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if want := (Task{ID: 1, Title: "write tests", Status: StatusNew}); first != want {
		t.Fatalf("Create() = %#v, want %#v", first, want)
	}

	second, err := store.Create("ship app", StatusDoing)
	if err != nil {
		t.Fatalf("second Create() error = %v", err)
	}
	if second.ID != 2 {
		t.Fatalf("second task ID = %d, want 2", second.ID)
	}

	got, err := store.Get(first.ID)
	if err != nil || got != first {
		t.Fatalf("Get(1) = %#v, %v; want %#v, nil", got, err, first)
	}

	updated, err := store.Update(first.ID, " tests written ", StatusDone)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if want := (Task{ID: 1, Title: "tests written", Status: StatusDone}); updated != want {
		t.Fatalf("Update() = %#v, want %#v", updated, want)
	}

	listed := store.List()
	if len(listed) != 2 || listed[0] != updated || listed[1] != second {
		t.Fatalf("List() = %#v, want tasks in ID order", listed)
	}

	if err := store.Delete(first.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := store.Get(first.ID); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("Get(deleted) error = %v, want ErrTaskNotFound", err)
	}
	if got := store.List(); len(got) != 1 || got[0] != second {
		t.Fatalf("List() after delete = %#v, want second task", got)
	}
}

func TestTaskStoreValidationAndMissingTasks(t *testing.T) {
	t.Parallel()
	store := NewTaskStore()

	tests := []struct {
		name   string
		title  string
		status Status
		want   error
	}{
		{"empty title", "", StatusNew, ErrInvalidTitle},
		{"whitespace title", " \t\n", StatusNew, ErrInvalidTitle},
		{"invalid status", "task", "pending", ErrInvalidStatus},
		{"empty status", "task", "", ErrInvalidStatus},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := store.Create(test.title, test.status); !errors.Is(err, test.want) {
				t.Errorf("Create() error = %v, want %v", err, test.want)
			}
		})
	}

	if _, err := store.Update(99, "task", StatusDone); !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("Update(missing) error = %v, want ErrTaskNotFound", err)
	}
	if err := store.Delete(99); !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("Delete(missing) error = %v, want ErrTaskNotFound", err)
	}
}

func TestTaskStoreConcurrentCreate(t *testing.T) {
	t.Parallel()
	store := NewTaskStore()
	const count = 100

	var wg sync.WaitGroup
	errs := make(chan error, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := store.Create(fmt.Sprintf("task %d", i), StatusNew); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent Create() error = %v", err)
	}

	tasks := store.List()
	if len(tasks) != count {
		t.Fatalf("List() returned %d tasks, want %d", len(tasks), count)
	}
	for i, task := range tasks {
		if task.ID != int64(i+1) {
			t.Fatalf("tasks[%d].ID = %d, want %d", i, task.ID, i+1)
		}
	}
}
