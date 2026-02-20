package provider

import (
	"testing"
)

func TestNewJobResource(t *testing.T) {
	r := NewJobResource()
	if r == nil {
		t.Fatal("expected non-nil resource")
	}
}

func TestJobResourceModel_Fields(t *testing.T) {
	model := JobResourceModel{}

	// Verify all fields are zero-valued by default (types.String, types.Bool, etc.)
	if !model.ID.IsNull() && !model.ID.IsUnknown() && model.ID.ValueString() != "" {
		t.Error("expected empty ID")
	}
	if !model.Name.IsNull() && !model.Name.IsUnknown() && model.Name.ValueString() != "" {
		t.Error("expected empty Name")
	}
}

func TestNewNamespaceResource(t *testing.T) {
	r := NewNamespaceResource()
	if r == nil {
		t.Fatal("expected non-nil resource")
	}
}

func TestNewWorkflowResource(t *testing.T) {
	r := NewWorkflowResource()
	if r == nil {
		t.Fatal("expected non-nil resource")
	}
}

func TestNewDAGResource(t *testing.T) {
	r := NewDAGResource()
	if r == nil {
		t.Fatal("expected non-nil resource")
	}
}

func TestNewPolicyResource(t *testing.T) {
	r := NewPolicyResource()
	if r == nil {
		t.Fatal("expected non-nil resource")
	}
}

func TestNewTriggerResource(t *testing.T) {
	r := NewTriggerResource()
	if r == nil {
		t.Fatal("expected non-nil resource")
	}
}

func TestNew_Provider(t *testing.T) {
	factory := New("test")
	p := factory()
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestClient_Defaults(t *testing.T) {
	client := NewClient("http://localhost:8080", "test-key", 30)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.endpoint != "http://localhost:8080" {
		t.Errorf("expected endpoint http://localhost:8080, got %s", client.endpoint)
	}
	if client.apiKey != "test-key" {
		t.Errorf("expected api key test-key, got %s", client.apiKey)
	}
}
