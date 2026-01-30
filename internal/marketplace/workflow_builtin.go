// Package marketplace provides workflow templates for Chronos.
package marketplace

// BuiltInWorkflowTemplates returns all built-in workflow templates.
// Templates are organized into separate files by category for maintainability.
func BuiltInWorkflowTemplates() []*WorkflowTemplate {
	templates := make([]*WorkflowTemplate, 0, 10)

	// ETL and Data workflows
	templates = append(templates, ETLWorkflowTemplates()...)

	// CI/CD workflows
	templates = append(templates, CICDWorkflowTemplates()...)

	// Operations workflows (incident response, maintenance, monitoring)
	templates = append(templates, OperationsWorkflowTemplates()...)

	// Notification and reporting workflows
	templates = append(templates, NotificationWorkflowTemplates()...)

	return templates
}