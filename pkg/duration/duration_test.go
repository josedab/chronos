package duration

import (
	"encoding/json"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestDuration_Duration(t *testing.T) {
	d := Duration(5 * time.Minute)
	if d.Duration() != 5*time.Minute {
		t.Errorf("expected 5m, got %v", d.Duration())
	}
}

func TestDuration_String(t *testing.T) {
	d := Duration(90 * time.Second)
	if d.String() != "1m30s" {
		t.Errorf("expected '1m30s', got %s", d.String())
	}
}

func TestDuration_MarshalJSON(t *testing.T) {
	d := Duration(5 * time.Minute)
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}
	if string(b) != `"5m0s"` {
		t.Errorf("expected '\"5m0s\"', got %s", string(b))
	}
}

func TestDuration_UnmarshalJSON_String(t *testing.T) {
	var d Duration
	if err := json.Unmarshal([]byte(`"5m"`), &d); err != nil {
		t.Fatalf("UnmarshalJSON failed: %v", err)
	}
	if d.Duration() != 5*time.Minute {
		t.Errorf("expected 5m, got %v", d.Duration())
	}
}

func TestDuration_UnmarshalJSON_Number(t *testing.T) {
	var d Duration
	// 5 minutes in nanoseconds
	ns := float64(5 * time.Minute)
	b, _ := json.Marshal(ns)
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatalf("UnmarshalJSON failed: %v", err)
	}
	if d.Duration() != 5*time.Minute {
		t.Errorf("expected 5m, got %v", d.Duration())
	}
}

func TestDuration_MarshalYAML(t *testing.T) {
	d := Duration(30 * time.Second)
	v, err := d.MarshalYAML()
	if err != nil {
		t.Fatalf("MarshalYAML failed: %v", err)
	}
	if v != "30s" {
		t.Errorf("expected '30s', got %v", v)
	}
}

func TestDuration_UnmarshalYAML(t *testing.T) {
	yamlData := `timeout: 1h30m`
	var config struct {
		Timeout Duration `yaml:"timeout"`
	}
	if err := yaml.Unmarshal([]byte(yamlData), &config); err != nil {
		t.Fatalf("UnmarshalYAML failed: %v", err)
	}
	expected := 90 * time.Minute
	if config.Timeout.Duration() != expected {
		t.Errorf("expected %v, got %v", expected, config.Timeout.Duration())
	}
}
