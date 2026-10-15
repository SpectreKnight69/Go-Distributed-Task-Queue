package main

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/devang/go-task-queue/internal/queue"
)

type fakeEnqueueStore struct {
	enqueueErr   error
	statusErr    error
	enqueueCalls int
	statusCalls  int
	statusJobID  int64
	status       string
}

func (s *fakeEnqueueStore) Enqueue(queue.Job) error {
	s.enqueueCalls++
	return s.enqueueErr
}

func (s *fakeEnqueueStore) SetJobStatus(jobID int64, status string) error {
	s.statusCalls++
	s.statusJobID = jobID
	s.status = status
	return s.statusErr
}

func TestEnqueueHandlerReturnsErrorWhenQueueWriteFails(t *testing.T) {
	store := &fakeEnqueueStore{enqueueErr: errors.New("redis unavailable")}
	job := queue.Job{ID: 42}
	handler := newEnqueueHandler(func(string) queue.Job { return job }, store)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/enqueue?name=Example", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != 500 {
		t.Errorf("status = %d, want %d", recorder.Code, 500)
	}
	if got, want := recorder.Body.String(), "failed to enqueue job\n"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
	if store.enqueueCalls != 1 {
		t.Errorf("enqueue calls = %d, want 1", store.enqueueCalls)
	}
	if store.statusCalls != 0 {
		t.Errorf("status calls = %d, want 0 after enqueue failure", store.statusCalls)
	}
}

func TestEnqueueHandlerPreservesSuccessResponse(t *testing.T) {
	store := &fakeEnqueueStore{}
	job := queue.Job{ID: 42}
	handler := newEnqueueHandler(func(string) queue.Job { return job }, store)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/enqueue?name=Example", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != 200 {
		t.Errorf("status = %d, want %d", recorder.Code, 200)
	}
	if got, want := recorder.Body.String(), "Job 42 enqueued successfully\n"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
	if store.enqueueCalls != 1 {
		t.Errorf("enqueue calls = %d, want 1", store.enqueueCalls)
	}
	if store.statusCalls != 1 || store.statusJobID != job.ID || store.status != "QUEUED" {
		t.Errorf("status update = (%d calls, job %d, status %q), want (1, %d, %q)", store.statusCalls, store.statusJobID, store.status, job.ID, "QUEUED")
	}
}

func TestEnqueueHandlerPreservesSuccessWhenStatusWriteFails(t *testing.T) {
	store := &fakeEnqueueStore{statusErr: errors.New("redis unavailable")}
	job := queue.Job{ID: 42}
	handler := newEnqueueHandler(func(string) queue.Job { return job }, store)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/enqueue?name=Example", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != 200 {
		t.Errorf("status = %d, want %d", recorder.Code, 200)
	}
	if got, want := recorder.Body.String(), "Job 42 enqueued successfully\n"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
	if store.enqueueCalls != 1 {
		t.Errorf("enqueue calls = %d, want 1", store.enqueueCalls)
	}
	if store.statusCalls != 1 || store.statusJobID != job.ID || store.status != "QUEUED" {
		t.Errorf("status update = (%d calls, job %d, status %q), want (1, %d, %q)", store.statusCalls, store.statusJobID, store.status, job.ID, "QUEUED")
	}
}
