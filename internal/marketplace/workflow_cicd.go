// Package marketplace provides workflow templates for Chronos.
// This file contains CI/CD workflow templates.
package marketplace

import "time"

// CICDWorkflowTemplates returns CI/CD workflow templates.
func CICDWorkflowTemplates() []*WorkflowTemplate {
	return []*WorkflowTemplate{
		ciCDDeploymentTemplate(),
	}
}

func ciCDDeploymentTemplate() *WorkflowTemplate {
	return &WorkflowTemplate{
		ID:          "ci-cd-deployment",
		Name:        "CI/CD Deployment Pipeline",
		Description: "Build, test, and deploy application to multiple environments",
		Category:    WorkflowCategoryCI,
		Author:      "Chronos",
		Version:     "1.0.0",
		Tags:        []string{"ci", "cd", "deployment", "kubernetes", "docker"},
		Icon:        "🚀",
		License:     "Apache-2.0",
		Verified:    true,
		Featured:    true,
		Steps: []WorkflowStepTemplate{
			{
				ID:          "checkout",
				Name:        "Checkout Code",
				Description: "Clone the repository",
				Type:        StepTypeHTTP,
				Config: map[string]interface{}{
					"url":    "{{CI_SERVER_URL}}/api/checkout",
					"method": "POST",
					"body":   `{"repo": "{{REPO_URL}}", "branch": "{{BRANCH}}"}`,
				},
			},
			{
				ID:           "build",
				Name:         "Build Application",
				Description:  "Build Docker image",
				Type:         StepTypeHTTP,
				Dependencies: []string{"checkout"},
				Config: map[string]interface{}{
					"url":    "{{CI_SERVER_URL}}/api/build",
					"method": "POST",
					"body":   `{"image": "{{IMAGE_NAME}}", "tag": "{{VERSION}}"}`,
				},
				Timeout: "15m",
			},
			{
				ID:           "test-unit",
				Name:         "Run Unit Tests",
				Description:  "Execute unit test suite",
				Type:         StepTypeHTTP,
				Dependencies: []string{"build"},
				Config: map[string]interface{}{
					"url":    "{{CI_SERVER_URL}}/api/test",
					"method": "POST",
					"body":   `{"type": "unit"}`,
				},
				Timeout: "10m",
			},
			{
				ID:           "test-integration",
				Name:         "Run Integration Tests",
				Description:  "Execute integration test suite",
				Type:         StepTypeHTTP,
				Dependencies: []string{"build"},
				Config: map[string]interface{}{
					"url":    "{{CI_SERVER_URL}}/api/test",
					"method": "POST",
					"body":   `{"type": "integration"}`,
				},
				Timeout: "20m",
			},
			{
				ID:           "security-scan",
				Name:         "Security Scan",
				Description:  "Run security vulnerability scan",
				Type:         StepTypeHTTP,
				Dependencies: []string{"build"},
				Config: map[string]interface{}{
					"url":    "{{SECURITY_SCANNER_URL}}/scan",
					"method": "POST",
					"body":   `{"image": "{{IMAGE_NAME}}:{{VERSION}}"}`,
				},
			},
			{
				ID:           "deploy-staging",
				Name:         "Deploy to Staging",
				Description:  "Deploy to staging environment",
				Type:         StepTypeHTTP,
				Dependencies: []string{"test-unit", "test-integration", "security-scan"},
				Config: map[string]interface{}{
					"url":    "{{K8S_API_URL}}/apis/apps/v1/namespaces/staging/deployments/{{APP_NAME}}",
					"method": "PATCH",
					"headers": map[string]string{
						"Authorization": "Bearer {{K8S_TOKEN}}",
						"Content-Type":  "application/strategic-merge-patch+json",
					},
					"body": `{"spec":{"template":{"spec":{"containers":[{"name":"{{APP_NAME}}","image":"{{IMAGE_NAME}}:{{VERSION}}"}]}}}}`,
				},
			},
			{
				ID:           "smoke-test",
				Name:         "Staging Smoke Test",
				Description:  "Run smoke tests against staging",
				Type:         StepTypeHTTP,
				Dependencies: []string{"deploy-staging"},
				Config: map[string]interface{}{
					"url":    "{{STAGING_URL}}/health",
					"method": "GET",
				},
				RetryPolicy: &RetryPolicyTemplate{
					MaxAttempts:     5,
					InitialInterval: "10s",
					Multiplier:      2.0,
				},
			},
			{
				ID:           "approval",
				Name:         "Production Approval",
				Description:  "Wait for manual approval to deploy to production",
				Type:         StepTypeApproval,
				Dependencies: []string{"smoke-test"},
				Config: map[string]interface{}{
					"approvers": []string{"{{APPROVER_GROUP}}"},
					"timeout":   "24h",
				},
			},
			{
				ID:           "deploy-production",
				Name:         "Deploy to Production",
				Description:  "Deploy to production environment",
				Type:         StepTypeHTTP,
				Dependencies: []string{"approval"},
				Config: map[string]interface{}{
					"url":    "{{K8S_API_URL}}/apis/apps/v1/namespaces/production/deployments/{{APP_NAME}}",
					"method": "PATCH",
					"headers": map[string]string{
						"Authorization": "Bearer {{K8S_TOKEN}}",
						"Content-Type":  "application/strategic-merge-patch+json",
					},
					"body": `{"spec":{"template":{"spec":{"containers":[{"name":"{{APP_NAME}}","image":"{{IMAGE_NAME}}:{{VERSION}}"}]}}}}`,
				},
			},
			{
				ID:           "notify-deployment",
				Name:         "Notify Deployment",
				Description:  "Send deployment notification",
				Type:         StepTypeNotification,
				Dependencies: []string{"deploy-production"},
				Config: map[string]interface{}{
					"channel": "slack",
					"webhook": "{{SLACK_WEBHOOK}}",
					"message": "🚀 {{APP_NAME}} v{{VERSION}} deployed to production!",
				},
			},
		},
		Triggers: []TriggerTemplate{
			{
				Type:    TriggerTypeWebhook,
				Name:    "Git Push",
				Config:  map[string]interface{}{"path": "/webhooks/deploy"},
				Enabled: true,
			},
			{
				Type:    TriggerTypeManual,
				Name:    "Manual Deploy",
				Config:  map[string]interface{}{},
				Enabled: true,
			},
		},
		Variables: []TemplateVariable{
			{Name: "CI_SERVER_URL", Description: "CI server base URL", Type: "string", Required: true},
			{Name: "REPO_URL", Description: "Git repository URL", Type: "string", Required: true},
			{Name: "BRANCH", Description: "Git branch to deploy", Type: "string", Default: "main"},
			{Name: "IMAGE_NAME", Description: "Docker image name", Type: "string", Required: true},
			{Name: "VERSION", Description: "Version/tag to deploy", Type: "string", Required: true},
			{Name: "APP_NAME", Description: "Application name", Type: "string", Required: true},
			{Name: "K8S_API_URL", Description: "Kubernetes API server URL", Type: "string", Required: true},
			{Name: "K8S_TOKEN", Description: "Kubernetes service account token", Type: "secret", Required: true},
			{Name: "STAGING_URL", Description: "Staging environment URL", Type: "string", Required: true},
			{Name: "SECURITY_SCANNER_URL", Description: "Security scanner URL", Type: "string", Required: true},
			{Name: "APPROVER_GROUP", Description: "Group allowed to approve production", Type: "string", Default: "platform-team"},
			{Name: "SLACK_WEBHOOK", Description: "Slack webhook for notifications", Type: "secret", Required: true},
		},
		Readme: `# CI/CD Deployment Pipeline

Complete CI/CD pipeline with build, test, security scan, and multi-environment deployment.

## Flow

Checkout -> Build -> Tests + Security Scan -> Deploy Staging -> Smoke Test -> Approval -> Deploy Prod -> Notify

## Features

- Parallel unit and integration tests
- Security vulnerability scanning
- Staging environment validation
- Manual approval gate for production
- Slack notifications
`,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}
