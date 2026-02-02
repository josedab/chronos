package marketplace

import (
	"context"
	"testing"
	"time"
)

func TestMarketplaceService_RegisterPublisher(t *testing.T) {
	registry := NewWorkflowRegistry()
	svc := NewMarketplaceService(registry)

	pub := &Publisher{
		Name:        "test-publisher",
		DisplayName: "Test Publisher",
		Email:       "pub@test.com",
	}

	err := svc.RegisterPublisher(pub)
	if err != nil {
		t.Fatalf("RegisterPublisher failed: %v", err)
	}

	if pub.ID == "" {
		t.Error("expected publisher ID to be set")
	}

	// Try to register with same name
	dup := &Publisher{
		Name:  "test-publisher",
		Email: "other@test.com",
	}
	err = svc.RegisterPublisher(dup)
	if err != ErrPublisherExists {
		t.Errorf("expected ErrPublisherExists, got %v", err)
	}
}

func TestMarketplaceService_GetPublisher(t *testing.T) {
	registry := NewWorkflowRegistry()
	svc := NewMarketplaceService(registry)

	pub := &Publisher{
		Name:        "test-publisher",
		DisplayName: "Test Publisher",
		Email:       "pub@test.com",
	}
	svc.RegisterPublisher(pub)

	retrieved, err := svc.GetPublisher(pub.ID)
	if err != nil {
		t.Fatalf("GetPublisher failed: %v", err)
	}

	if retrieved.Name != pub.Name {
		t.Errorf("expected name %q, got %q", pub.Name, retrieved.Name)
	}
}

func TestMarketplaceService_GetPublisher_NotFound(t *testing.T) {
	registry := NewWorkflowRegistry()
	svc := NewMarketplaceService(registry)

	_, err := svc.GetPublisher("nonexistent")
	if err != ErrPublisherNotFound {
		t.Errorf("expected ErrPublisherNotFound, got %v", err)
	}
}

func TestMarketplaceService_VerifyPublisher(t *testing.T) {
	registry := NewWorkflowRegistry()
	svc := NewMarketplaceService(registry)

	pub := &Publisher{
		Name:           "test-publisher",
		Email:          "pub@test.com",
		GithubUsername: "testuser",
	}
	svc.RegisterPublisher(pub)

	// Verify with GitHub method
	err := svc.VerifyPublisher(context.Background(), pub.ID, PublisherVerification{
		Method:      VerificationMethodGitHub,
		GitHubToken: "test-token",
	})
	if err != nil {
		t.Fatalf("VerifyPublisher failed: %v", err)
	}

	// Check publisher is now verified
	verified, _ := svc.GetPublisher(pub.ID)
	if !verified.Verified {
		t.Error("expected publisher to be verified")
	}
	if verified.VerifiedAt == nil {
		t.Error("expected VerifiedAt to be set")
	}
	if verified.TrustScore <= 0.5 {
		t.Error("expected trust score to increase after verification")
	}

	// Should have verification badge
	hasBadge := false
	for _, badge := range verified.Badges {
		if badge.ID == "verified" {
			hasBadge = true
			break
		}
	}
	if !hasBadge {
		t.Error("expected verified badge")
	}
}

func TestMarketplaceService_SubmitRating(t *testing.T) {
	registry := NewWorkflowRegistry()
	svc := NewMarketplaceService(registry)

	// Get a built-in template ID
	templates := registry.ListWorkflowTemplates(WorkflowFilter{})
	if len(templates) == 0 {
		t.Skip("no templates available for testing")
	}
	templateID := templates[0].ID

	rating, err := svc.SubmitRating("user-1", templateID, 5, "Great!", "Works perfectly")
	if err != nil {
		t.Fatalf("SubmitRating failed: %v", err)
	}

	if rating.Score != 5 {
		t.Errorf("expected score 5, got %d", rating.Score)
	}
	if rating.UserID != "user-1" {
		t.Errorf("expected user ID 'user-1', got %q", rating.UserID)
	}
}

func TestMarketplaceService_SubmitRating_InvalidScore(t *testing.T) {
	registry := NewWorkflowRegistry()
	svc := NewMarketplaceService(registry)

	templates := registry.ListWorkflowTemplates(WorkflowFilter{})
	if len(templates) == 0 {
		t.Skip("no templates available for testing")
	}
	templateID := templates[0].ID

	_, err := svc.SubmitRating("user-1", templateID, 6, "", "")
	if err != ErrInvalidRating {
		t.Errorf("expected ErrInvalidRating, got %v", err)
	}

	_, err = svc.SubmitRating("user-1", templateID, 0, "", "")
	if err != ErrInvalidRating {
		t.Errorf("expected ErrInvalidRating, got %v", err)
	}
}

