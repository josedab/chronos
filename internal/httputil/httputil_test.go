package httputil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteJSON(rec, http.StatusOK, map[string]string{"key": "value"})

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Error("expected application/json content type")
	}

	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)
	if body["key"] != "value" {
		t.Errorf("expected key=value, got %v", body)
	}
}

func TestSuccess(t *testing.T) {
	rec := httptest.NewRecorder()
	Success(rec, map[string]int{"count": 42})

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&body)
	if body["success"] != true {
		t.Error("expected success=true")
	}
}

func TestCreated(t *testing.T) {
	rec := httptest.NewRecorder()
	Created(rec, map[string]string{"id": "123"})

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rec.Code)
	}
}

func TestError(t *testing.T) {
	rec := httptest.NewRecorder()
	Error(rec, http.StatusBadRequest, "invalid", "bad input")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}

	var body map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&body)
	if body["success"] != false {
		t.Error("expected success=false")
	}
	errObj := body["error"].(map[string]interface{})
	if errObj["code"] != "invalid" {
		t.Errorf("expected code=invalid, got %v", errObj["code"])
	}
}
