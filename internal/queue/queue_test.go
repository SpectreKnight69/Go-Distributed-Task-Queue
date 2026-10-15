package queue

import (
	"testing"
	"time"
)

func TestEnqueueReturnsIncreasingIDsBeyondFormerChannelCapacity(t *testing.T) {
	q := NewQueue()
	done := make(chan []Job, 1)

	go func() {
		jobs := make([]Job, 20)
		for i := range jobs {
			jobs[i] = q.Enqueue("test")
		}
		done <- jobs
	}()

	var jobs []Job
	select {
	case jobs = <-done:
	case <-time.After(time.Second):
		t.Fatal("Enqueue did not return for 20 jobs")
	}

	for i, job := range jobs {
		wantID := int64(i + 1)
		if job.ID != wantID {
			t.Errorf("job %d has ID %d, want %d", i, job.ID, wantID)
		}
	}
}

func TestRetryBackoff(t *testing.T) {
	tests := []struct {
		name       string
		retryCount int
		want       time.Duration
	}{
		{name: "first retry", retryCount: 1, want: 10 * time.Second},
		{name: "second retry", retryCount: 2, want: 20 * time.Second},
		{name: "third retry at maximum", retryCount: 3, want: 40 * time.Second},
		{name: "no retry beyond maximum", retryCount: 4, want: 0},
		{name: "zero retry count", retryCount: 0, want: 0},
		{name: "negative retry count", retryCount: -1, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := retryBackoff(tt.retryCount); got != tt.want {
				t.Errorf("retryBackoff(%d) = %s, want %s", tt.retryCount, got, tt.want)
			}
		})
	}
}
