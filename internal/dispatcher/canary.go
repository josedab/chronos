// Package dispatcher provides canary and shadow webhook execution.
package dispatcher

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/chronos/chronos/internal/models"
)

// ShadowResult holds the comparison between primary and shadow execution.
type ShadowResult struct {
	PrimaryResult *models.ExecutionResult `json:"primary"`
	ShadowResult  *models.ExecutionResult `json:"shadow"`
	Match         bool                    `json:"match"`
	Discrepancies []Discrepancy           `json:"discrepancies,omitempty"`
	ShadowLatency time.Duration           `json:"shadow_latency"`
}

// Discrepancy describes a difference between primary and shadow responses.
type Discrepancy struct {
	Field    string `json:"field"`
	Primary  string `json:"primary"`
	Shadow   string `json:"shadow"`
}

// ExecuteShadow runs both primary and shadow webhooks in parallel, compares results.
func (d *Dispatcher) ExecuteShadow(ctx context.Context, primary, shadow *models.WebhookConfig, execution *models.Execution) (*ShadowResult, error) {
	if primary == nil {
		return nil, fmt.Errorf("primary webhook is required")
	}
	if shadow == nil {
		return nil, fmt.Errorf("shadow webhook is required")
	}

	var (
		primaryResult *models.ExecutionResult
		shadowResult  *models.ExecutionResult
		primaryErr    error
		shadowErr     error
		wg            sync.WaitGroup
		shadowStart   time.Time
		shadowDur     time.Duration
	)

	wg.Add(2)

	go func() {
		defer wg.Done()
		primaryResult, primaryErr = d.executeWebhook(ctx, primary, execution)
	}()

	go func() {
		defer wg.Done()
		shadowStart = time.Now()
		shadowResult, shadowErr = d.executeWebhook(ctx, shadow, execution)
		shadowDur = time.Since(shadowStart)
	}()

	wg.Wait()

	sr := &ShadowResult{
		PrimaryResult: primaryResult,
		ShadowResult:  shadowResult,
		ShadowLatency: shadowDur,
		Match:         true,
	}

	// Compare results
	if primaryErr != nil || shadowErr != nil {
		sr.Match = false
		if primaryErr != nil && shadowErr == nil {
			sr.Discrepancies = append(sr.Discrepancies, Discrepancy{
				Field: "error", Primary: primaryErr.Error(), Shadow: "none",
			})
		} else if primaryErr == nil && shadowErr != nil {
			sr.Discrepancies = append(sr.Discrepancies, Discrepancy{
				Field: "error", Primary: "none", Shadow: shadowErr.Error(),
			})
		}
	}

	if primaryResult != nil && shadowResult != nil {
		if primaryResult.StatusCode != shadowResult.StatusCode {
			sr.Match = false
			sr.Discrepancies = append(sr.Discrepancies, Discrepancy{
				Field:   "status_code",
				Primary: fmt.Sprintf("%d", primaryResult.StatusCode),
				Shadow:  fmt.Sprintf("%d", shadowResult.StatusCode),
			})
		}
		if primaryResult.Success != shadowResult.Success {
			sr.Match = false
			sr.Discrepancies = append(sr.Discrepancies, Discrepancy{
				Field:   "success",
				Primary: fmt.Sprintf("%v", primaryResult.Success),
				Shadow:  fmt.Sprintf("%v", shadowResult.Success),
			})
		}
	}

	return sr, primaryErr
}

// ShouldUseCanary determines if this execution should route to the canary target.
func ShouldUseCanary(percent int) bool {
	if percent <= 0 {
		return false
	}
	if percent >= 100 {
		return true
	}
	return rand.Intn(100) < percent
}
