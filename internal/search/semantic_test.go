package search

import (
	"context"
	"testing"
	"time"
)

func TestDefaultSearchConfig(t *testing.T) {
	cfg := DefaultSearchConfig()

	if cfg.EmbeddingProvider != "local" {
		t.Errorf("expected provider 'local', got %s", cfg.EmbeddingProvider)
	}
	if cfg.EmbeddingDim != 384 {
		t.Errorf("expected dim 384, got %d", cfg.EmbeddingDim)
	}
	if cfg.MinScore != 0.5 {
		t.Errorf("expected min score 0.5, got %f", cfg.MinScore)
	}
	if cfg.MaxResults != 20 {
		t.Errorf("expected max results 20, got %d", cfg.MaxResults)
	}
}

func TestNewVectorIndex(t *testing.T) {
	idx := NewVectorIndex(384)

	if idx.dim != 384 {
		t.Errorf("expected dim 384, got %d", idx.dim)
	}
	if idx.documents == nil {
		t.Error("documents map should be initialized")
	}
}

func TestVectorIndex_AddAndGet(t *testing.T) {
	idx := NewVectorIndex(3)

	job := &JobDocument{ID: "job-1", Name: "Test Job"}
	vector := []float64{0.1, 0.2, 0.3}

	idx.Add("job-1", vector, job)

	doc := idx.Get("job-1")
	if doc == nil {
		t.Fatal("document should be found")
	}
	if doc.ID != "job-1" {
		t.Errorf("expected ID job-1, got %s", doc.ID)
	}
	if len(doc.Vector) != 3 {
		t.Errorf("expected 3-dim vector, got %d", len(doc.Vector))
	}

	retrievedJob := doc.Data.(*JobDocument)
	if retrievedJob.Name != "Test Job" {
		t.Errorf("expected name 'Test Job', got %s", retrievedJob.Name)
	}
}

func TestVectorIndex_Remove(t *testing.T) {
	idx := NewVectorIndex(3)
	idx.Add("job-1", []float64{0.1, 0.2, 0.3}, "data")

	idx.Remove("job-1")

	doc := idx.Get("job-1")
	if doc != nil {
		t.Error("document should be removed")
	}
}

func TestVectorIndex_GetNonexistent(t *testing.T) {
	idx := NewVectorIndex(3)
	doc := idx.Get("nonexistent")
	if doc != nil {
		t.Error("should return nil for nonexistent document")
	}
}

func TestVectorIndex_ForEach(t *testing.T) {
	idx := NewVectorIndex(3)
	idx.Add("job-1", []float64{0.1, 0.2, 0.3}, "data1")
	idx.Add("job-2", []float64{0.4, 0.5, 0.6}, "data2")

	count := 0
	idx.ForEach(func(id string, doc *IndexDocument) {
		count++
	})

	if count != 2 {
		t.Errorf("expected 2 iterations, got %d", count)
	}
}

func TestVectorIndex_Search(t *testing.T) {
	idx := NewVectorIndex(3)

	// Add some documents
	idx.Add("job-1", []float64{1.0, 0.0, 0.0}, &JobDocument{ID: "job-1", Name: "Job 1"})
	idx.Add("job-2", []float64{0.0, 1.0, 0.0}, &JobDocument{ID: "job-2", Name: "Job 2"})
	idx.Add("job-3", []float64{0.9, 0.1, 0.0}, &JobDocument{ID: "job-3", Name: "Job 3"})

	// Search for similar to [1,0,0]
	results := idx.Search([]float64{1.0, 0.0, 0.0}, 3)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// job-1 should be first (exact match)
	if results[0].ID != "job-1" {
		t.Errorf("expected job-1 first, got %s", results[0].ID)
	}
	// job-3 should be second (most similar)
	if results[1].ID != "job-3" {
		t.Errorf("expected job-3 second, got %s", results[1].ID)
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		a, b     []float64
		expected float64
	}{
		{[]float64{1, 0}, []float64{1, 0}, 1.0},
		{[]float64{1, 0}, []float64{0, 1}, 0.0},
		{[]float64{1, 0}, []float64{-1, 0}, -1.0},
		{[]float64{1, 1}, []float64{1, 1}, 1.0},
	}

	for _, tt := range tests {
		got := cosineSimilarity(tt.a, tt.b)
		if absFloat64(got-tt.expected) > 0.0001 {
			t.Errorf("cosineSimilarity(%v, %v) = %f, want %f", tt.a, tt.b, got, tt.expected)
		}
	}
}

