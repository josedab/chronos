package geo

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func setupFederationAPI(t *testing.T) (*APIHandler, *chi.Mux) {
	t.Helper()
	cfg := DefaultFederationConfig()
	cfg.Enabled = true
	fm := NewFederationManager(cfg)
	handler := NewAPIHandler(fm)
	r := chi.NewRouter()
	r.Mount("/federation", handler.Routes())
	return handler, r
}

func TestFederationAPI_AddAndListClusters(t *testing.T) {
	_, router := setupFederationAPI(t)

	body, _ := json.Marshal(FederatedCluster{
		ID:        "us-east-1",
		Name:      "US East",
		Region:    "us-east-1",
		Endpoints: []string{"https://chronos-east.example.com"},
	})

	req := httptest.NewRequest("POST", "/federation/clusters", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// List
	req = httptest.NewRequest("GET", "/federation/clusters", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	data := resp["data"].(map[string]interface{})
	if data["total"].(float64) != 1 {
		t.Errorf("expected 1 cluster, got %v", data["total"])
	}
}

func TestFederationAPI_GetCluster(t *testing.T) {
	handler, router := setupFederationAPI(t)

	handler.federation.AddCluster(&FederatedCluster{
		ID: "eu-west-1", Name: "EU West", Region: "eu-west-1",
		Endpoints: []string{"https://example.com"},
	})

	req := httptest.NewRequest("GET", "/federation/clusters/eu-west-1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestFederationAPI_GetCluster_NotFound(t *testing.T) {
	_, router := setupFederationAPI(t)

	req := httptest.NewRequest("GET", "/federation/clusters/nonexistent", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestFederationAPI_RemoveCluster(t *testing.T) {
	handler, router := setupFederationAPI(t)

	handler.federation.AddCluster(&FederatedCluster{
		ID: "remove-me", Name: "Remove", Region: "test",
		Endpoints: []string{"https://example.com"},
	})

	req := httptest.NewRequest("DELETE", "/federation/clusters/remove-me", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestFederationAPI_ClusterStatus(t *testing.T) {
	handler, router := setupFederationAPI(t)

	handler.federation.AddCluster(&FederatedCluster{
		ID: "status-test", Name: "Status", Region: "test",
		Endpoints: []string{"https://example.com"},
	})

	req := httptest.NewRequest("GET", "/federation/clusters/status-test/status", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}
