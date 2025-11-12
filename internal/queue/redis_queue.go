package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/devang/go-task-queue/internal/metrics"
	"github.com/redis/go-redis/v9"
)

type RedisQueue struct {
	client *redis.Client
	ctx    context.Context
}

func NewRedisQueue() *RedisQueue {
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	return &RedisQueue{
		client: rdb,
		ctx:    context.Background(),
	}
}

func (rq *RedisQueue) Enqueue(job Job) error {
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}

	depth, _ := rq.client.LLen(rq.ctx, "job_queue").Result()
	metrics.QueueDepth.Set(float64(depth))

	return rq.client.LPush(rq.ctx, "job_queue", data).Err()
}

func (rq *RedisQueue) Dequeue() (Job, error) {
	result, err := rq.client.BRPop(rq.ctx, 0, "job_queue").Result()
	if err != nil {
		return Job{}, err
	}
	depth, _ := rq.client.LLen(rq.ctx, "job_queue").Result()
	metrics.QueueDepth.Set(float64(depth))

	var job Job
	err = json.Unmarshal([]byte(result[1]), &job)
	return job, err
}

func (rq *RedisQueue) MoveToDLQ(job Job) {
	data, _ := json.Marshal(job)
	rq.client.LPush(rq.ctx, "dead_letter_queue", data)

	// Update DLQ metric
	size, _ := rq.client.LLen(rq.ctx, "dead_letter_queue").Result()
	metrics.DLQSize.Set(float64(size))
}

func (rq *RedisQueue) EnqueueWithDelay(job Job, delay time.Duration) {
	go func() {
		time.Sleep(delay)
		rq.Enqueue(job)

		depth, _ := rq.client.LLen(rq.ctx, "job_queue").Result()
		metrics.QueueDepth.Set(float64(depth))

		fmt.Printf("⏱️ Job #%d requeued after %.0f sec delay\n", job.ID, delay.Seconds())
	}()
}

func (rq *RedisQueue) SetJobStatus(jobID int64, status string) {
	key := fmt.Sprintf("job_status:%d", jobID)
	rq.client.Set(rq.ctx, key, status, 0)
}

func (rq *RedisQueue) GetJobStatus(jobID int64) (string, error) {
	key := fmt.Sprintf("job_status:%d", jobID)
	return rq.client.Get(rq.ctx, key).Result()
}

func (rq *RedisQueue) SaveJob(job Job) {
	key := fmt.Sprintf("job:%d", job.ID)

	rq.client.HSet(rq.ctx, key, map[string]interface{}{
		"id":         job.ID,
		"name":       job.Name,
		"retries":    job.Retries,
		"max_retry":  job.MaxRetry,
		"status":     job.Status,
		"started_at": job.StartedAt.Format(time.RFC3339),
		"ended_at":   job.EndedAt.Format(time.RFC3339),
	})

	if job.Status == "SUCCESS" || job.Status == "FAILED" {
		rq.client.LPush(rq.ctx, "job_history", job.ID)
	}
}

func (rq *RedisQueue) ListRecentJobs(n int64) ([]Job, error) {
	ids, err := rq.client.LRange(rq.ctx, "job_history", 0, n-1).Result()
	if err != nil {
		return nil, err
	}

	var out []Job
	for _, s := range ids {
		key := "job:" + s
		h, err := rq.client.HGetAll(rq.ctx, key).Result()
		if err != nil || len(h) == 0 {
			continue
		}
		var j Job
		// minimal parse (ignore errors for brevity)
		if id, _ := strconv.ParseInt(h["id"], 10, 64); id != 0 {
			j.ID = id
		}
		j.Name = h["name"]
		j.Status = h["status"]
		if r, _ := strconv.Atoi(h["retries"]); r >= 0 {
			j.Retries = r
		}
		j.StartedAt, _ = time.Parse(time.RFC3339, h["started_at"])
		j.EndedAt, _ = time.Parse(time.RFC3339, h["ended_at"])
		out = append(out, j)
	}
	return out, nil
}

func (rq *RedisQueue) ListDLQ(n int64) ([]Job, error) {
	raw, err := rq.client.LRange(rq.ctx, "dead_letter_queue", 0, n-1).Result()
	if err != nil {
		return nil, err
	}
	var out []Job
	for _, s := range raw {
		var j Job
		if err := json.Unmarshal([]byte(s), &j); err == nil {
			out = append(out, j)
		}
	}
	return out, nil
}

func (rq *RedisQueue) Client() *redis.Client {
	return rq.client
}

func (rq *RedisQueue) Ctx() context.Context {
	return rq.ctx
}
