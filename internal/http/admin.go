package httpui

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"

	"github.com/devang/go-task-queue/internal/metrics"
	"github.com/devang/go-task-queue/internal/queue"
)

var adminTmpl = template.Must(template.New("admin").Parse(`
<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Task Queue Admin</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/css/bootstrap.min.css" rel="stylesheet">
<style> body{padding:24px} .badge{font-size:.85rem} </style>
</head>
<body>
<div class="container">
  <h1 class="mb-4">Task Queue Admin</h1>

  <div class="row g-4">
    <div class="col-lg-6">
      <div class="card shadow-sm">
        <div class="card-body">
          <h5 class="card-title">Recent Jobs</h5>
          <div class="table-responsive">
            <table class="table table-sm align-middle">
              <thead><tr>
                <th>ID</th><th>Name</th><th>Status</th><th>Retries</th><th>Started</th><th>Ended</th>
              </tr></thead>
              <tbody>
              {{range .Recent}}
                <tr>
                  <td>#{{.ID}}</td>
                  <td>{{.Name}}</td>
                  <td>
                    {{if eq .Status "SUCCESS"}}<span class="badge bg-success">SUCCESS</span>{{else if eq .Status "FAILED"}}<span class="badge bg-danger">FAILED</span>{{else if eq .Status "RETRYING"}}<span class="badge bg-warning text-dark">RETRYING</span>{{else if eq .Status "PROCESSING"}}<span class="badge bg-primary">PROCESSING</span>{{else}}<span class="badge bg-secondary">QUEUED</span>{{end}}
                  </td>
                  <td>{{.Retries}}</td>
                  <td>{{.StartedAt.Format "15:04:05"}}</td>
                  <td>{{.EndedAt.Format "15:04:05"}}</td>
                </tr>
              {{else}}
                <tr><td colspan="6" class="text-muted">No recent jobs.</td></tr>
              {{end}}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>

    <div class="col-lg-6">
      <div class="card shadow-sm">
        <div class="card-body">
          <h5 class="card-title">Dead Letter Queue</h5>
          <div class="table-responsive">
            <table class="table table-sm align-middle">
              <thead><tr>
                <th>ID</th><th>Name</th><th>Retries</th>
              </tr></thead>
              <tbody>
              {{range .DLQ}}
                <tr>
                  <td>#{{.ID}}</td>
                  <td>{{.Name}}</td>
                  <td>{{.Retries}}</td>
				  <td>
    				<a href="/dlq/retry?id={{.ID}}" class="btn btn-outline-primary btn-sm">Retry</a>
   				 	<a href="/dlq/delete?id={{.ID}}" class="btn btn-outline-danger btn-sm">Delete</a>
  				  </td>
                </tr>
              {{else}}
                <tr><td colspan="3" class="text-muted">DLQ is empty.</td></tr>
              {{end}}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  </div>

</div>
</body>
</html>
`))

func RegisterAdminRoutes(mux *http.ServeMux, rq *queue.RedisQueue) {
	mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		recent, _ := rq.ListRecentJobs(20)
		dlq, _ := rq.ListDLQ(20)

		_ = adminTmpl.Execute(w, map[string]any{
			"Recent": recent,
			"DLQ":    dlq,
		})
	})
}

func RegisterAdminActions(mux *http.ServeMux, rq *queue.RedisQueue) {
	// Retry job from DLQ
	mux.HandleFunc("/dlq/retry", func(w http.ResponseWriter, r *http.Request) {
		idStr := r.URL.Query().Get("id")
		if idStr == "" {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}

		id, _ := strconv.ParseInt(idStr, 10, 64)

		// Find the job in DLQ
		rawJobs, _ := rq.Client().LRange(rq.Ctx(), "dead_letter_queue", 0, -1).Result()
		for _, raw := range rawJobs {
			var job queue.Job
			_ = json.Unmarshal([]byte(raw), &job)

			if job.ID == id {
				// Remove from DLQ
				rq.Client().LRem(rq.Ctx(), "dead_letter_queue", 1, raw)

				// Reset retries and requeue
				job.Retries = 0
				rq.Enqueue(job)

				// Update DLQ metric
				size, _ := rq.Client().LLen(rq.Ctx(), "dead_letter_queue").Result()
				metrics.DLQSize.Set(float64(size))

				http.Redirect(w, r, "/admin", http.StatusSeeOther)
				return
			}
		}

		http.Error(w, "job not found", http.StatusNotFound)
	})

	// Delete job permanently
	mux.HandleFunc("/dlq/delete", func(w http.ResponseWriter, r *http.Request) {
		idStr := r.URL.Query().Get("id")
		if idStr == "" {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}

		id, _ := strconv.ParseInt(idStr, 10, 64)

		rawJobs, _ := rq.Client().LRange(rq.Ctx(), "dead_letter_queue", 0, -1).Result()
		for _, raw := range rawJobs {
			var job queue.Job
			_ = json.Unmarshal([]byte(raw), &job)

			if job.ID == id {
				rq.Client().LRem(rq.Ctx(), "dead_letter_queue", 1, raw)

				// Update DLQ metric
				size, _ := rq.Client().LLen(rq.Ctx(), "dead_letter_queue").Result()
				metrics.DLQSize.Set(float64(size))

				http.Redirect(w, r, "/admin", http.StatusSeeOther)
				return
			}
		}

		http.Error(w, "job not found", http.StatusNotFound)
	})
}