func TestMarketplaceService_SubmitRating_RequiresComment(t *testing.T) {
	registry := NewWorkflowRegistry()
	svc := NewMarketplaceService(registry)

	templates := registry.ListWorkflowTemplates(WorkflowFilter{})
	if len(templates) == 0 {
		t.Skip("no templates available for testing")
	}
	templateID := templates[0].ID

	// Low ratings require a comment
	_, err := svc.SubmitRating("user-1", templateID, 2, "", "")
	if err != ErrReviewRequired {
		t.Errorf("expected ErrReviewRequired, got %v", err)
	}

	// With comment should work
	rating, err := svc.SubmitRating("user-1", templateID, 2, "Issues", "Had some problems")
	if err != nil {
		t.Fatalf("SubmitRating with comment failed: %v", err)
	}
	if rating.Score != 2 {
		t.Errorf("expected score 2, got %d", rating.Score)
	}
}

func TestMarketplaceService_SubmitRating_DuplicatePrevention(t *testing.T) {
	registry := NewWorkflowRegistry()
	svc := NewMarketplaceService(registry)

	templates := registry.ListWorkflowTemplates(WorkflowFilter{})
	if len(templates) == 0 {
		t.Skip("no templates available for testing")
	}
	templateID := templates[0].ID

	// First rating should succeed
	_, err := svc.SubmitRating("user-1", templateID, 5, "", "Great")
	if err != nil {
		t.Fatalf("First rating failed: %v", err)
	}

	// Second rating from same user should fail
	_, err = svc.SubmitRating("user-1", templateID, 4, "", "Changed my mind")
	if err != ErrRatingExists {
		t.Errorf("expected ErrRatingExists, got %v", err)
	}
}

func TestMarketplaceService_UpdateRating(t *testing.T) {
	registry := NewWorkflowRegistry()
	svc := NewMarketplaceService(registry)

	templates := registry.ListWorkflowTemplates(WorkflowFilter{})
	if len(templates) == 0 {
		t.Skip("no templates available for testing")
	}
	templateID := templates[0].ID

	// Submit initial rating
	svc.SubmitRating("user-1", templateID, 3, "OK", "Decent")

	// Update it
	updated, err := svc.UpdateRating("user-1", templateID, 5, "Great!", "Much better now")
	if err != nil {
		t.Fatalf("UpdateRating failed: %v", err)
	}

	if updated.Score != 5 {
		t.Errorf("expected score 5, got %d", updated.Score)
	}
}

func TestMarketplaceService_GetRatings(t *testing.T) {
	registry := NewWorkflowRegistry()
	svc := NewMarketplaceService(registry)

	templates := registry.ListWorkflowTemplates(WorkflowFilter{})
	if len(templates) == 0 {
		t.Skip("no templates available for testing")
	}
	templateID := templates[0].ID

	// Add multiple ratings
	svc.SubmitRating("user-1", templateID, 5, "", "Great")
	svc.SubmitRating("user-2", templateID, 4, "", "Good")
	svc.SubmitRating("user-3", templateID, 3, "", "OK")

	ratings, err := svc.GetRatings(templateID, RatingQueryOptions{})
	if err != nil {
		t.Fatalf("GetRatings failed: %v", err)
	}

	if len(ratings) != 3 {
		t.Errorf("expected 3 ratings, got %d", len(ratings))
	}
}

func TestMarketplaceService_GetRatings_Sorted(t *testing.T) {
	registry := NewWorkflowRegistry()
	svc := NewMarketplaceService(registry)

	templates := registry.ListWorkflowTemplates(WorkflowFilter{})
	if len(templates) == 0 {
		t.Skip("no templates available for testing")
	}
	templateID := templates[0].ID

	svc.SubmitRating("user-1", templateID, 5, "", "Great")
	time.Sleep(time.Millisecond) // Ensure different timestamps
	svc.SubmitRating("user-2", templateID, 3, "", "OK")
	time.Sleep(time.Millisecond)
	svc.SubmitRating("user-3", templateID, 4, "", "Good")

	// Sort by highest
	ratings, _ := svc.GetRatings(templateID, RatingQueryOptions{SortBy: "highest"})
	if ratings[0].Score != 5 {
		t.Errorf("expected highest score first, got %d", ratings[0].Score)
	}

	// Sort by lowest
	ratings, _ = svc.GetRatings(templateID, RatingQueryOptions{SortBy: "lowest"})
	if ratings[0].Score != 3 {
		t.Errorf("expected lowest score first, got %d", ratings[0].Score)
	}
}

