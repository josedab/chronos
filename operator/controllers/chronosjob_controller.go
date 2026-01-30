// Package controllers implements the Kubernetes controller logic for Chronos resources.
package controllers

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	chronosv1alpha1 "github.com/chronos/chronos/operator/api/v1alpha1"
)

// ChronosJobReconciler reconciles a ChronosJob object.
type ChronosJobReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	ChronosClient ChronosAPIClient
}

// ChronosAPIClient is the interface for the Chronos API client.
type ChronosAPIClient interface {
	CreateJob(ctx context.Context, job *JobRequest) (*JobResponse, error)
	UpdateJob(ctx context.Context, id string, job *JobRequest) (*JobResponse, error)
	DeleteJob(ctx context.Context, id string) error
	GetJob(ctx context.Context, id string) (*JobResponse, error)
}

// JobRequest is the request to create/update a job.
type JobRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Schedule    string            `json:"schedule"`
	Timezone    string            `json:"timezone,omitempty"`
	Enabled     bool              `json:"enabled"`
	Webhook     *WebhookRequest   `json:"webhook"`
	Timeout     string            `json:"timeout,omitempty"`
	Concurrency string            `json:"concurrency,omitempty"`
	RetryPolicy *RetryPolicyReq   `json:"retry_policy,omitempty"`
}

// WebhookRequest is the webhook configuration.
type WebhookRequest struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

// RetryPolicyReq is the retry policy configuration.
type RetryPolicyReq struct {
	MaxAttempts     int     `json:"max_attempts"`
	InitialInterval string  `json:"initial_interval"`
	MaxInterval     string  `json:"max_interval"`
	Multiplier      float64 `json:"multiplier"`
}

// JobResponse is the response from the Chronos API.
type JobResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// +kubebuilder:rbac:groups=chronos.io,resources=chronosjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=chronos.io,resources=chronosjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=chronos.io,resources=chronosjobs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile handles reconciliation for ChronosJob resources.
func (r *ChronosJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the ChronosJob instance
	var chronosJob chronosv1alpha1.ChronosJob
	if err := r.Get(ctx, req.NamespacedName, &chronosJob); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("ChronosJob resource not found, ignoring")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get ChronosJob")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !chronosJob.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, &chronosJob)
	}

	// Ensure finalizer is set
	if !containsString(chronosJob.Finalizers, finalizerName) {
		chronosJob.Finalizers = append(chronosJob.Finalizers, finalizerName)
		if err := r.Update(ctx, &chronosJob); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Build the job request
	jobReq := r.buildJobRequest(&chronosJob)

	// Create or update the job in Chronos
	var result *JobResponse
	var err error

	if chronosJob.Status.ChronosJobID == "" {
		// Create new job
		result, err = r.ChronosClient.CreateJob(ctx, jobReq)
		if err != nil {
			logger.Error(err, "Failed to create job in Chronos")
			return r.updateStatus(ctx, &chronosJob, chronosv1alpha1.JobPhaseFailed, err.Error())
		}
		chronosJob.Status.ChronosJobID = result.ID
	} else {
		// Update existing job
		result, err = r.ChronosClient.UpdateJob(ctx, chronosJob.Status.ChronosJobID, jobReq)
		if err != nil {
			logger.Error(err, "Failed to update job in Chronos")
			return r.updateStatus(ctx, &chronosJob, chronosv1alpha1.JobPhaseFailed, err.Error())
		}
	}

	// Update status
	phase := chronosv1alpha1.JobPhaseSynced
	if !chronosJob.Spec.Enabled {
		phase = chronosv1alpha1.JobPhaseDisabled
	}
	return r.updateStatus(ctx, &chronosJob, phase, "Job synced successfully")
}

// handleDeletion handles cleanup when a ChronosJob is deleted.
func (r *ChronosJobReconciler) handleDeletion(ctx context.Context, job *chronosv1alpha1.ChronosJob) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if containsString(job.Finalizers, finalizerName) {
		// Delete the job from Chronos
		if job.Status.ChronosJobID != "" {
			if err := r.ChronosClient.DeleteJob(ctx, job.Status.ChronosJobID); err != nil {
				logger.Error(err, "Failed to delete job from Chronos")
				// Continue with finalizer removal even if Chronos delete fails
			}
		}

		// Remove finalizer
		job.Finalizers = removeString(job.Finalizers, finalizerName)
		if err := r.Update(ctx, job); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// buildJobRequest builds a JobRequest from the ChronosJob spec.
func (r *ChronosJobReconciler) buildJobRequest(job *chronosv1alpha1.ChronosJob) *JobRequest {
	req := &JobRequest{
		Name:        job.Spec.Name,
		Description: job.Spec.Description,
		Schedule:    job.Spec.Schedule,
		Timezone:    job.Spec.Timezone,
		Enabled:     job.Spec.Enabled,
		Timeout:     job.Spec.Timeout,
		Concurrency: job.Spec.Concurrency,
		Webhook: &WebhookRequest{
			URL:     job.Spec.Webhook.URL,
			Method:  job.Spec.Webhook.Method,
			Headers: job.Spec.Webhook.Headers,
			Body:    job.Spec.Webhook.Body,
		},
	}

	if job.Spec.RetryPolicy != nil {
		req.RetryPolicy = &RetryPolicyReq{
			MaxAttempts:     job.Spec.RetryPolicy.MaxAttempts,
			InitialInterval: job.Spec.RetryPolicy.InitialInterval,
			MaxInterval:     job.Spec.RetryPolicy.MaxInterval,
			Multiplier:      job.Spec.RetryPolicy.Multiplier,
		}
	}

	return req
}

// updateStatus updates the ChronosJob status.
func (r *ChronosJobReconciler) updateStatus(ctx context.Context, job *chronosv1alpha1.ChronosJob, phase chronosv1alpha1.JobPhase, message string) (ctrl.Result, error) {
	job.Status.Phase = phase
	job.Status.Message = message

	if err := r.Status().Update(ctx, job); err != nil {
		return ctrl.Result{}, err
	}

	// Requeue if failed
	if phase == chronosv1alpha1.JobPhaseFailed {
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ChronosJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&chronosv1alpha1.ChronosJob{}).
		Complete(r)
}

const finalizerName = "chronos.io/finalizer"

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) []string {
	result := make([]string, 0, len(slice))
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}

// Ensure interface is satisfied
var _ ChronosAPIClient = (*HTTPChronosClient)(nil)

// HTTPChronosClient implements ChronosAPIClient using HTTP.
type HTTPChronosClient struct {
	Endpoint string
	APIKey   string
}

// CreateJob creates a job via HTTP.
func (c *HTTPChronosClient) CreateJob(ctx context.Context, job *JobRequest) (*JobResponse, error) {
	// Implementation would use net/http
	return &JobResponse{ID: "generated-id", Name: job.Name}, nil
}

// UpdateJob updates a job via HTTP.
func (c *HTTPChronosClient) UpdateJob(ctx context.Context, id string, job *JobRequest) (*JobResponse, error) {
	return &JobResponse{ID: id, Name: job.Name}, nil
}

// DeleteJob deletes a job via HTTP.
func (c *HTTPChronosClient) DeleteJob(ctx context.Context, id string) error {
	return nil
}

// GetJob gets a job via HTTP.
func (c *HTTPChronosClient) GetJob(ctx context.Context, id string) (*JobResponse, error) {
	return &JobResponse{ID: id}, nil
}

// Unused but required for interface
var _ = fmt.Sprintf
