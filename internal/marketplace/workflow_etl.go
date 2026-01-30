// Package marketplace provides workflow templates for Chronos.
// This file contains ETL and data-related workflow templates.
package marketplace

import "time"

// ETLWorkflowTemplates returns ETL and data pipeline workflow templates.
func ETLWorkflowTemplates() []*WorkflowTemplate {
	return []*WorkflowTemplate{
		etlDataPipelineTemplate(),
		dataQualityPipelineTemplate(),
	}
}

func etlDataPipelineTemplate() *WorkflowTemplate {
	return &WorkflowTemplate{
		ID:          "etl-data-pipeline",
		Name:        "ETL Data Pipeline",
		Description: "Extract data from sources, transform it, and load into a data warehouse",
		Category:    WorkflowCategoryETL,
		Author:      "Chronos",
		Version:     "1.0.0",
		Tags:        []string{"etl", "data", "pipeline", "warehouse", "analytics"},
		Icon:        "🔄",
		License:     "Apache-2.0",
		Verified:    true,
		Featured:    true,
		Steps: []WorkflowStepTemplate{
			{
				ID:          "extract-source-a",
				Name:        "Extract from Source A",
				Description: "Pull data from the primary data source",
				Type:        StepTypeHTTP,
				Config: map[string]interface{}{
					"url":    "{{SOURCE_A_URL}}/api/export",
					"method": "POST",
					"headers": map[string]string{
						"Authorization": "Bearer {{SOURCE_A_TOKEN}}",
					},
					"body": `{"start_date": "{{START_DATE}}", "end_date": "{{END_DATE}}"}`,
				},
				Timeout: "10m",
			},
			{
				ID:          "extract-source-b",
				Name:        "Extract from Source B",
				Description: "Pull data from the secondary data source",
				Type:        StepTypeHTTP,
				Config: map[string]interface{}{
					"url":    "{{SOURCE_B_URL}}/data",
					"method": "GET",
					"headers": map[string]string{
						"X-API-Key": "{{SOURCE_B_KEY}}",
					},
				},
				Timeout: "10m",
			},
			{
				ID:           "transform",
				Name:         "Transform Data",
				Description:  "Clean, normalize, and join extracted data",
				Type:         StepTypeHTTP,
				Dependencies: []string{"extract-source-a", "extract-source-b"},
				Config: map[string]interface{}{
					"url":    "{{TRANSFORM_SERVICE_URL}}/transform",
					"method": "POST",
					"body":   `{"source_a": "{{steps.extract-source-a.output}}", "source_b": "{{steps.extract-source-b.output}}"}`,
				},
				Timeout: "30m",
			},
			{
				ID:           "validate",
				Name:         "Validate Data Quality",
				Description:  "Run data quality checks",
				Type:         StepTypeHTTP,
				Dependencies: []string{"transform"},
				Config: map[string]interface{}{
					"url":    "{{VALIDATION_SERVICE_URL}}/validate",
					"method": "POST",
				},
				OnFailure: []string{"notify-failure"},
			},
			{
				ID:           "load",
				Name:         "Load to Warehouse",
				Description:  "Insert transformed data into the data warehouse",
				Type:         StepTypeHTTP,
				Dependencies: []string{"validate"},
				Config: map[string]interface{}{
					"url":    "{{WAREHOUSE_URL}}/load",
					"method": "POST",
					"headers": map[string]string{
						"Authorization": "Bearer {{WAREHOUSE_TOKEN}}",
					},
				},
				Timeout: "60m",
				RetryPolicy: &RetryPolicyTemplate{
					MaxAttempts:     3,
					InitialInterval: "30s",
					Multiplier:      2.0,
				},
			},
			{
				ID:           "notify-success",
				Name:         "Notify Success",
				Description:  "Send success notification",
				Type:         StepTypeNotification,
				Dependencies: []string{"load"},
				Config: map[string]interface{}{
					"channel": "slack",
					"webhook": "{{SLACK_WEBHOOK}}",
					"message": "ETL pipeline completed successfully. Processed {{steps.load.output.rows}} rows.",
				},
			},
			{
				ID:          "notify-failure",
				Name:        "Notify Failure",
				Description: "Send failure notification",
				Type:        StepTypeNotification,
				Config: map[string]interface{}{
					"channel": "slack",
					"webhook": "{{SLACK_WEBHOOK}}",
					"message": "⚠️ ETL pipeline failed: {{error.message}}",
				},
			},
		},
		Triggers: []TriggerTemplate{
			{
				Type: TriggerTypeCron,
				Name: "Daily at 2 AM",
				Config: map[string]interface{}{
					"schedule": "0 2 * * *",
					"timezone": "UTC",
				},
				Enabled: true,
			},
			{
				Type:    TriggerTypeManual,
				Name:    "Manual trigger",
				Config:  map[string]interface{}{},
				Enabled: true,
			},
		},
		Variables: []TemplateVariable{
			{Name: "SOURCE_A_URL", Description: "Primary data source URL", Type: "string", Required: true},
			{Name: "SOURCE_A_TOKEN", Description: "Primary source auth token", Type: "secret", Required: true},
			{Name: "SOURCE_B_URL", Description: "Secondary data source URL", Type: "string", Required: true},
			{Name: "SOURCE_B_KEY", Description: "Secondary source API key", Type: "secret", Required: true},
			{Name: "TRANSFORM_SERVICE_URL", Description: "Data transformation service URL", Type: "string", Required: true},
			{Name: "VALIDATION_SERVICE_URL", Description: "Data validation service URL", Type: "string", Required: true},
			{Name: "WAREHOUSE_URL", Description: "Data warehouse URL", Type: "string", Required: true},
			{Name: "WAREHOUSE_TOKEN", Description: "Warehouse auth token", Type: "secret", Required: true},
			{Name: "SLACK_WEBHOOK", Description: "Slack notification webhook", Type: "secret", Required: true},
			{Name: "START_DATE", Description: "Data extraction start date", Type: "string", Default: "{{yesterday}}"},
			{Name: "END_DATE", Description: "Data extraction end date", Type: "string", Default: "{{today}}"},
		},
		Readme: `# ETL Data Pipeline

A complete ETL workflow that extracts data from multiple sources, transforms it, validates quality, and loads into a data warehouse.

## Architecture

Source A --> Transform --> Validate --> Load --> Notify
Source B -->

## Steps

1. **Extract from Source A/B** - Parallel extraction from two data sources
2. **Transform** - Clean, normalize, and join the data
3. **Validate** - Run data quality checks
4. **Load** - Insert into data warehouse
5. **Notify** - Send Slack notification

## Configuration

Set up all required variables before running. Ensure your transformation and validation services are deployed.
`,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func dataQualityPipelineTemplate() *WorkflowTemplate {
	return &WorkflowTemplate{
		ID:          "data-quality-pipeline",
		Name:        "Data Quality Pipeline",
		Description: "Automated data quality checks with alerting",
		Category:    WorkflowCategoryData,
		Author:      "Chronos",
		Version:     "1.0.0",
		Tags:        []string{"data-quality", "validation", "monitoring"},
		Icon:        "✅",
		License:     "Apache-2.0",
		Verified:    true,
		Steps: []WorkflowStepTemplate{
			{
				ID:          "run-checks",
				Name:        "Run Data Quality Checks",
				Type:        StepTypeHTTP,
				Description: "Execute data quality rules",
				Config: map[string]interface{}{
					"url":    "{{DQ_SERVICE_URL}}/checks/run",
					"method": "POST",
					"body":   `{"dataset": "{{DATASET}}", "rules": "{{RULES}}"}`,
				},
				Timeout: "30m",
			},
			{
				ID:           "evaluate-results",
				Name:         "Evaluate Results",
				Type:         StepTypeCondition,
				Dependencies: []string{"run-checks"},
				Config: map[string]interface{}{
					"expression": "{{steps.run-checks.output.pass_rate}} >= {{THRESHOLD}}",
				},
			},
			{
				ID:           "notify-success",
				Name:         "Notify Success",
				Type:         StepTypeNotification,
				Dependencies: []string{"evaluate-results"},
				Condition:    "{{steps.evaluate-results.output}} == true",
				Config: map[string]interface{}{
					"channel": "slack",
					"webhook": "{{SLACK_WEBHOOK}}",
					"message": "✅ Data quality checks passed ({{steps.run-checks.output.pass_rate}}%)",
				},
			},
			{
				ID:           "notify-failure",
				Name:         "Notify Failure",
				Type:         StepTypeNotification,
				Dependencies: []string{"evaluate-results"},
				Condition:    "{{steps.evaluate-results.output}} == false",
				Config: map[string]interface{}{
					"channel": "slack",
					"webhook": "{{SLACK_WEBHOOK}}",
					"message": "⚠️ Data quality checks failed ({{steps.run-checks.output.pass_rate}}%)",
				},
			},
		},
		Triggers: []TriggerTemplate{
			{
				Type:    TriggerTypeCron,
				Name:    "Hourly",
				Config:  map[string]interface{}{"schedule": "0 * * * *"},
				Enabled: true,
			},
		},
		Variables: []TemplateVariable{
			{Name: "DQ_SERVICE_URL", Description: "Data quality service URL", Type: "string", Required: true},
			{Name: "DATASET", Description: "Dataset to check", Type: "string", Required: true},
			{Name: "RULES", Description: "Quality rules to apply", Type: "string", Default: "all"},
			{Name: "THRESHOLD", Description: "Pass rate threshold (0-100)", Type: "number", Default: "95"},
			{Name: "SLACK_WEBHOOK", Description: "Slack webhook", Type: "secret", Required: true},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}
