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

	// Migration commands
	migrateCmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate jobs from external schedulers",
	}

	// K8s discovery subcommand
	k8sDiscoverCmd := &cobra.Command{
		Use:   "k8s-discover",
		Short: "Discover Kubernetes CronJobs",
		Long:  "Discover CronJobs from a Kubernetes cluster using kubeconfig",
		RunE:  k8sDiscover,
	}
	k8sDiscoverCmd.Flags().String("kubeconfig", "", "Path to kubeconfig file (default: ~/.kube/config)")
	k8sDiscoverCmd.Flags().String("context", "", "Kubernetes context to use")
	k8sDiscoverCmd.Flags().StringP("namespace", "n", "", "Filter by namespace (default: all namespaces)")
	k8sDiscoverCmd.Flags().StringP("selector", "l", "", "Label selector (e.g., app=myapp)")
	k8sDiscoverCmd.Flags().Bool("include-disabled", false, "Include suspended CronJobs")

	// K8s plan subcommand
	k8sPlanCmd := &cobra.Command{
		Use:   "k8s-plan",
		Short: "Create a migration plan for Kubernetes CronJobs",
		Long:  "Discover CronJobs and create a migration plan (dry-run)",
		RunE:  k8sPlan,
	}
	k8sPlanCmd.Flags().String("kubeconfig", "", "Path to kubeconfig file")
	k8sPlanCmd.Flags().String("context", "", "Kubernetes context to use")
	k8sPlanCmd.Flags().StringP("namespace", "n", "", "Filter by namespace")
	k8sPlanCmd.Flags().StringP("selector", "l", "", "Label selector")
	k8sPlanCmd.Flags().String("target-namespace", "", "Target Chronos namespace for imported jobs")
	k8sPlanCmd.Flags().String("adapter-url", "http://chronos-k8s-adapter:8081", "Webhook adapter base URL")
	k8sPlanCmd.Flags().StringP("output-file", "f", "", "Write plan to file (YAML)")

	// K8s apply subcommand
	k8sApplyCmd := &cobra.Command{
		Use:   "k8s-apply",
		Short: "Apply a migration plan",
		Long:  "Apply a migration plan to import CronJobs as Chronos jobs",
		RunE:  k8sApply,
	}
	k8sApplyCmd.Flags().StringP("file", "f", "", "Migration plan file to apply")
	k8sApplyCmd.Flags().Bool("dry-run", false, "Preview changes without applying")
	k8sApplyCmd.MarkFlagRequired("file")

	// Generic migrate from file
	migrateFromFileCmd := &cobra.Command{
		Use:   "from-file",
		Short: "Migrate from a file",
		Long:  "Import jobs from a file (supports K8s CronJob, Airflow DAG, EventBridge, Temporal)",
		RunE:  migrateFromFile,
	}
	migrateFromFileCmd.Flags().StringP("file", "f", "", "Source file to migrate")
	migrateFromFileCmd.Flags().String("source-type", "", "Source type: kubernetes, airflow, eventbridge, temporal, github-actions")
	migrateFromFileCmd.Flags().Bool("dry-run", false, "Preview changes without applying")
	migrateFromFileCmd.MarkFlagRequired("file")
	migrateFromFileCmd.MarkFlagRequired("source-type")

	migrateCmd.AddCommand(k8sDiscoverCmd, k8sPlanCmd, k8sApplyCmd, migrateFromFileCmd)
	rootCmd.AddCommand(jobCmd, execCmd, clusterCmd, migrateCmd)

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

// Migration commands

