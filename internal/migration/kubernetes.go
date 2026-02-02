// Package migration provides Kubernetes CronJob discovery and migration.
package migration

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chronos/chronos/internal/models"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// KubernetesDiscoveryConfig configures the Kubernetes discovery client.
type KubernetesDiscoveryConfig struct {
	// KubeconfigPath is the path to the kubeconfig file.
	// If empty, uses default locations (~/.kube/config or KUBECONFIG env).
	KubeconfigPath string `json:"kubeconfig_path" yaml:"kubeconfig_path"`

	// Context is the kubeconfig context to use. If empty, uses current context.
	Context string `json:"context" yaml:"context"`

	// Namespace filters discovery to a specific namespace. If empty, discovers all namespaces.
	Namespace string `json:"namespace" yaml:"namespace"`

	// LabelSelector filters CronJobs by label selector (e.g., "app=myapp").
	LabelSelector string `json:"label_selector" yaml:"label_selector"`

	// IncludeDisabled includes suspended CronJobs in discovery.
	IncludeDisabled bool `json:"include_disabled" yaml:"include_disabled"`
}

// KubernetesDiscoveryResult contains the result of Kubernetes CronJob discovery.
type KubernetesDiscoveryResult struct {
	Success       bool                    `json:"success"`
	ClusterName   string                  `json:"cluster_name"`
	Context       string                  `json:"context"`
	CronJobs      []*DiscoveredCronJob    `json:"cronjobs"`
	Total         int                     `json:"total"`
	Namespaces    []string                `json:"namespaces"`
	DiscoveredAt  time.Time               `json:"discovered_at"`
	Errors        []string                `json:"errors,omitempty"`
}

// DiscoveredCronJob represents a discovered Kubernetes CronJob with conversion preview.
type DiscoveredCronJob struct {
	// Original Kubernetes metadata
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	UID         string            `json:"uid"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`

	// Schedule info
	Schedule    string `json:"schedule"`
	Timezone    string `json:"timezone,omitempty"`
	Suspend     bool   `json:"suspend"`

	// Job template info
	Image       string   `json:"image"`
	Command     []string `json:"command,omitempty"`
	Args        []string `json:"args,omitempty"`

	// Policies
	ConcurrencyPolicy          string `json:"concurrency_policy"`
	StartingDeadlineSeconds    *int64 `json:"starting_deadline_seconds,omitempty"`
	BackoffLimit               *int32 `json:"backoff_limit,omitempty"`

	// Status
	LastScheduleTime    *time.Time `json:"last_schedule_time,omitempty"`
	LastSuccessfulTime  *time.Time `json:"last_successful_time,omitempty"`
	ActiveJobs          int        `json:"active_jobs"`

	// Conversion preview
	ConvertedJob  *models.Job `json:"converted_job,omitempty"`
	ConversionWarnings []string `json:"conversion_warnings,omitempty"`
	RequiresAdapter    bool     `json:"requires_adapter"`
}