func TestCosineSimilarity_ZeroVector(t *testing.T) {
	result := cosineSimilarity([]float64{0, 0}, []float64{1, 1})
	if result != 0 {
		t.Errorf("expected 0 for zero vector, got %f", result)
	}
}

func TestNewSemanticSearch(t *testing.T) {
	cfg := DefaultSearchConfig()
	search, err := NewSemanticSearch(cfg)

	if err != nil {
		t.Fatalf("failed to create semantic search: %v", err)
	}
	if search == nil {
		t.Fatal("search should not be nil")
	}
	if search.index == nil {
		t.Error("index should be initialized")
	}
	if search.embedder == nil {
		t.Error("embedder should be initialized")
	}
}

func TestSemanticSearch_IndexJob(t *testing.T) {
	cfg := DefaultSearchConfig()
	search, _ := NewSemanticSearch(cfg)
	ctx := context.Background()

	job := &JobDocument{
		ID:          "job-1",
		Name:        "Daily Backup",
		Description: "Backup database daily",
		Tags:        []string{"backup", "database"},
		Category:    "maintenance",
	}

	err := search.IndexJob(ctx, job)
	if err != nil {
		t.Fatalf("failed to index job: %v", err)
	}

	// Verify job is in index
	doc := search.index.Get("job-1")
	if doc == nil {
		t.Error("job should be in index")
	}
}

func TestSemanticSearch_RemoveJob(t *testing.T) {
	cfg := DefaultSearchConfig()
	search, _ := NewSemanticSearch(cfg)
	ctx := context.Background()

	job := &JobDocument{ID: "job-1", Name: "Test Job"}
	search.IndexJob(ctx, job)

	search.RemoveJob("job-1")

	doc := search.index.Get("job-1")
	if doc != nil {
		t.Error("job should be removed from index")
	}
}

func TestSemanticSearch_Search(t *testing.T) {
	cfg := DefaultSearchConfig()
	cfg.MinScore = 0.0 // Allow all results for testing
	search, _ := NewSemanticSearch(cfg)
	ctx := context.Background()

	// Index some jobs
	jobs := []*JobDocument{
		{ID: "job-1", Name: "Daily Backup", Description: "Backup database daily", Category: "maintenance"},
		{ID: "job-2", Name: "Send Report", Description: "Send daily report email", Category: "reporting"},
		{ID: "job-3", Name: "Cleanup Logs", Description: "Clean old log files", Category: "maintenance"},
	}
	for _, job := range jobs {
		search.IndexJob(ctx, job)
	}

	// Search for backup-related jobs
	results, err := search.Search(ctx, "backup database", SearchOptions{MaxResults: 10})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	// The local embedder may return different numbers of results
	if results == nil {
		t.Error("results should not be nil")
	}
	if results.Query != "backup database" {
		t.Errorf("expected query 'backup database', got %s", results.Query)
	}
}

