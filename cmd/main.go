package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	httpui "github.com/devang/go-task-queue/internal/http"
	"github.com/devang/go-task-queue/internal/metrics"
	"github.com/devang/go-task-queue/internal/queue"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	q := queue.NewQueue(10)
	redisQueue := queue.NewRedisQueue()

	for i := 1; i <= 3; i++ {
		q.StartWorkerWithRedis(i, redisQueue)
	}

	metrics.Register()

	mux := http.NewServeMux()
	httpui.RegisterAdminRoutes(mux, redisQueue)
	httpui.RegisterAdminActions(mux,redisQueue)

	mux.Handle("/metrics", promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{}))

	mux.HandleFunc("/enqueue", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if name == "" {
			name = "Generic Job"
		}
		job := q.Enqueue(name)
		redisQueue.Enqueue(job)
		redisQueue.SetJobStatus(job.ID, "QUEUED")
		fmt.Fprintf(w, "Job %d enqueued successfully\n", job.ID)
		w.Write([]byte("Job added to queue\n"))
	})

	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		idStr := r.URL.Query().Get("id")
		if idStr == "" {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}

		var jobID int64
		fmt.Sscanf(idStr, "%d", &jobID)

		status, err := redisQueue.GetJobStatus(jobID)
		if err != nil {
			http.Error(w, "job not found", http.StatusNotFound)
			return
		}

		fmt.Fprintf(w, "Job %d status: %s\n", jobID, status)
	})

	mux.HandleFunc("/history", func(w http.ResponseWriter, r *http.Request) {
		ids, err := redisQueue.Client().LRange(redisQueue.Ctx(), "job_history", 0, 20).Result()
		if err != nil {
			http.Error(w, "could not fetch history", http.StatusInternalServerError)
			return
		}

		for _, idStr := range ids {
			id, _ := strconv.ParseInt(idStr, 10, 64)
			status, _ := redisQueue.GetJobStatus(id)
			fmt.Fprintf(w, "Job %d - Status: %s\n", id, status)
		}
	})

	mux.HandleFunc("/dlq", func(w http.ResponseWriter, r *http.Request) {
		jobs, _ := redisQueue.Client().LRange(redisQueue.Ctx(), "dead_letter_queue", 0, 20).Result()

		fmt.Fprintf(w, "---- Dead Letter Queue ----\n")
		for _, data := range jobs {
			var job queue.Job
			json.Unmarshal([]byte(data), &job)
			fmt.Fprintf(w, "Job %d (%s), retries: %d\n", job.ID, job.Name, job.Retries)
		}
	})

	fmt.Println("Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))

}