// MigrationPlan represents a plan for migrating CronJobs to Chronos.
type MigrationPlan struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	Description     string                 `json:"description,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
	SourceCluster   string                 `json:"source_cluster"`
	TargetNamespace string                 `json:"target_namespace,omitempty"`
	CronJobs        []*MigrationPlanItem   `json:"cronjobs"`
	AdapterConfig   *WebhookAdapterConfig  `json:"adapter_config,omitempty"`
	DryRun          bool                   `json:"dry_run"`
	Status          MigrationPlanStatus    `json:"status"`
}

// MigrationPlanItem represents a single CronJob in a migration plan.
type MigrationPlanItem struct {
	SourceName      string       `json:"source_name"`
	SourceNamespace string       `json:"source_namespace"`
	TargetJob       *models.Job  `json:"target_job"`
	Action          string       `json:"action"` // migrate, skip, manual
	Reason          string       `json:"reason,omitempty"`
	Warnings        []string     `json:"warnings,omitempty"`
}

// MigrationPlanStatus represents the status of a migration plan.
type MigrationPlanStatus string

const (
	MigrationPlanPending   MigrationPlanStatus = "pending"
	MigrationPlanRunning   MigrationPlanStatus = "running"
	MigrationPlanCompleted MigrationPlanStatus = "completed"
	MigrationPlanFailed    MigrationPlanStatus = "failed"
)

// WebhookAdapterConfig configures the webhook adapter for container-based jobs.
type WebhookAdapterConfig struct {
	// BaseURL is the base URL of the webhook adapter service.
	BaseURL string `json:"base_url" yaml:"base_url"`

	// AuthToken is the authentication token for the adapter.
	AuthToken string `json:"auth_token,omitempty" yaml:"auth_token,omitempty"`

	// Timeout is the default timeout for adapter requests.
	Timeout time.Duration `json:"timeout" yaml:"timeout"`

	// Mode determines how jobs are executed.
	// - "kubernetes": Creates K8s Jobs to run containers
	// - "docker": Runs containers via Docker API
	// - "proxy": Proxies to original container behavior
	Mode string `json:"mode" yaml:"mode"`

	// KubernetesConfig is used when Mode is "kubernetes".
	KubernetesConfig *AdapterKubernetesConfig `json:"kubernetes_config,omitempty" yaml:"kubernetes_config,omitempty"`
}

// AdapterKubernetesConfig configures Kubernetes job execution.
type AdapterKubernetesConfig struct {
	Namespace          string            `json:"namespace" yaml:"namespace"`
	ServiceAccountName string            `json:"service_account_name,omitempty" yaml:"service_account_name,omitempty"`
	ImagePullSecrets   []string          `json:"image_pull_secrets,omitempty" yaml:"image_pull_secrets,omitempty"`
	NodeSelector       map[string]string `json:"node_selector,omitempty" yaml:"node_selector,omitempty"`
}

// KubernetesDiscovery discovers CronJobs from a Kubernetes cluster.
type KubernetesDiscovery struct {
	config     KubernetesDiscoveryConfig
	httpClient *http.Client
	apiServer  string
	token      string
	caCert     []byte
}

// Kubeconfig represents the kubeconfig file structure.
type Kubeconfig struct {
	APIVersion     string `yaml:"apiVersion"`
	Kind           string `yaml:"kind"`
	CurrentContext string `yaml:"current-context"`
	Clusters       []struct {
		Name    string `yaml:"name"`
		Cluster struct {
			Server                   string `yaml:"server"`
			CertificateAuthorityData string `yaml:"certificate-authority-data"`
			CertificateAuthority     string `yaml:"certificate-authority"`
			InsecureSkipTLSVerify    bool   `yaml:"insecure-skip-tls-verify"`
		} `yaml:"cluster"`
	} `yaml:"clusters"`
	Contexts []struct {
		Name    string `yaml:"name"`
		Context struct {
			Cluster   string `yaml:"cluster"`
			User      string `yaml:"user"`
			Namespace string `yaml:"namespace"`
		} `yaml:"context"`
	} `yaml:"contexts"`
	Users []struct {
		Name string `yaml:"name"`
		User struct {
			Token                 string `yaml:"token"`
			ClientCertificateData string `yaml:"client-certificate-data"`
			ClientKeyData         string `yaml:"client-key-data"`
			Exec                  *struct {
				APIVersion string   `yaml:"apiVersion"`
				Command    string   `yaml:"command"`
				Args       []string `yaml:"args"`
			} `yaml:"exec"`
		} `yaml:"user"`
	} `yaml:"users"`
}

// NewKubernetesDiscovery creates a new Kubernetes discovery client.
func NewKubernetesDiscovery(cfg KubernetesDiscoveryConfig) (*KubernetesDiscovery, error) {
	d := &KubernetesDiscovery{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	if err := d.loadKubeconfig(); err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	return d, nil
}

// loadKubeconfig loads and parses the kubeconfig file.
func (d *KubernetesDiscovery) loadKubeconfig() error {
	kubeconfigPath := d.config.KubeconfigPath
	if kubeconfigPath == "" {
		// Try KUBECONFIG env first
		kubeconfigPath = os.Getenv("KUBECONFIG")
		if kubeconfigPath == "" {
			// Fall back to default location
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("cannot determine home directory: %w", err)
			}
			kubeconfigPath = filepath.Join(home, ".kube", "config")
		}
	}

	data, err := os.ReadFile(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("cannot read kubeconfig from %s: %w", kubeconfigPath, err)
	}

	var kc Kubeconfig
	if err := yaml.Unmarshal(data, &kc); err != nil {
		return fmt.Errorf("cannot parse kubeconfig: %w", err)
	}

	// Determine which context to use
	contextName := d.config.Context
	if contextName == "" {
		contextName = kc.CurrentContext
	}

	// Find the context
	var clusterName, userName string
	for _, ctx := range kc.Contexts {
		if ctx.Name == contextName {
			clusterName = ctx.Context.Cluster
			userName = ctx.Context.User
			break
		}
	}

	if clusterName == "" {
		return fmt.Errorf("context %q not found in kubeconfig", contextName)
	}

	// Find the cluster
	for _, cluster := range kc.Clusters {
		if cluster.Name == clusterName {
			d.apiServer = cluster.Cluster.Server
			if cluster.Cluster.CertificateAuthorityData != "" {
				d.caCert, _ = base64.StdEncoding.DecodeString(cluster.Cluster.CertificateAuthorityData)
			}
			break
		}
	}

	// Find the user
	for _, user := range kc.Users {
		if user.Name == userName {
			d.token = user.User.Token
			break
		}
	}

	if d.apiServer == "" {
		return fmt.Errorf("cluster %q not found in kubeconfig", clusterName)
	}

	return nil
}

// Discover discovers CronJobs from the Kubernetes cluster.
func (d *KubernetesDiscovery) Discover(ctx context.Context) (*KubernetesDiscoveryResult, error) {
	result := &KubernetesDiscoveryResult{
		DiscoveredAt: time.Now().UTC(),
		CronJobs:     make([]*DiscoveredCronJob, 0),
		Namespaces:   make([]string, 0),
		Errors:       make([]string, 0),
	}

	// Build API URL
	url := d.apiServer + "/apis/batch/v1/cronjobs"
	if d.config.Namespace != "" {
		url = d.apiServer + "/apis/batch/v1/namespaces/" + d.config.Namespace + "/cronjobs"
	}

	if d.config.LabelSelector != "" {
		url += "?labelSelector=" + d.config.LabelSelector
	}

	// Make request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if d.token != "" {
		req.Header.Set("Authorization", "Bearer "+d.token)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Kubernetes API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Kubernetes API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var cronJobList K8sCronJobList
	if err := json.NewDecoder(resp.Body).Decode(&cronJobList); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Track namespaces
	namespaceSet := make(map[string]bool)

	// Convert each CronJob
	for _, cj := range cronJobList.Items {
		// Skip suspended if not requested
		if cj.Spec.Suspend != nil && *cj.Spec.Suspend && !d.config.IncludeDisabled {
			continue
		}

		discovered := d.convertToDiscoveredCronJob(cj)
		result.CronJobs = append(result.CronJobs, discovered)
		namespaceSet[cj.Metadata.Namespace] = true
	}

	for ns := range namespaceSet {
		result.Namespaces = append(result.Namespaces, ns)
	}

	result.Total = len(result.CronJobs)
	result.Success = len(result.Errors) == 0

	return result, nil
}

// K8sCronJobList represents a Kubernetes CronJob list response.
type K8sCronJobList struct {
	APIVersion string        `json:"apiVersion"`
	Kind       string        `json:"kind"`
	Items      []K8sCronJob  `json:"items"`
}

// K8sCronJob represents a Kubernetes CronJob.
type K8sCronJob struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name              string            `json:"name"`
		Namespace         string            `json:"namespace"`
		UID               string            `json:"uid"`
		Labels            map[string]string `json:"labels"`
		Annotations       map[string]string `json:"annotations"`
		CreationTimestamp string            `json:"creationTimestamp"`
	} `json:"metadata"`
	Spec struct {
		Schedule                   string  `json:"schedule"`
		TimeZone                   *string `json:"timeZone"`
		Suspend                    *bool   `json:"suspend"`
		ConcurrencyPolicy          string  `json:"concurrencyPolicy"`
		StartingDeadlineSeconds    *int64  `json:"startingDeadlineSeconds"`
		SuccessfulJobsHistoryLimit *int32  `json:"successfulJobsHistoryLimit"`
		FailedJobsHistoryLimit     *int32  `json:"failedJobsHistoryLimit"`
		JobTemplate                struct {
			Spec struct {
				BackoffLimit   *int32 `json:"backoffLimit"`
				Template       struct {
					Spec struct {
						Containers []struct {
							Name       string            `json:"name"`
							Image      string            `json:"image"`
							Command    []string          `json:"command"`
							Args       []string          `json:"args"`
							Env        []K8sEnvVar       `json:"env"`
							EnvFrom    []K8sEnvFromSource `json:"envFrom"`
							WorkingDir string            `json:"workingDir"`
						} `json:"containers"`
						RestartPolicy string `json:"restartPolicy"`
					} `json:"spec"`
				} `json:"template"`
			} `json:"spec"`
		} `json:"jobTemplate"`
	} `json:"spec"`
	Status struct {
		Active             []interface{} `json:"active"`
		LastScheduleTime   *string       `json:"lastScheduleTime"`
		LastSuccessfulTime *string       `json:"lastSuccessfulTime"`
	} `json:"status"`
}

// K8sEnvVar represents a Kubernetes environment variable.
type K8sEnvVar struct {
	Name      string `json:"name"`
	Value     string `json:"value,omitempty"`
	ValueFrom *struct {
		SecretKeyRef *struct {
			Name string `json:"name"`
			Key  string `json:"key"`
		} `json:"secretKeyRef,omitempty"`
		ConfigMapKeyRef *struct {
			Name string `json:"name"`
			Key  string `json:"key"`
		} `json:"configMapKeyRef,omitempty"`
	} `json:"valueFrom,omitempty"`
}

// K8sEnvFromSource represents a Kubernetes envFrom source.
type K8sEnvFromSource struct {
	SecretRef *struct {
		Name string `json:"name"`
	} `json:"secretRef,omitempty"`
	ConfigMapRef *struct {
		Name string `json:"name"`
	} `json:"configMapRef,omitempty"`
}

func (d *KubernetesDiscovery) convertToDiscoveredCronJob(cj K8sCronJob) *DiscoveredCronJob {
	discovered := &DiscoveredCronJob{
		Name:              cj.Metadata.Name,
		Namespace:         cj.Metadata.Namespace,
		UID:               cj.Metadata.UID,
		Labels:            cj.Metadata.Labels,
		Annotations:       cj.Metadata.Annotations,
		Schedule:          cj.Spec.Schedule,
		ConcurrencyPolicy: cj.Spec.ConcurrencyPolicy,
		StartingDeadlineSeconds: cj.Spec.StartingDeadlineSeconds,
		BackoffLimit:            cj.Spec.JobTemplate.Spec.BackoffLimit,
		ActiveJobs:              len(cj.Status.Active),
		ConversionWarnings:      make([]string, 0),
	}

	if cj.Spec.TimeZone != nil {
		discovered.Timezone = *cj.Spec.TimeZone
	}

	if cj.Spec.Suspend != nil {
		discovered.Suspend = *cj.Spec.Suspend
	}

	// Parse timestamps
	if cj.Status.LastScheduleTime != nil {
		if t, err := time.Parse(time.RFC3339, *cj.Status.LastScheduleTime); err == nil {
			discovered.LastScheduleTime = &t
		}
	}
	if cj.Status.LastSuccessfulTime != nil {
		if t, err := time.Parse(time.RFC3339, *cj.Status.LastSuccessfulTime); err == nil {
			discovered.LastSuccessfulTime = &t
		}
	}

	// Get container info
	if len(cj.Spec.JobTemplate.Spec.Template.Spec.Containers) > 0 {
		container := cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
		discovered.Image = container.Image
		discovered.Command = container.Command
		discovered.Args = container.Args

		// Check for secrets/configmaps
		for _, env := range container.Env {
			if env.ValueFrom != nil {
				if env.ValueFrom.SecretKeyRef != nil {
					discovered.ConversionWarnings = append(discovered.ConversionWarnings,
						fmt.Sprintf("Uses Kubernetes Secret: %s", env.ValueFrom.SecretKeyRef.Name))
				}
				if env.ValueFrom.ConfigMapKeyRef != nil {
					discovered.ConversionWarnings = append(discovered.ConversionWarnings,
						fmt.Sprintf("Uses Kubernetes ConfigMap: %s", env.ValueFrom.ConfigMapKeyRef.Name))
				}
			}
		}
		for _, envFrom := range container.EnvFrom {
			if envFrom.SecretRef != nil {
				discovered.ConversionWarnings = append(discovered.ConversionWarnings,
					fmt.Sprintf("Uses Kubernetes Secret: %s", envFrom.SecretRef.Name))
			}
			if envFrom.ConfigMapRef != nil {
				discovered.ConversionWarnings = append(discovered.ConversionWarnings,
					fmt.Sprintf("Uses Kubernetes ConfigMap: %s", envFrom.ConfigMapRef.Name))
			}
		}
	}

	// Determine if adapter is needed
	discovered.RequiresAdapter = discovered.Image != ""

	// Generate converted job
	discovered.ConvertedJob = d.convertToChronosJob(discovered, nil)

	return discovered
}

func (d *KubernetesDiscovery) convertToChronosJob(discovered *DiscoveredCronJob, adapterConfig *WebhookAdapterConfig) *models.Job {
	job := &models.Job{
		ID:          uuid.New().String(),
		Name:        fmt.Sprintf("%s-%s", discovered.Namespace, discovered.Name),
		Description: fmt.Sprintf("Migrated from Kubernetes CronJob %s/%s", discovered.Namespace, discovered.Name),
		Schedule:    discovered.Schedule,
		Timezone:    discovered.Timezone,
		Enabled:     !discovered.Suspend,
		Tags: map[string]string{
			"migration_source":   "kubernetes_cronjob",
			"k8s_namespace":      discovered.Namespace,
			"k8s_name":           discovered.Name,
			"k8s_uid":            discovered.UID,
		},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		Version:   1,
		Namespace: discovered.Namespace,
	}

	// Copy labels (with prefix to avoid conflicts)
	for k, v := range discovered.Labels {
		job.Tags["label_"+k] = v
	}

	// Map concurrency policy
	switch discovered.ConcurrencyPolicy {
	case "Forbid":
		job.Concurrency = models.ConcurrencyForbid
	case "Replace":
		job.Concurrency = models.ConcurrencyReplace
	default:
		job.Concurrency = models.ConcurrencyAllow
	}

	// Set retry policy
	if discovered.BackoffLimit != nil {
		job.RetryPolicy = &models.RetryPolicy{
			MaxAttempts:     int(*discovered.BackoffLimit),
			InitialInterval: models.Duration(time.Second),
			MaxInterval:     models.Duration(time.Minute),
			Multiplier:      2.0,
		}
	} else {
		job.RetryPolicy = models.DefaultRetryPolicy()
	}

	// Build webhook config
	if discovered.RequiresAdapter {
		baseURL := "http://chronos-k8s-adapter:8081"
		if adapterConfig != nil && adapterConfig.BaseURL != "" {
			baseURL = adapterConfig.BaseURL
		}

		// Build command string for adapter
		cmdParts := append(discovered.Command, discovered.Args...)
		cmdStr := strings.Join(cmdParts, " ")

		job.Webhook = &models.WebhookConfig{
			URL:    fmt.Sprintf("%s/execute", baseURL),
			Method: "POST",
			Headers: map[string]string{
				"Content-Type":         "application/json",
				"X-Original-Image":     discovered.Image,
				"X-Original-Namespace": discovered.Namespace,
				"X-Original-Name":      discovered.Name,
				"X-Migration-Source":   "kubernetes-cronjob",
			},
			Body: fmt.Sprintf(`{
  "image": %q,
  "command": %q,
  "namespace": %q,
  "name": %q,
  "labels": {"app": "chronos-migrated", "source-cronjob": %q}
}`, discovered.Image, cmdStr, discovered.Namespace, discovered.Name, discovered.Name),
		}

		if adapterConfig != nil && adapterConfig.AuthToken != "" {
			job.Webhook.Auth = &models.AuthConfig{
				Type:  models.AuthTypeBearer,
				Token: adapterConfig.AuthToken,
			}
		}
	} else {
		// Job doesn't have a container (rare but possible)
		job.Webhook = &models.WebhookConfig{
			URL:    "http://localhost:8080/placeholder",
			Method: "POST",
			Headers: map[string]string{
				"X-Migration-Source": "kubernetes-cronjob",
			},
		}
	}

	return job
}

// CreateMigrationPlan creates a migration plan from discovered CronJobs.
func (d *KubernetesDiscovery) CreateMigrationPlan(
	ctx context.Context,
	discoveryResult *KubernetesDiscoveryResult,
	adapterConfig *WebhookAdapterConfig,
	targetNamespace string,
) *MigrationPlan {
	plan := &MigrationPlan{
		ID:              uuid.New().String(),
		Name:            fmt.Sprintf("k8s-migration-%s", time.Now().Format("20060102-150405")),
		CreatedAt:       time.Now().UTC(),
		SourceCluster:   d.apiServer,
		TargetNamespace: targetNamespace,
		AdapterConfig:   adapterConfig,
		CronJobs:        make([]*MigrationPlanItem, 0, len(discoveryResult.CronJobs)),
		Status:          MigrationPlanPending,
	}

	for _, cj := range discoveryResult.CronJobs {
		item := &MigrationPlanItem{
			SourceName:      cj.Name,
			SourceNamespace: cj.Namespace,
			TargetJob:       d.convertToChronosJob(cj, adapterConfig),
			Action:          "migrate",
			Warnings:        cj.ConversionWarnings,
		}

		// Override namespace if specified
		if targetNamespace != "" {
			item.TargetJob.Namespace = targetNamespace
		}

		// Determine if manual intervention is needed
		if len(cj.ConversionWarnings) > 3 {
			item.Action = "manual"
			item.Reason = "Too many conversion warnings - manual review recommended"
		}

		plan.CronJobs = append(plan.CronJobs, item)
	}

	return plan
}

// MigrationComparison compares source CronJobs with existing Chronos jobs.
type MigrationComparison struct {
	SourceCronJob    *DiscoveredCronJob `json:"source_cronjob"`
	ExistingJob      *models.Job        `json:"existing_job,omitempty"`
	Status           string             `json:"status"` // new, updated, identical, conflict
	Differences      []string           `json:"differences,omitempty"`
	RecommendedAction string            `json:"recommended_action"`
}

// CompareMigration compares discovered CronJobs with existing Chronos jobs.
func CompareMigration(discovered []*DiscoveredCronJob, existing []*models.Job) []*MigrationComparison {
	comparisons := make([]*MigrationComparison, 0, len(discovered))

	// Build lookup map for existing jobs by migration source tags
	existingBySource := make(map[string]*models.Job)
	for _, job := range existing {
		if job.Tags != nil {
			if ns, ok := job.Tags["k8s_namespace"]; ok {
				if name, ok := job.Tags["k8s_name"]; ok {
					key := fmt.Sprintf("%s/%s", ns, name)
					existingBySource[key] = job
				}
			}
		}
	}

	for _, cj := range discovered {
		comparison := &MigrationComparison{
			SourceCronJob: cj,
			Differences:   make([]string, 0),
		}

		key := fmt.Sprintf("%s/%s", cj.Namespace, cj.Name)
		if existingJob, found := existingBySource[key]; found {
			comparison.ExistingJob = existingJob

			// Check for differences
			if existingJob.Schedule != cj.Schedule {
				comparison.Differences = append(comparison.Differences,
					fmt.Sprintf("Schedule: %s -> %s", existingJob.Schedule, cj.Schedule))
			}
			if existingJob.Enabled == cj.Suspend {
				comparison.Differences = append(comparison.Differences,
					fmt.Sprintf("Enabled: %v -> %v", existingJob.Enabled, !cj.Suspend))
			}

			if len(comparison.Differences) == 0 {
				comparison.Status = "identical"
				comparison.RecommendedAction = "skip"
			} else {
				comparison.Status = "updated"
				comparison.RecommendedAction = "update"
			}
		} else {
			comparison.Status = "new"
			comparison.RecommendedAction = "create"
		}

		comparisons = append(comparisons, comparison)
	}

	return comparisons
}

// ExportMigrationPlanYAML exports the migration plan as YAML.
func ExportMigrationPlanYAML(plan *MigrationPlan) ([]byte, error) {
	return yaml.Marshal(plan)
}

// ExportMigrationPlanJSON exports the migration plan as JSON.
func ExportMigrationPlanJSON(plan *MigrationPlan) ([]byte, error) {
	return json.MarshalIndent(plan, "", "  ")
}
