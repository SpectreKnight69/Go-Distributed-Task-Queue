package queue

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/devang/go-task-queue/internal/metrics"
)

type Job struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Retries   int       `json:"retries"`
	MaxRetry  int       `json:"max_retry"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
}

const maxRetryCount = 3

// retryBackoff returns the delay for a scheduled retry. Retry counts outside
// the queue's supported range do not have a retry delay.
func retryBackoff(retryCount int) time.Duration {
	if retryCount < 1 || retryCount > maxRetryCount {
		return 0
	}
	return time.Duration(5*(1<<retryCount)) * time.Second
}

type Queue struct {
	counter       int64
	totalDuration float64
	totalJobs     int
	wg            sync.WaitGroup
}

func NewQueue() *Queue {
	rand.Seed(time.Now().UnixNano())
	return &Queue{}
}

func (q *Queue) Enqueue(name string) Job {
	id := atomic.AddInt64(&q.counter, 1)
	job := Job{ID: id, Name: name, MaxRetry: maxRetryCount}
	fmt.Printf("✅ Enqueued job #%d: %s\n", job.ID, job.Name)
	metrics.JobsEnqueued.Inc()
	return job
}

func (q *Queue) StartWorkerWithRedis(workerID int, rq *RedisQueue) {
	go func() {
		for {
			job, err := rq.Dequeue()
			if err != nil {
				fmt.Println("Redis dequeue error:", err)
				continue
			}

			// Track job for graceful shutdown
			q.wg.Add(1)
			go func(job Job) {
				defer q.wg.Done()

				job.Status = "PROCESSING"
				job.StartedAt = time.Now()
				if err := rq.SetJobStatus(job.ID, "PROCESSING"); err != nil {
					log.Printf("worker %d: failed to set PROCESSING status for job %d: %v", workerID, job.ID, err)
				}
				rq.SaveJob(job)

				fmt.Printf("👷 Worker %d started job #%d (%s)\n", workerID, job.ID, job.Name)

				success := q.processJobWithTimeout(workerID, job, 3*time.Second)

				if success {
					job.Status = "SUCCESS"
					job.EndedAt = time.Now()
					if err := rq.SetJobStatus(job.ID, "SUCCESS"); err != nil {
						log.Printf("worker %d: failed to set SUCCESS status for job %d: %v", workerID, job.ID, err)
					}
					rq.SaveJob(job)
					metrics.JobsCompleted.Inc()
				} else if job.Retries < job.MaxRetry {
					job.Retries++
					backoff := retryBackoff(job.Retries)
					rq.EnqueueWithDelay(job, backoff)
					job.Status = "RETRYING"
					if err := rq.SetJobStatus(job.ID, "RETRYING"); err != nil {
						log.Printf("worker %d: failed to set RETRYING status for job %d: %v", workerID, job.ID, err)
					}
					rq.SaveJob(job)
					fmt.Printf("🔁 Retrying job #%d (retry %d) after %.0f seconds\n", job.ID, job.Retries, backoff.Seconds())
				} else {
					job.Status = "FAILED"
					job.EndedAt = time.Now()
					rq.MoveToDLQ(job)
					if err := rq.SetJobStatus(job.ID, "FAILED"); err != nil {
						log.Printf("worker %d: failed to set FAILED status for job %d: %v", workerID, job.ID, err)
					}
					rq.SaveJob(job)
					metrics.JobsFailed.Inc()
				}
			}(job)
		}
	}()
}

func (q *Queue) processJobWithTimeout(workerID int, job Job, timeout time.Duration) bool {
	start := time.Now() // start timer

	done := make(chan bool, 1)

	// Run job in a goroutine
	go func() {
		workTime := time.Duration(rand.Intn(4)+1) * time.Second
		time.Sleep(workTime)

		if rand.Float32() < 0.3 {
			done <- false
			return
		}
		done <- true
	}()

	select {
	case success := <-done:
		duration := time.Since(start).Seconds()
		metrics.JobProcessingTime.Observe(duration)
		metrics.JobProcessingTimeLatest.Set(duration)

		q.totalDuration += duration
		q.totalJobs++
		metrics.JobProcessingTimeAverage.Set(q.totalDuration / float64(q.totalJobs))

		if success {
			fmt.Printf("✅ Worker %d finished job #%d successfully in %.2f sec\n", workerID, job.ID, duration)
			return true
		}

		fmt.Printf("❌ Worker %d failed job #%d in %.2f sec\n", workerID, job.ID, duration)
		return false

	case <-time.After(timeout):
		duration := time.Since(start).Seconds()
		metrics.JobProcessingTime.Observe(duration)
		metrics.JobProcessingTimeLatest.Set(duration)

		q.totalDuration += duration
		q.totalJobs++
		metrics.JobProcessingTimeAverage.Set(q.totalDuration / float64(q.totalJobs))

		fmt.Printf("⏰ Worker %d timeout on job #%d after %.2f sec\n", workerID, job.ID, duration)
		return false
	}
}

func (q *Queue) WaitForOngoingJobs() {
	fmt.Println("🕒 Waiting for all workers to finish current jobs...")
	q.wg.Wait() // q.wg is your worker WaitGroup
	fmt.Println("✅ All workers completed their jobs.")
}
