// Package dashboard provides a terminal dashboard for Chronos.
package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Client fetches data from the Chronos API.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewClient creates a new dashboard client.
func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// DashboardData holds all data rendered in the dashboard.
type DashboardData struct {
	Jobs       []JobSummary       `json:"jobs"`
	Executions []ExecutionSummary `json:"executions"`
	Cluster    ClusterInfo        `json:"cluster"`
	FetchedAt  time.Time          `json:"fetched_at"`
}

// JobSummary is a simplified job for dashboard display.
type JobSummary struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Schedule string `json:"schedule"`
	Enabled  bool   `json:"enabled"`
}

// ExecutionSummary is a simplified execution for dashboard display.
type ExecutionSummary struct {
	ID        string `json:"id"`
	JobName   string `json:"job_name"`
	Status    string `json:"status"`
	Duration  int64  `json:"duration"`
	StartedAt string `json:"started_at"`
	Error     string `json:"error,omitempty"`
	TraceID   string `json:"trace_id,omitempty"`
}

// ClusterInfo contains cluster status.
type ClusterInfo struct {
	IsLeader  bool `json:"is_leader"`
	JobsTotal int  `json:"jobs_total"`
	Running   int  `json:"running"`
}

// Fetch retrieves dashboard data from the Chronos API.
func (c *Client) Fetch(ctx context.Context) *DashboardData {
	data := &DashboardData{FetchedAt: time.Now()}

	if jobs, err := c.fetchJSON(ctx, "/api/v1/jobs"); err == nil {
		if jobsData, ok := jobs["data"]; ok {
			if jobsMap, ok := jobsData.(map[string]interface{}); ok {
				if jobList, ok := jobsMap["jobs"].([]interface{}); ok {
					for _, j := range jobList {
						if jm, ok := j.(map[string]interface{}); ok {
							data.Jobs = append(data.Jobs, JobSummary{
								ID:       strVal(jm, "id"),
								Name:     strVal(jm, "name"),
								Schedule: strVal(jm, "schedule"),
								Enabled:  boolVal(jm, "enabled"),
							})
						}
					}
				}
			}
		}
	}

	if cluster, err := c.fetchJSON(ctx, "/api/v1/cluster/status"); err == nil {
		if d, ok := cluster["data"]; ok {
			if cm, ok := d.(map[string]interface{}); ok {
				data.Cluster = ClusterInfo{
					IsLeader:  boolVal(cm, "is_leader"),
					JobsTotal: intVal(cm, "jobs_total"),
					Running:   intVal(cm, "running"),
				}
			}
		}
	}

	return data
}

func (c *Client) fetchJSON(ctx context.Context, path string) (map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	return result, nil
}

// Render formats dashboard data as a terminal-friendly string.
func Render(data *DashboardData) string {
	var b strings.Builder
	w := 60

	b.WriteString(box("CHRONOS DASHBOARD", w))
	b.WriteString(fmt.Sprintf(" Updated: %s\n\n", data.FetchedAt.Format("15:04:05")))

	leaderStr := "✓ Leader"
	if !data.Cluster.IsLeader {
		leaderStr = "○ Follower"
	}
	b.WriteString(fmt.Sprintf(" %-20s │ Jobs: %-5d │ Running: %d\n",
		leaderStr, data.Cluster.JobsTotal, data.Cluster.Running))
	b.WriteString(strings.Repeat("─", w) + "\n\n")

	b.WriteString(" JOBS\n")
	b.WriteString(fmt.Sprintf(" %-25s %-18s %s\n", "NAME", "SCHEDULE", "STATUS"))
	b.WriteString(" " + strings.Repeat("─", w-2) + "\n")

	sort.Slice(data.Jobs, func(i, j int) bool { return data.Jobs[i].Name < data.Jobs[j].Name })
	for _, job := range data.Jobs {
		status := "enabled"
		if !job.Enabled {
			status = "disabled"
		}
		name := truncStr(job.Name, 24)
		b.WriteString(fmt.Sprintf(" %-25s %-18s %s\n", name, job.Schedule, status))
	}

	if len(data.Executions) > 0 {
		b.WriteString("\n RECENT EXECUTIONS\n")
		b.WriteString(fmt.Sprintf(" %-20s %-10s %-10s %s\n", "JOB", "STATUS", "DURATION", "TRACE"))
		b.WriteString(" " + strings.Repeat("─", w-2) + "\n")
		for _, exec := range data.Executions {
			trace := exec.TraceID
			if len(trace) > 8 {
				trace = trace[:8]
			}
			b.WriteString(fmt.Sprintf(" %-20s %-10s %-10s %s\n",
				truncStr(exec.JobName, 19), exec.Status, fmt.Sprintf("%dms", exec.Duration), trace))
		}
	}

	b.WriteString("\n" + strings.Repeat("─", w) + "\n")
	b.WriteString(" Press Ctrl+C to exit\n")
	return b.String()
}

func box(title string, width int) string {
	padding := width - len(title) - 4
	left := padding / 2
	right := padding - left
	return fmt.Sprintf("┌%s┐\n│%s%s%s│\n└%s┘\n",
		strings.Repeat("─", width-2),
		strings.Repeat(" ", left+1), title, strings.Repeat(" ", right+1),
		strings.Repeat("─", width-2))
}

func truncStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func strVal(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return v
}

func boolVal(m map[string]interface{}, key string) bool {
	v, _ := m[key].(bool)
	return v
}

func intVal(m map[string]interface{}, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return 0
}