func k8sDiscover(cmd *cobra.Command, args []string) error {
	kubeconfig, _ := cmd.Flags().GetString("kubeconfig")
	context, _ := cmd.Flags().GetString("context")
	namespace, _ := cmd.Flags().GetString("namespace")
	selector, _ := cmd.Flags().GetString("selector")
	includeDisabled, _ := cmd.Flags().GetBool("include-disabled")

	// Make discovery request to API
	reqBody := map[string]interface{}{
		"kubeconfig_path":  kubeconfig,
		"context":          context,
		"namespace":        namespace,
		"label_selector":   selector,
		"include_disabled": includeDisabled,
	}

	result, err := apiRequest("POST", "/api/v1/migration/kubernetes/discover", reqBody)
	if err != nil {
		return err
	}

	data := result["data"].(map[string]interface{})

	if output == "table" {
		cronjobs := data["cronjobs"].([]interface{})
		namespaces := data["namespaces"].([]interface{})

		fmt.Printf("Discovered %d CronJobs across %d namespaces\n\n",
			int(data["total"].(float64)), len(namespaces))

		if len(cronjobs) > 0 {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAMESPACE\tNAME\tSCHEDULE\tIMAGE\tSUSPENDED\tWARNINGS")

			for _, cj := range cronjobs {
				cronjob := cj.(map[string]interface{})
				suspended := "no"
				if s, ok := cronjob["suspend"].(bool); ok && s {
					suspended = "yes"
				}
				warnings := 0
				if w, ok := cronjob["conversion_warnings"].([]interface{}); ok {
					warnings = len(w)
				}
				image := cronjob["image"].(string)
				if len(image) > 40 {
					image = image[:37] + "..."
				}

				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\n",
					cronjob["namespace"],
					cronjob["name"],
					cronjob["schedule"],
					image,
					suspended,
					warnings,
				)
			}
			w.Flush()
		}
	} else {
		printOutput(data)
	}

	return nil
}

func k8sPlan(cmd *cobra.Command, args []string) error {
	kubeconfig, _ := cmd.Flags().GetString("kubeconfig")
	context, _ := cmd.Flags().GetString("context")
	namespace, _ := cmd.Flags().GetString("namespace")
	selector, _ := cmd.Flags().GetString("selector")
	targetNamespace, _ := cmd.Flags().GetString("target-namespace")
	adapterURL, _ := cmd.Flags().GetString("adapter-url")
	outputFile, _ := cmd.Flags().GetString("output-file")

	reqBody := map[string]interface{}{
		"kubeconfig_path":  kubeconfig,
		"context":          context,
		"namespace":        namespace,
		"label_selector":   selector,
		"target_namespace": targetNamespace,
		"adapter_config": map[string]interface{}{
			"base_url": adapterURL,
			"mode":     "kubernetes",
		},
	}

	result, err := apiRequest("POST", "/api/v1/migration/kubernetes/plan", reqBody)
	if err != nil {
		return err
	}

	data := result["data"].(map[string]interface{})

	if outputFile != "" {
		// Write to file
		planData, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal plan: %w", err)
		}

		// Convert to YAML if .yaml extension
		if strings.HasSuffix(outputFile, ".yaml") || strings.HasSuffix(outputFile, ".yml") {
			var planMap map[string]interface{}
			json.Unmarshal(planData, &planMap)
			planData, err = yaml.Marshal(planMap)
			if err != nil {
				return fmt.Errorf("failed to convert to YAML: %w", err)
			}
		}

		if err := os.WriteFile(outputFile, planData, 0644); err != nil {
			return fmt.Errorf("failed to write plan file: %w", err)
		}
		fmt.Printf("Migration plan written to: %s\n", outputFile)
	}

	if output == "table" {
		cronjobs := data["cronjobs"].([]interface{})

		fmt.Printf("Migration Plan: %s\n", data["name"])
		fmt.Printf("Source Cluster: %s\n", data["source_cluster"])
		if tn, ok := data["target_namespace"].(string); ok && tn != "" {
			fmt.Printf("Target Namespace: %s\n", tn)
		}
		fmt.Printf("Total Jobs: %d\n\n", len(cronjobs))

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "SOURCE\tTARGET NAME\tACTION\tWARNINGS")

		for _, item := range cronjobs {
			cj := item.(map[string]interface{})
			source := fmt.Sprintf("%s/%s", cj["source_namespace"], cj["source_name"])
			targetJob := cj["target_job"].(map[string]interface{})
			warnings := 0
			if w, ok := cj["warnings"].([]interface{}); ok {
				warnings = len(w)
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%d\n",
				source,
				targetJob["name"],
				cj["action"],
				warnings,
			)
		}
		w.Flush()
	} else {
		printOutput(data)
	}

	return nil
}

