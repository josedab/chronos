package dispatcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/chronos/chronos/internal/models"
)

func TestComputeHMAC(t *testing.T) {
	secret := "test-secret"
	payload := `{"key":"value"}`

	sig := ComputeHMAC(secret, payload, 0)
	if sig == "" {
		t.Fatal("expected non-empty signature")
	}

	// Same input produces same output
	sig2 := ComputeHMAC(secret, payload, 0)
	if sig != sig2 {
		t.Errorf("expected deterministic output, got %s and %s", sig, sig2)
	}

	// Different secret produces different output
	sig3 := ComputeHMAC("other-secret", payload, 0)
	if sig == sig3 {
		t.Error("different secrets should produce different signatures")
	}

	// Timestamp changes signature
	sig4 := ComputeHMAC(secret, payload, 1234567890)
	if sig == sig4 {
		t.Error("timestamp should change signature")
	}
}

func TestVerifyHMAC(t *testing.T) {
	secret := "test-secret"
	payload := `{"key":"value"}`

	sig := ComputeHMAC(secret, payload, 0)
	if !VerifyHMAC(secret, payload, sig, 0) {
		t.Error("expected valid signature to verify")
	}

	if VerifyHMAC(secret, payload, "invalid-sig", 0) {
		t.Error("expected invalid signature to fail verification")
	}

	if VerifyHMAC("wrong-secret", payload, sig, 0) {
		t.Error("expected wrong secret to fail verification")
	}
}

func TestApplyWebhookSignature(t *testing.T) {
	t.Run("nil config is no-op", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "http://example.com", nil)
		applyWebhookSignature(req, nil, "body")
		if req.Header.Get(defaultSignatureHeader) != "" {
			t.Error("expected no signature header with nil config")
		}
	})

	t.Run("empty secret is no-op", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "http://example.com", nil)
		applyWebhookSignature(req, &SigningConfig{}, "body")
		if req.Header.Get(defaultSignatureHeader) != "" {
			t.Error("expected no signature header with empty secret")
		}
	})

	t.Run("basic signing", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "http://example.com", nil)
		cfg := &SigningConfig{Secret: "my-secret"}
		applyWebhookSignature(req, cfg, `{"data":true}`)

		sig := req.Header.Get(defaultSignatureHeader)
		if sig == "" {
			t.Fatal("expected signature header")
		}
		if !strings.HasPrefix(sig, signaturePrefix) {
			t.Errorf("expected prefix %s, got %s", signaturePrefix, sig)
		}
	})

	t.Run("custom header name", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "http://example.com", nil)
		cfg := &SigningConfig{Secret: "my-secret", HeaderName: "X-My-Sig"}
		applyWebhookSignature(req, cfg, "body")

		if req.Header.Get("X-My-Sig") == "" {
			t.Error("expected custom signature header")
		}
		if req.Header.Get(defaultSignatureHeader) != "" {
			t.Error("should not set default header when custom is specified")
		}
	})

	t.Run("with timestamp", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "http://example.com", nil)
		cfg := &SigningConfig{
			Secret:           "my-secret",
			IncludeTimestamp: true,
		}
		applyWebhookSignature(req, cfg, "body")

		ts := req.Header.Get(defaultTimestampHeader)
		if ts == "" {
			t.Fatal("expected timestamp header")
		}
		if _, err := strconv.ParseInt(ts, 10, 64); err != nil {
			t.Errorf("timestamp should be valid int64: %v", err)
		}
	})
}

func TestDispatcher_ExecuteWebhook_WithSigning(t *testing.T) {
	secret := "webhook-test-secret"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sig := r.Header.Get(defaultSignatureHeader)
		if sig == "" {
			t.Error("missing signature header")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if !strings.HasPrefix(sig, signaturePrefix) {
			t.Errorf("signature missing prefix: %s", sig)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	disp := New(&Config{
		NodeID:        "test-node",
		Timeout:       30 * time.Second,
		MaxConcurrent: 10,
		Signing: &SigningConfig{
			Secret:           secret,
			IncludeTimestamp: true,
		},
	})

	job := &models.Job{
		ID:       "signed-job",
		Name:     "Signed Job",
		Schedule: "* * * * *",
		Webhook: &models.WebhookConfig{
			URL:    server.URL,
			Method: "POST",
			Body:   `{"event":"test"}`,
		},
		RetryPolicy: &models.RetryPolicy{
			MaxAttempts:     1,
			InitialInterval: models.Duration(time.Second),
			MaxInterval:     models.Duration(time.Minute),
			Multiplier:      2.0,
		},
		Timeout: models.Duration(30 * time.Second),
	}

	ctx := context.Background()
	exec, err := disp.Execute(ctx, job, time.Now())
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if exec.Status != models.ExecutionSuccess {
		t.Errorf("expected success, got %s", exec.Status)
	}
}