func TestMarketplaceService_GetRatingStats(t *testing.T) {
	registry := NewWorkflowRegistry()
	svc := NewMarketplaceService(registry)

	templates := registry.ListWorkflowTemplates(WorkflowFilter{})
	if len(templates) == 0 {
		t.Skip("no templates available for testing")
	}
	templateID := templates[0].ID

	// Add ratings
	svc.SubmitRating("user-1", templateID, 5, "", "Great")
	svc.SubmitRating("user-2", templateID, 5, "", "Excellent")
	svc.SubmitRating("user-3", templateID, 4, "", "Good")
	svc.SubmitRating("user-4", templateID, 3, "", "OK")

	stats, err := svc.GetRatingStats(templateID)
	if err != nil {
		t.Fatalf("GetRatingStats failed: %v", err)
	}

	if stats.TotalRatings != 4 {
		t.Errorf("expected 4 total ratings, got %d", stats.TotalRatings)
	}

	expectedAvg := (5.0 + 5.0 + 4.0 + 3.0) / 4.0
	if stats.AverageRating != expectedAvg {
		t.Errorf("expected average %.2f, got %.2f", expectedAvg, stats.AverageRating)
	}

	if stats.Distribution[5] != 2 {
		t.Errorf("expected 2 five-star ratings, got %d", stats.Distribution[5])
	}
}

func TestMarketplaceService_MarkHelpful(t *testing.T) {
	registry := NewWorkflowRegistry()
	svc := NewMarketplaceService(registry)

	templates := registry.ListWorkflowTemplates(WorkflowFilter{})
	if len(templates) == 0 {
		t.Skip("no templates available for testing")
	}
	templateID := templates[0].ID

	rating, _ := svc.SubmitRating("user-1", templateID, 5, "", "Great")

	err := svc.MarkHelpful(rating.ID, true)
	if err != nil {
		t.Fatalf("MarkHelpful failed: %v", err)
	}

	ratings, _ := svc.GetRatings(templateID, RatingQueryOptions{})
	if ratings[0].Helpful != 1 {
		t.Errorf("expected 1 helpful vote, got %d", ratings[0].Helpful)
	}

	// Mark not helpful
	svc.MarkHelpful(rating.ID, false)
	ratings, _ = svc.GetRatings(templateID, RatingQueryOptions{})
	if ratings[0].NotHelpful != 1 {
		t.Errorf("expected 1 not helpful vote, got %d", ratings[0].NotHelpful)
	}
}

func TestMarketplaceService_SearchTemplates(t *testing.T) {
	registry := NewWorkflowRegistry()
	svc := NewMarketplaceService(registry)

	result, err := svc.SearchTemplates(SearchQuery{
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("SearchTemplates failed: %v", err)
	}

	if result.TotalCount == 0 {
		t.Skip("no templates available for testing")
	}

	if len(result.Templates) > 10 {
		t.Errorf("expected max 10 templates, got %d", len(result.Templates))
	}
}

func TestMarketplaceService_SearchTemplates_ByCategory(t *testing.T) {
	registry := NewWorkflowRegistry()
	svc := NewMarketplaceService(registry)

	result, err := svc.SearchTemplates(SearchQuery{
		Category: WorkflowCategoryETL,
	})
	if err != nil {
		t.Fatalf("SearchTemplates failed: %v", err)
	}

	for _, tmpl := range result.Templates {
		if tmpl.Category != WorkflowCategoryETL {
			t.Errorf("expected category ETL, got %q", tmpl.Category)
		}
	}
}

func TestMarketplaceService_GetTrendingTemplates(t *testing.T) {
	registry := NewWorkflowRegistry()
	svc := NewMarketplaceService(registry)

	trending := svc.GetTrendingTemplates(5)
	if len(trending) > 5 {
		t.Errorf("expected max 5 templates, got %d", len(trending))
	}
}

func TestMarketplaceService_GetFeaturedTemplates(t *testing.T) {
	registry := NewWorkflowRegistry()
	svc := NewMarketplaceService(registry)

	featured := svc.GetFeaturedTemplates()
	for _, tmpl := range featured {
		if !tmpl.Featured {
			t.Errorf("expected featured template, got non-featured: %s", tmpl.ID)
		}
	}
}

func TestGenerateTemplateHash(t *testing.T) {
	tmpl := &WorkflowTemplate{
		Name:    "Test Template",
		Version: "1.0.0",
		Steps: []WorkflowStepTemplate{
			{ID: "step-1", Name: "Step 1", Type: StepTypeJob},
		},
	}

	hash := GenerateTemplateHash(tmpl)
	if hash == "" {
		t.Error("expected non-empty hash")
	}

	// Same template should produce same hash
	hash2 := GenerateTemplateHash(tmpl)
	if hash != hash2 {
		t.Error("expected same hash for same template")
	}

	// Modified template should produce different hash
	tmpl.Version = "1.0.1"
	hash3 := GenerateTemplateHash(tmpl)
	if hash == hash3 {
		t.Error("expected different hash for modified template")
	}
}