func k8sApply(cmd *cobra.Command, args []string) error {
	file, _ := cmd.Flags().GetString("file")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	// Read plan file
	planData, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read plan file: %w", err)
	}

	var plan map[string]interface{}
	if strings.HasSuffix(file, ".yaml") || strings.HasSuffix(file, ".yml") {
		if err := yaml.Unmarshal(planData, &plan); err != nil {
			return fmt.Errorf("failed to parse YAML: %w", err)
		}
	} else {
		if err := json.Unmarshal(planData, &plan); err != nil {
			return fmt.Errorf("failed to parse JSON: %w", err)
		}
	}

	plan["dry_run"] = dryRun

	result, err := apiRequest("POST", "/api/v1/migration/kubernetes/apply", plan)
	if err != nil {
		return err
	}

	data := result["data"].(map[string]interface{})

	if dryRun {
		fmt.Println("DRY RUN - No changes applied")
		fmt.Println()
	}

	created := int(data["created"].(float64))
	updated := int(data["updated"].(float64))
	skipped := int(data["skipped"].(float64))
	failed := int(data["failed"].(float64))

	fmt.Printf("Migration Results:\n")
	fmt.Printf("  Created: %d\n", created)
	fmt.Printf("  Updated: %d\n", updated)
	fmt.Printf("  Skipped: %d\n", skipped)
	fmt.Printf("  Failed:  %d\n", failed)

	if errors, ok := data["errors"].([]interface{}); ok && len(errors) > 0 {
		fmt.Println("\nErrors:")
		for _, e := range errors {
			errMap := e.(map[string]interface{})
			fmt.Printf("  - %s: %s\n", errMap["item_name"], errMap["error"])
		}
	}

	return nil
}

func migrateFromFile(cmd *cobra.Command, args []string) error {
	file, _ := cmd.Flags().GetString("file")
	sourceType, _ := cmd.Flags().GetString("source-type")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Determine endpoint based on source type
	var endpoint string
	switch sourceType {
	case "kubernetes":
		endpoint = "/api/v1/migration/kubernetes"
	case "airflow":
		endpoint = "/api/v1/migration/airflow"
	case "eventbridge":
		endpoint = "/api/v1/migration/eventbridge"
	case "temporal":
		endpoint = "/api/v1/migration/temporal"
	case "github-actions":
		endpoint = "/api/v1/migration/github-actions"
	default:
		return fmt.Errorf("unsupported source type: %s", sourceType)
	}

	if dryRun {
		endpoint = "/api/v1/migration/preview"
	}

	var reqBody interface{}
	if dryRun {
		reqBody = map[string]interface{}{
			"source_type": sourceType + "_cronjob",
			"data":        string(data),
		}
	} else {
		// Send raw data for non-preview
		reqBody = json.RawMessage(data)
	}

	result, err := apiRequest("POST", endpoint, reqBody)
	if err != nil {
		return err
	}

	resultData := result["data"].(map[string]interface{})

	if dryRun {
		fmt.Println("PREVIEW - No changes applied")
		fmt.Println()
	}

	if output == "table" {
		if jobs, ok := resultData["jobs"].([]interface{}); ok {
			fmt.Printf("Jobs to import: %d\n\n", len(jobs))
			for _, j := range jobs {
				job := j.(map[string]interface{})
				fmt.Printf("  - %s (%s)\n", job["name"], job["schedule"])
			}
		}

		if errors, ok := resultData["errors"].([]interface{}); ok && len(errors) > 0 {
			fmt.Println("\nErrors:")
			for _, e := range errors {
				errMap := e.(map[string]interface{})
				fmt.Printf("  - %s: %s\n", errMap["item_name"], errMap["error"])
			}
		}
	} else {
		printOutput(resultData)
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
