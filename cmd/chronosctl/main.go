// chronosctl - CLI tool for Chronos
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
)

var (
	serverURL string
	output    string
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "chronosctl",
		Short:   "Chronos CLI - Manage your distributed cron jobs",
		Version: fmt.Sprintf("%s (built %s)", Version, BuildTime),
	}

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&serverURL, "server", "s", "http://localhost:8080", "Chronos server URL")
	rootCmd.PersistentFlags().StringVarP(&output, "output", "o", "table", "Output format (table, json, yaml)")

	// Job commands
	jobCmd := &cobra.Command{
		Use:   "job",
		Short: "Manage jobs",
	}

	jobCmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List all jobs",
			RunE:  listJobs,
		},
		&cobra.Command{
			Use:   "get [job-id]",
			Short: "Get job details",
			Args:  cobra.ExactArgs(1),
			RunE:  getJob,
		},
		&cobra.Command{
			Use:   "create -f [file]",
			Short: "Create a new job",
			RunE:  createJob,
		},
		&cobra.Command{
			Use:   "delete [job-id]",
			Short: "Delete a job",
			Args:  cobra.ExactArgs(1),
			RunE:  deleteJob,
		},
		&cobra.Command{
			Use:   "trigger [job-id]",
			Short: "Trigger a job execution",
			Args:  cobra.ExactArgs(1),
			RunE:  triggerJob,
		},
		&cobra.Command{
			Use:   "enable [job-id]",
			Short: "Enable a job",
			Args:  cobra.ExactArgs(1),
			RunE:  enableJob,
		},
		&cobra.Command{
			Use:   "disable [job-id]",
			Short: "Disable a job",
			Args:  cobra.ExactArgs(1),
			RunE:  disableJob,
		},
	)

	// Add file flag for create
	for _, cmd := range jobCmd.Commands() {
		if cmd.Use == "create -f [file]" {
			cmd.Flags().StringP("file", "f", "", "Job definition file (YAML or JSON)")
			cmd.MarkFlagRequired("file")
		}
	}

	// Execution commands
	execCmd := &cobra.Command{
		Use:   "execution",
		Short: "Manage executions",
	}

	execCmd.AddCommand(
		&cobra.Command{
			Use:   "list [job-id]",
			Short: "List executions for a job",
			Args:  cobra.ExactArgs(1),
			RunE:  listExecutions,
		},
		&cobra.Command{
			Use:   "get [job-id] [execution-id]",
			Short: "Get execution details",
			Args:  cobra.ExactArgs(2),
			RunE:  getExecution,
		},
	)

	// Cluster commands
	clusterCmd := &cobra.Command{
		Use:   "cluster",
		Short: "Manage cluster",
	}

	clusterCmd.AddCommand(
		&cobra.Command{
			Use:   "status",
			Short: "Get cluster status",
			RunE:  clusterStatus,
		},
	)

	rootCmd.AddCommand(jobCmd, execCmd, clusterCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// API client

func apiRequest(method, path string, body interface{}) (map[string]interface{}, error) {
	url := serverURL + path

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if success, ok := result["success"].(bool); !ok || !success {
		if errInfo, ok := result["error"].(map[string]interface{}); ok {
			return nil, fmt.Errorf("%s: %s", errInfo["code"], errInfo["message"])
		}
		return nil, fmt.Errorf("request failed")
	}

	return result, nil
}

// Output helpers

func printOutput(data interface{}) {
	switch output {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(data)
	case "yaml":
		enc := yaml.NewEncoder(os.Stdout)
		enc.Encode(data)
	default:
		// Table format handled by specific commands
	}
}

func printJobsTable(jobs []interface{}) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tSCHEDULE\tENABLED\tNEXT RUN")

	for _, j := range jobs {
		job := j.(map[string]interface{})
		enabled := "no"
		if e, ok := job["enabled"].(bool); ok && e {
			enabled = "yes"
		}
		nextRun := "-"
		if nr, ok := job["next_run"].(string); ok && nr != "" {
			if t, err := time.Parse(time.RFC3339, nr); err == nil {
				nextRun = t.Local().Format("2006-01-02 15:04:05")
			}
		}

		id := job["id"].(string)
		if len(id) > 8 {
			id = id[:8]
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			id,
			job["name"],
			job["schedule"],
			enabled,
			nextRun,
		)
	}
	w.Flush()
}

func printExecutionsTable(executions []interface{}) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSTATUS\tATTEMPTS\tSTARTED\tDURATION")

	for _, e := range executions {
		exec := e.(map[string]interface{})
		id := exec["id"].(string)
		if len(id) > 8 {
			id = id[:8]
		}

		started := "-"
		if s, ok := exec["started_at"].(string); ok {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				started = t.Local().Format("2006-01-02 15:04:05")
			}
		}

		duration := "-"
		if d, ok := exec["duration"].(float64); ok {
			duration = time.Duration(d).String()
		}

		fmt.Fprintf(w, "%s\t%s\t%.0f\t%s\t%s\n",
			id,
			exec["status"],
			exec["attempts"],
			started,
			duration,
		)
	}
	w.Flush()
}

// Job commands

func listJobs(cmd *cobra.Command, args []string) error {
	result, err := apiRequest("GET", "/api/v1/jobs", nil)
	if err != nil {
		return err
	}

	data := result["data"].(map[string]interface{})
	jobs := data["jobs"].([]interface{})

	if output == "table" {
		fmt.Printf("Total: %d jobs\n\n", len(jobs))
		printJobsTable(jobs)
	} else {
		printOutput(jobs)
	}

	return nil
}

