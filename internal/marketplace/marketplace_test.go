package marketplace

import (
	"context"
	"testing"
)

func TestNewMarketplace(t *testing.T) {
	m := NewMarketplace()
	if m == nil {
		t.Fatal("expected non-nil marketplace")
	}

	// Should have built-in templates
	templates, err := m.ListTemplates(context.Background(), "", "")
	if err != nil {
		t.Fatalf("failed to list templates: %v", err)
	}
	if len(templates) == 0 {
		t.Error("expected built-in templates")
	}
}

func TestListTemplates(t *testing.T) {
	m := NewMarketplace()
	ctx := context.Background()

	tests := []struct {
		name     string
		category Category
		search   string
		wantMin  int
	}{
		{"all templates", "", "", 1},
		{"backup category", CategoryBackup, "", 1},
		{"monitoring category", CategoryMonitoring, "", 1},
		{"search backup", "", "backup", 1},
		{"search ssl", "", "ssl", 1},
		{"nonexistent", "", "xyznonexistent", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			templates, err := m.ListTemplates(ctx, tt.category, tt.search)
			if err != nil {
				t.Fatalf("ListTemplates failed: %v", err)
			}
			if len(templates) < tt.wantMin {
				t.Errorf("got %d templates, want at least %d", len(templates), tt.wantMin)
			}
		})
	}
}

func TestGetTemplate(t *testing.T) {
	m := NewMarketplace()
	ctx := context.Background()

	// Get existing template
	tpl, err := m.GetTemplate(ctx, "database-backup")
	if err != nil {
		t.Fatalf("GetTemplate failed: %v", err)
	}
	if tpl.Name != "Database Backup" {
		t.Errorf("got name %q, want %q", tpl.Name, "Database Backup")
	}

	// Get nonexistent template
	_, err = m.GetTemplate(ctx, "nonexistent")
	if err != ErrTemplateNotFound {
		t.Errorf("expected ErrTemplateNotFound, got %v", err)
	}
}

func TestInstantiateTemplate(t *testing.T) {
	m := NewMarketplace()
	ctx := context.Background()

	// Instantiate with all required variables
	req := &InstantiateRequest{
		TemplateID: "database-backup",
		Name:       "my-backup-job",
		Variables: map[string]string{
			"backup_endpoint":  "https://backup.example.com",
			"database_name":    "production_db",
			"storage_provider": "s3",
		},
		Namespace: "production",
	}

	config, err := m.InstantiateTemplate(ctx, req)
	if err != nil {
		t.Fatalf("InstantiateTemplate failed: %v", err)
	}

	if config.Name != "my-backup-job" {
		t.Errorf("got name %q, want %q", config.Name, "my-backup-job")
	}
	if config.Webhook.URL != "https://backup.example.com/backup" {
		t.Errorf("got webhook URL %q, want %q", config.Webhook.URL, "https://backup.example.com/backup")
	}
	if config.Namespace != "production" {
		t.Errorf("got namespace %q, want %q", config.Namespace, "production")
	}

	// Check that template tags are added
	if config.Tags["template"] != "database-backup" {
		t.Errorf("expected template tag to be set")
	}
}

func TestInstantiateTemplateMissingRequired(t *testing.T) {
	m := NewMarketplace()
	ctx := context.Background()

	// Missing required variable
	req := &InstantiateRequest{
		TemplateID: "database-backup",
		Name:       "my-backup-job",
		Variables: map[string]string{
			"backup_endpoint": "https://backup.example.com",
			// Missing database_name which is required
		},
	}

	_, err := m.InstantiateTemplate(ctx, req)
	if err == nil {
		t.Error("expected error for missing required variable")
	}
}

func TestInstantiateTemplateNotFound(t *testing.T) {
	m := NewMarketplace()
	ctx := context.Background()

	req := &InstantiateRequest{
		TemplateID: "nonexistent",
		Name:       "my-job",
		Variables:  map[string]string{},
	}

	_, err := m.InstantiateTemplate(ctx, req)
	if err != ErrTemplateNotFound {
		t.Errorf("expected ErrTemplateNotFound, got %v", err)
	}
}

func TestAddTemplate(t *testing.T) {
	m := NewMarketplace()
	ctx := context.Background()

	// Add custom template
	tpl := &Template{
		ID:          "custom-job",
		Name:        "Custom Job",
		Description: "A custom job template",
		Category:    CategoryIntegration,
		Author:      "User",
		Version:     "1.0.0",
		Schedule:    "0 * * * *",
		Webhook: WebhookTemplate{
			URL:    "{{endpoint}}",
			Method: "POST",
		},
		Variables: []TemplateVariable{
			{Name: "endpoint", Type: "string", Required: true},
		},
	}

	err := m.AddTemplate(ctx, tpl)
	if err != nil {
		t.Fatalf("AddTemplate failed: %v", err)
	}

	// Verify it was added
	got, err := m.GetTemplate(ctx, "custom-job")
	if err != nil {
		t.Fatalf("GetTemplate failed: %v", err)
	}
	if got.Name != "Custom Job" {
		t.Errorf("got name %q, want %q", got.Name, "Custom Job")
	}

	// Try to add duplicate
	err = m.AddTemplate(ctx, tpl)
	if err != ErrTemplateExists {
		t.Errorf("expected ErrTemplateExists, got %v", err)
	}
}

func TestAddTemplateInvalid(t *testing.T) {
	m := NewMarketplace()
	ctx := context.Background()

	// Missing ID
	err := m.AddTemplate(ctx, &Template{Name: "Test"})
	if err != ErrInvalidTemplate {
		t.Errorf("expected ErrInvalidTemplate, got %v", err)
	}

	// Missing Name
	err = m.AddTemplate(ctx, &Template{ID: "test"})
	if err != ErrInvalidTemplate {
		t.Errorf("expected ErrInvalidTemplate, got %v", err)
	}
}

func TestAllCategories(t *testing.T) {
	categories := AllCategories()
	if len(categories) == 0 {
		t.Error("expected at least one category")
	}

	// Check specific categories exist
	found := make(map[Category]bool)
	for _, c := range categories {
		found[c] = true
	}

	expected := []Category{CategoryBackup, CategoryMonitoring, CategorySecurity}
	for _, c := range expected {
		if !found[c] {
			t.Errorf("expected category %s to exist", c)
		}
	}
}

func TestDownloadsIncrement(t *testing.T) {
	m := NewMarketplace()
	ctx := context.Background()

	// Get initial downloads
	tpl, _ := m.GetTemplate(ctx, "database-backup")
	initialDownloads := tpl.Downloads

	// Instantiate
	req := &InstantiateRequest{
		TemplateID: "database-backup",
		Name:       "test-job",
		Variables: map[string]string{
			"backup_endpoint":  "https://example.com",
			"database_name":    "db",
			"storage_provider": "s3",
		},
	}
	m.InstantiateTemplate(ctx, req)

	// Check downloads incremented
	tpl, _ = m.GetTemplate(ctx, "database-backup")
	if tpl.Downloads != initialDownloads+1 {
		t.Errorf("got downloads %d, want %d", tpl.Downloads, initialDownloads+1)
	}
}