func TestSemanticSearch_Search_WithFilters(t *testing.T) {
	cfg := DefaultSearchConfig()
	search, _ := NewSemanticSearch(cfg)
	ctx := context.Background()

	// Index jobs with different categories
	jobs := []*JobDocument{
		{ID: "job-1", Name: "Job A", Category: "maintenance", Enabled: true},
		{ID: "job-2", Name: "Job B", Category: "reporting", Enabled: true},
		{ID: "job-3", Name: "Job C", Category: "maintenance", Enabled: false},
	}
	for _, job := range jobs {
		search.IndexJob(ctx, job)
	}

	// Search with category filter
	results, err := search.Search(ctx, "job", SearchOptions{
		Filters:    map[string]string{"category": "maintenance"},
		MaxResults: 10,
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	// Should only get maintenance jobs
	for _, r := range results.Results {
		if r.Job.Category != "maintenance" {
			t.Errorf("expected category maintenance, got %s", r.Job.Category)
		}
	}
}

func TestSemanticSearch_BuildSearchableText(t *testing.T) {
	cfg := DefaultSearchConfig()
	search, _ := NewSemanticSearch(cfg)

	job := &JobDocument{
		ID:          "job-1",
		Name:        "Daily Backup",
		Description: "Backup database",
		Tags:        []string{"backup", "db"},
		Category:    "maintenance",
		Owner:       "devops",
		Namespace:   "production",
		Labels:      map[string]string{"env": "prod"},
	}

	text := search.buildSearchableText(job)

	if text == "" {
		t.Error("searchable text should not be empty")
	}
	if !contains(text, "Daily Backup") {
		t.Error("text should contain job name")
	}
	if !contains(text, "backup") {
		t.Error("text should contain tag")
	}
	if !contains(text, "maintenance") {
		t.Error("text should contain category")
	}
	if !contains(text, "owner:devops") {
		t.Error("text should contain owner")
	}
}

func TestSemanticSearch_SearchHybrid(t *testing.T) {
	cfg := DefaultSearchConfig()
	search, _ := NewSemanticSearch(cfg)
	ctx := context.Background()

	// Index some jobs
	jobs := []*JobDocument{
		{ID: "job-1", Name: "Daily Backup", Description: "Backup database daily"},
		{ID: "job-2", Name: "Send Report", Description: "Send daily report"},
	}
	for _, job := range jobs {
		search.IndexJob(ctx, job)
	}

	results, err := search.SearchHybrid(ctx, "backup", SearchOptions{MaxResults: 10})
	if err != nil {
		t.Fatalf("hybrid search failed: %v", err)
	}

	if results == nil {
		t.Error("results should not be nil")
	}
}

func TestSemanticSearch_SimilarJobs(t *testing.T) {
	cfg := DefaultSearchConfig()
	search, _ := NewSemanticSearch(cfg)
	ctx := context.Background()

	// Index some jobs
	jobs := []*JobDocument{
		{ID: "job-1", Name: "Daily Backup", Description: "Backup database daily"},
		{ID: "job-2", Name: "Weekly Backup", Description: "Backup database weekly"},
		{ID: "job-3", Name: "Send Report", Description: "Send report email"},
	}
	for _, job := range jobs {
		search.IndexJob(ctx, job)
	}

	similar, err := search.SimilarJobs(ctx, "job-1", 2)
	if err != nil {
		t.Fatalf("similar jobs failed: %v", err)
	}

	// Should not include the source job
	for _, r := range similar {
		if r.Job.ID == "job-1" {
			t.Error("similar jobs should not include source job")
		}
	}
}

func TestSemanticSearch_SimilarJobs_NotFound(t *testing.T) {
	cfg := DefaultSearchConfig()
	search, _ := NewSemanticSearch(cfg)
	ctx := context.Background()

	_, err := search.SimilarJobs(ctx, "nonexistent", 10)
	if err == nil {
		t.Error("expected error for nonexistent job")
	}
}

func TestJobDocument(t *testing.T) {
	job := &JobDocument{
		ID:          "job-123",
		Name:        "Test Job",
		Description: "A test job",
		Tags:        []string{"test", "example"},
		Category:    "testing",
		Owner:       "admin",
		Namespace:   "default",
		Labels:      map[string]string{"env": "test"},
		Enabled:     true,
		Schedule:    "0 * * * *",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if job.ID != "job-123" {
		t.Errorf("unexpected ID: %s", job.ID)
	}
	if len(job.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(job.Tags))
	}
}

func TestSearchOptions(t *testing.T) {
	opts := SearchOptions{
		Filters:    map[string]string{"category": "test"},
		MaxResults: 50,
		Boost:      map[string]float64{"name": 2.0},
	}

	if opts.Filters["category"] != "test" {
		t.Error("filter not set correctly")
	}
	if opts.MaxResults != 50 {
		t.Errorf("expected max results 50, got %d", opts.MaxResults)
	}
}

// Helper functions

func absFloat64(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