func getJob(cmd *cobra.Command, args []string) error {
	result, err := apiRequest("GET", "/api/v1/jobs/"+args[0], nil)
	if err != nil {
		return err
	}

	data := result["data"]

	if output == "table" {
		job := data.(map[string]interface{})
		fmt.Printf("ID:          %s\n", job["id"])
		fmt.Printf("Name:        %s\n", job["name"])
		fmt.Printf("Description: %s\n", job["description"])
		fmt.Printf("Schedule:    %s\n", job["schedule"])
		fmt.Printf("Timezone:    %s\n", job["timezone"])
		fmt.Printf("Enabled:     %v\n", job["enabled"])
		if webhook, ok := job["webhook"].(map[string]interface{}); ok {
			fmt.Printf("Webhook URL: %s\n", webhook["url"])
			fmt.Printf("Method:      %s\n", webhook["method"])
		}
		if nextRun, ok := job["next_run"].(string); ok && nextRun != "" {
			if t, err := time.Parse(time.RFC3339, nextRun); err == nil {
				fmt.Printf("Next Run:    %s\n", t.Local().Format("2006-01-02 15:04:05"))
			}
		}
	} else {
		printOutput(data)
	}

	return nil
}

func createJob(cmd *cobra.Command, args []string) error {
	file, _ := cmd.Flags().GetString("file")

	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var jobDef map[string]interface{}
	if strings.HasSuffix(file, ".yaml") || strings.HasSuffix(file, ".yml") {
		if err := yaml.Unmarshal(data, &jobDef); err != nil {
			return fmt.Errorf("failed to parse YAML: %w", err)
		}
	} else {
		if err := json.Unmarshal(data, &jobDef); err != nil {
			return fmt.Errorf("failed to parse JSON: %w", err)
		}
	}

	result, err := apiRequest("POST", "/api/v1/jobs", jobDef)
	if err != nil {
		return err
	}

	job := result["data"].(map[string]interface{})
	fmt.Printf("Job created: %s (%s)\n", job["name"], job["id"])

	return nil
}

func deleteJob(cmd *cobra.Command, args []string) error {
	_, err := apiRequest("DELETE", "/api/v1/jobs/"+args[0], nil)
	if err != nil {
		return err
	}

	fmt.Println("Job deleted")
	return nil
}

func triggerJob(cmd *cobra.Command, args []string) error {
	result, err := apiRequest("POST", "/api/v1/jobs/"+args[0]+"/trigger", nil)
	if err != nil {
		return err
	}

	exec := result["data"].(map[string]interface{})
	fmt.Printf("Job triggered: execution %s, status: %s\n", exec["id"], exec["status"])

	return nil
}

func enableJob(cmd *cobra.Command, args []string) error {
	_, err := apiRequest("POST", "/api/v1/jobs/"+args[0]+"/enable", nil)
	if err != nil {
		return err
	}

	fmt.Println("Job enabled")
	return nil
}

func disableJob(cmd *cobra.Command, args []string) error {
	_, err := apiRequest("POST", "/api/v1/jobs/"+args[0]+"/disable", nil)
	if err != nil {
		return err
	}

	fmt.Println("Job disabled")
	return nil
}

// Execution commands

func listExecutions(cmd *cobra.Command, args []string) error {
	result, err := apiRequest("GET", "/api/v1/jobs/"+args[0]+"/executions", nil)
	if err != nil {
		return err
	}

	data := result["data"].(map[string]interface{})
	executions := data["executions"].([]interface{})

	if output == "table" {
		fmt.Printf("Total: %d executions\n\n", len(executions))
		printExecutionsTable(executions)
	} else {
		printOutput(executions)
	}

	return nil
}

func getExecution(cmd *cobra.Command, args []string) error {
	result, err := apiRequest("GET", "/api/v1/jobs/"+args[0]+"/executions/"+args[1], nil)
	if err != nil {
		return err
	}

	data := result["data"]

	if output == "table" {
		exec := data.(map[string]interface{})
		fmt.Printf("ID:         %s\n", exec["id"])
		fmt.Printf("Job ID:     %s\n", exec["job_id"])
		fmt.Printf("Status:     %s\n", exec["status"])
		fmt.Printf("Attempts:   %.0f\n", exec["attempts"])
		if s, ok := exec["started_at"].(string); ok {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				fmt.Printf("Started:    %s\n", t.Local().Format("2006-01-02 15:04:05"))
			}
		}
		if e, ok := exec["error"].(string); ok && e != "" {
			fmt.Printf("Error:      %s\n", e)
		}
		if r, ok := exec["response"].(string); ok && r != "" {
			fmt.Printf("Response:   %s\n", truncate(r, 200))
		}
	} else {
		printOutput(data)
	}

	return nil
}

// Cluster commands

func clusterStatus(cmd *cobra.Command, args []string) error {
	result, err := apiRequest("GET", "/api/v1/cluster/status", nil)
	if err != nil {
		return err
	}

	data := result["data"]

	if output == "table" {
		status := data.(map[string]interface{})
		fmt.Printf("Is Leader:  %v\n", status["is_leader"])
		fmt.Printf("Jobs Total: %.0f\n", status["jobs_total"])
	} else {
		printOutput(data)
	}

	return nil
}

// Helpers

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
