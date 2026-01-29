// Package search provides semantic job search capabilities for Chronos.
package search

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// SearchConfig configures the search engine.
type SearchConfig struct {
	EmbeddingProvider string `json:"embedding_provider"` // "openai", "local", "sentence-transformers"
	EmbeddingModel    string `json:"embedding_model"`
	APIKey            string `json:"api_key,omitempty"`
	EmbeddingDim      int    `json:"embedding_dim"`
	MinScore          float64 `json:"min_score"` // Minimum similarity score
	MaxResults        int    `json:"max_results"`
}

// DefaultSearchConfig returns sensible defaults.
func DefaultSearchConfig() SearchConfig {
	return SearchConfig{
		EmbeddingProvider: "local",
		EmbeddingModel:    "all-MiniLM-L6-v2",
		EmbeddingDim:      384,
		MinScore:          0.5,
		MaxResults:        20,
	}
}

// SemanticSearch provides vector-based job search.
type SemanticSearch struct {
	config    SearchConfig
	index     *VectorIndex
	embedder  Embedder
	mu        sync.RWMutex
}

// NewSemanticSearch creates a new semantic search engine.
func NewSemanticSearch(config SearchConfig) (*SemanticSearch, error) {
	embedder, err := createEmbedder(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedder: %w", err)
	}

	return &SemanticSearch{
		config:   config,
		index:    NewVectorIndex(config.EmbeddingDim),
		embedder: embedder,
	}, nil
}

// IndexJob indexes a job for search.
func (s *SemanticSearch) IndexJob(ctx context.Context, job *JobDocument) error {
	// Create searchable text
	text := s.buildSearchableText(job)

	// Generate embedding
	embedding, err := s.embedder.Embed(ctx, text)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Add to index
	s.mu.Lock()
	defer s.mu.Unlock()

	s.index.Add(job.ID, embedding, job)
	return nil
}

// RemoveJob removes a job from the index.
func (s *SemanticSearch) RemoveJob(jobID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.index.Remove(jobID)
}

// Search performs semantic search.
func (s *SemanticSearch) Search(ctx context.Context, query string, opts SearchOptions) (*SearchResults, error) {
	// Generate query embedding
	queryEmbedding, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Vector search
	candidates := s.index.Search(queryEmbedding, opts.MaxResults*2) // Get extra for filtering

	// Apply filters
	filtered := s.applyFilters(candidates, opts)

	// Score and rank
	results := s.rankResults(filtered, query, opts)

	// Limit results
	maxResults := s.config.MaxResults
	if opts.MaxResults > 0 {
		maxResults = opts.MaxResults
	}
	if len(results) > maxResults {
		results = results[:maxResults]
	}

	return &SearchResults{
		Query:       query,
		Total:       len(results),
		Results:     results,
		Suggestions: s.generateSuggestions(query, results),
		Facets:      s.computeFacets(results),
		Took:        0, // Would measure actual time
	}, nil
}

// SearchHybrid performs hybrid search combining vector and keyword search.
func (s *SemanticSearch) SearchHybrid(ctx context.Context, query string, opts SearchOptions) (*SearchResults, error) {
	// Vector search
	vectorResults, err := s.Search(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	// Keyword search
	keywordResults := s.keywordSearch(query, opts)

	// Merge results using reciprocal rank fusion
	merged := s.mergeResults(vectorResults.Results, keywordResults, 60.0)

	return &SearchResults{
		Query:       query,
		Total:       len(merged),
		Results:     merged,
		Suggestions: vectorResults.Suggestions,
		Facets:      vectorResults.Facets,
	}, nil
}

// SimilarJobs finds jobs similar to a given job.
func (s *SemanticSearch) SimilarJobs(ctx context.Context, jobID string, limit int) ([]SearchResult, error) {
	s.mu.RLock()
	doc := s.index.Get(jobID)
	s.mu.RUnlock()

	if doc == nil {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	candidates := s.index.Search(doc.Vector, limit+1)

	// Filter out the source job
	results := make([]SearchResult, 0, limit)
	for _, c := range candidates {
		if c.ID != jobID {
			results = append(results, SearchResult{
				Job:   c.Data.(*JobDocument),
				Score: c.Score,
			})
		}
		if len(results) >= limit {
			break
		}
	}

	return results, nil
}

func (s *SemanticSearch) buildSearchableText(job *JobDocument) string {
	var parts []string

	parts = append(parts, job.Name)
	if job.Description != "" {
		parts = append(parts, job.Description)
	}
	for _, tag := range job.Tags {
		parts = append(parts, tag)
	}
	if job.Category != "" {
		parts = append(parts, job.Category)
	}
	if job.Owner != "" {
		parts = append(parts, "owner:"+job.Owner)
	}
	if job.Namespace != "" {
		parts = append(parts, "namespace:"+job.Namespace)
	}
	for k, v := range job.Labels {
		parts = append(parts, k+":"+v)
	}

	return strings.Join(parts, " ")
}

func (s *SemanticSearch) applyFilters(candidates []VectorResult, opts SearchOptions) []VectorResult {
	if len(opts.Filters) == 0 {
		return candidates
	}

	filtered := make([]VectorResult, 0)
	for _, c := range candidates {
		job := c.Data.(*JobDocument)
		match := true

		for key, value := range opts.Filters {
			switch key {
			case "namespace":
				if job.Namespace != value {
					match = false
				}
			case "owner":
				if job.Owner != value {
					match = false
				}
			case "enabled":
				if fmt.Sprintf("%v", job.Enabled) != value {
					match = false
				}
			case "category":
				if job.Category != value {
					match = false
				}
			case "tag":
				hasTag := false
				for _, t := range job.Tags {
					if t == value {
						hasTag = true
						break
					}
				}
				if !hasTag {
					match = false
				}
			default:
				if v, ok := job.Labels[key]; !ok || v != value {
					match = false
				}
			}
		}

		if match {
			filtered = append(filtered, c)
		}
	}

	return filtered
}

func (s *SemanticSearch) rankResults(candidates []VectorResult, query string, opts SearchOptions) []SearchResult {
	results := make([]SearchResult, 0)

	for _, c := range candidates {
		if c.Score < s.config.MinScore {
			continue
		}

		job := c.Data.(*JobDocument)

		// Boost for exact matches
		boost := 1.0
		queryLower := strings.ToLower(query)
		if strings.Contains(strings.ToLower(job.Name), queryLower) {
			boost *= 1.5
		}
		if strings.Contains(strings.ToLower(job.Description), queryLower) {
			boost *= 1.2
		}

		results = append(results, SearchResult{
			Job:       job,
			Score:     c.Score * boost,
			Highlights: s.generateHighlights(job, query),
		})
	}

	// Sort by score
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

func (s *SemanticSearch) keywordSearch(query string, opts SearchOptions) []SearchResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]SearchResult, 0)
	queryTerms := strings.Fields(strings.ToLower(query))

	s.index.ForEach(func(id string, doc *IndexDocument) {
		job := doc.Data.(*JobDocument)
		score := 0.0

		// Simple TF scoring
		text := strings.ToLower(s.buildSearchableText(job))
		for _, term := range queryTerms {
			count := strings.Count(text, term)
			if count > 0 {
				score += float64(count) / float64(len(text)) * 100
			}
		}

		if score > 0 {
			results = append(results, SearchResult{
				Job:   job,
				Score: score,
			})
		}
	})

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

func (s *SemanticSearch) mergeResults(vectorResults, keywordResults []SearchResult, k float64) []SearchResult {
	scores := make(map[string]float64)
	jobs := make(map[string]*JobDocument)

	// Score from vector search
	for i, r := range vectorResults {
		scores[r.Job.ID] += 1.0 / (k + float64(i+1))
		jobs[r.Job.ID] = r.Job
	}

	// Score from keyword search
	for i, r := range keywordResults {
		scores[r.Job.ID] += 1.0 / (k + float64(i+1))
		jobs[r.Job.ID] = r.Job
	}

	// Build merged results
	merged := make([]SearchResult, 0, len(scores))
	for id, score := range scores {
		merged = append(merged, SearchResult{
			Job:   jobs[id],
			Score: score,
		})
	}

	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Score > merged[j].Score
	})

	return merged
}

func (s *SemanticSearch) generateHighlights(job *JobDocument, query string) map[string][]string {
	highlights := make(map[string][]string)
	queryTerms := strings.Fields(strings.ToLower(query))

	// Check name
	for _, term := range queryTerms {
		if strings.Contains(strings.ToLower(job.Name), term) {
			highlights["name"] = append(highlights["name"], highlightTerm(job.Name, term))
		}
	}

	// Check description
	for _, term := range queryTerms {
		if strings.Contains(strings.ToLower(job.Description), term) {
			highlights["description"] = append(highlights["description"], highlightTerm(job.Description, term))
		}
	}

	return highlights
}

func highlightTerm(text, term string) string {
	// Simple highlight with markers
	lower := strings.ToLower(text)
	idx := strings.Index(lower, term)
	if idx == -1 {
		return text
	}

	start := idx - 30
	if start < 0 {
		start = 0
	}
	end := idx + len(term) + 30
	if end > len(text) {
		end = len(text)
	}

	snippet := text[start:end]
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(text) {
		snippet = snippet + "..."
	}

	return snippet
}

func (s *SemanticSearch) generateSuggestions(query string, results []SearchResult) []string {
	suggestions := make([]string, 0)

	// Extract common terms from top results
	termCounts := make(map[string]int)
	for i, r := range results {
		if i >= 5 {
			break
		}
		for _, tag := range r.Job.Tags {
			termCounts[tag]++
		}
		if r.Job.Category != "" {
			termCounts["category:"+r.Job.Category]++
		}
	}

	// Sort by frequency
	type termCount struct {
		term  string
		count int
	}
	var terms []termCount
	for term, count := range termCounts {
		terms = append(terms, termCount{term, count})
	}
	sort.Slice(terms, func(i, j int) bool {
		return terms[i].count > terms[j].count
	})

	// Build suggestions
	for _, tc := range terms {
		if len(suggestions) >= 3 {
			break
		}
		if !strings.Contains(strings.ToLower(query), strings.ToLower(tc.term)) {
			suggestions = append(suggestions, query+" "+tc.term)
		}
	}

	return suggestions
}

func (s *SemanticSearch) computeFacets(results []SearchResult) map[string][]Facet {
	facets := make(map[string][]Facet)

	// Compute category facets
	categoryCounts := make(map[string]int)
	namespaceCounts := make(map[string]int)
	tagCounts := make(map[string]int)

	for _, r := range results {
		if r.Job.Category != "" {
			categoryCounts[r.Job.Category]++
		}
		if r.Job.Namespace != "" {
			namespaceCounts[r.Job.Namespace]++
		}
		for _, tag := range r.Job.Tags {
			tagCounts[tag]++
		}
	}

	// Convert to facets
	facets["category"] = countsToFacets(categoryCounts)
	facets["namespace"] = countsToFacets(namespaceCounts)
	facets["tag"] = countsToFacets(tagCounts)

	return facets
}

func countsToFacets(counts map[string]int) []Facet {
	facets := make([]Facet, 0, len(counts))
	for value, count := range counts {
		facets = append(facets, Facet{Value: value, Count: count})
	}
	sort.Slice(facets, func(i, j int) bool {
		return facets[i].Count > facets[j].Count
	})
	return facets
}

// JobDocument represents an indexed job.
type JobDocument struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Tags        []string          `json:"tags"`
	Category    string            `json:"category"`
	Owner       string            `json:"owner"`
	Namespace   string            `json:"namespace"`
	Labels      map[string]string `json:"labels"`
	Enabled     bool              `json:"enabled"`
	Schedule    string            `json:"schedule"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// SearchOptions defines search parameters.
type SearchOptions struct {
	Filters    map[string]string `json:"filters,omitempty"`
	MaxResults int               `json:"max_results,omitempty"`
	Boost      map[string]float64 `json:"boost,omitempty"`
}

// SearchResults contains search results.
type SearchResults struct {
	Query       string              `json:"query"`
	Total       int                 `json:"total"`
	Results     []SearchResult      `json:"results"`
	Suggestions []string            `json:"suggestions"`
	Facets      map[string][]Facet  `json:"facets"`
	Took        time.Duration       `json:"took"`
}

// SearchResult is a single search result.
type SearchResult struct {
	Job        *JobDocument       `json:"job"`
	Score      float64            `json:"score"`
	Highlights map[string][]string `json:"highlights,omitempty"`
}

// Facet is a search facet.
type Facet struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

// VectorIndex is a simple in-memory vector index.
type VectorIndex struct {
	dim       int
	documents map[string]*IndexDocument
	mu        sync.RWMutex
}

// IndexDocument is a document in the index.
type IndexDocument struct {
	ID     string
	Vector []float64
	Data   interface{}
}

// NewVectorIndex creates a new vector index.
func NewVectorIndex(dim int) *VectorIndex {
	return &VectorIndex{
		dim:       dim,
		documents: make(map[string]*IndexDocument),
	}
}

// Add adds a document to the index.
func (idx *VectorIndex) Add(id string, vector []float64, data interface{}) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.documents[id] = &IndexDocument{
		ID:     id,
		Vector: vector,
		Data:   data,
	}
}

// Remove removes a document from the index.
func (idx *VectorIndex) Remove(id string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	delete(idx.documents, id)
}

// Get retrieves a document by ID.
func (idx *VectorIndex) Get(id string) *IndexDocument {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.documents[id]
}

// ForEach iterates over all documents.
func (idx *VectorIndex) ForEach(f func(string, *IndexDocument)) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	for id, doc := range idx.documents {
		f(id, doc)
	}
}

// VectorResult is a vector search result.
type VectorResult struct {
	ID    string
	Score float64
	Data  interface{}
}

// Search finds the nearest neighbors.
func (idx *VectorIndex) Search(query []float64, k int) []VectorResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	results := make([]VectorResult, 0, len(idx.documents))

	for id, doc := range idx.documents {
		score := cosineSimilarity(query, doc.Vector)
		results = append(results, VectorResult{
			ID:    id,
			Score: score,
			Data:  doc.Data,
		})
	}

	// Sort by score
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > k {
		results = results[:k]
	}

	return results
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// Embedder generates text embeddings.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float64, error)
}

func createEmbedder(config SearchConfig) (Embedder, error) {
	switch config.EmbeddingProvider {
	case "openai":
		return NewOpenAIEmbedder(config.APIKey, config.EmbeddingModel), nil
	case "local":
		return NewLocalEmbedder(config.EmbeddingDim), nil
	default:
		return NewLocalEmbedder(config.EmbeddingDim), nil
	}
}

// LocalEmbedder generates simple local embeddings (for development).
type LocalEmbedder struct {
	dim int
}

// NewLocalEmbedder creates a local embedder.
func NewLocalEmbedder(dim int) *LocalEmbedder {
	return &LocalEmbedder{dim: dim}
}

// Embed generates a simple hash-based embedding.
func (e *LocalEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	// Simple deterministic embedding based on text hash
	// In production, use actual embedding model
	embedding := make([]float64, e.dim)
	words := strings.Fields(strings.ToLower(text))

	for i, word := range words {
		for j, c := range word {
			idx := (i*31 + j*17 + int(c)) % e.dim
			embedding[idx] += float64(c) / 128.0
		}
	}

	// Normalize
	var norm float64
	for _, v := range embedding {
		norm += v * v
	}
	if norm > 0 {
		norm = math.Sqrt(norm)
		for i := range embedding {
			embedding[i] /= norm
		}
	}

	return embedding, nil
}

// OpenAIEmbedder uses OpenAI's embedding API.
type OpenAIEmbedder struct {
	apiKey string
	model  string
}

// NewOpenAIEmbedder creates an OpenAI embedder.
func NewOpenAIEmbedder(apiKey, model string) *OpenAIEmbedder {
	if model == "" {
		model = "text-embedding-3-small"
	}
	return &OpenAIEmbedder{
		apiKey: apiKey,
		model:  model,
	}
}

// Embed generates embeddings using OpenAI API.
func (e *OpenAIEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	// Would call OpenAI API
	// For now, return placeholder
	return make([]float64, 1536), nil
}

// IntentParser parses search intent from natural language.
type IntentParser struct {
	patterns []IntentPattern
}

// IntentPattern defines a search intent pattern.
type IntentPattern struct {
	Pattern    string
	Intent     SearchIntent
	Extractors []string
}

// SearchIntent represents parsed search intent.
type SearchIntent struct {
	Type    string            `json:"type"`
	Entity  string            `json:"entity"`
	Filters map[string]string `json:"filters"`
	Sort    string            `json:"sort"`
}

// NewIntentParser creates an intent parser.
func NewIntentParser() *IntentParser {
	return &IntentParser{
		patterns: []IntentPattern{
			{Pattern: "find all backup jobs", Intent: SearchIntent{Type: "search", Filters: map[string]string{"category": "backup"}}},
			{Pattern: "show failed jobs", Intent: SearchIntent{Type: "search", Filters: map[string]string{"status": "failed"}}},
			{Pattern: "jobs owned by", Intent: SearchIntent{Type: "search"}},
			{Pattern: "jobs in namespace", Intent: SearchIntent{Type: "search"}},
		},
	}
}

// Parse parses search intent from natural language.
func (p *IntentParser) Parse(query string) (*SearchIntent, error) {
	query = strings.ToLower(query)

	for _, pattern := range p.patterns {
		if strings.Contains(query, pattern.Pattern) {
			intent := pattern.Intent
			return &intent, nil
		}
	}

	// Default to simple search
	return &SearchIntent{
		Type: "search",
	}, nil
}

// AutocompleteEngine provides search autocomplete.
type AutocompleteEngine struct {
	search *SemanticSearch
}

// NewAutocompleteEngine creates an autocomplete engine.
func NewAutocompleteEngine(search *SemanticSearch) *AutocompleteEngine {
	return &AutocompleteEngine{search: search}
}

// Suggest returns autocomplete suggestions.
func (e *AutocompleteEngine) Suggest(ctx context.Context, prefix string, limit int) ([]Suggestion, error) {
	suggestions := make([]Suggestion, 0)

	// Search with prefix
	results, err := e.search.Search(ctx, prefix, SearchOptions{MaxResults: limit * 2})
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	for _, r := range results.Results {
		// Job name suggestions
		if strings.HasPrefix(strings.ToLower(r.Job.Name), strings.ToLower(prefix)) {
			if !seen[r.Job.Name] {
				suggestions = append(suggestions, Suggestion{
					Text:  r.Job.Name,
					Type:  "job",
					Score: r.Score,
				})
				seen[r.Job.Name] = true
			}
		}

		// Tag suggestions
		for _, tag := range r.Job.Tags {
			if strings.HasPrefix(strings.ToLower(tag), strings.ToLower(prefix)) {
				key := "tag:" + tag
				if !seen[key] {
					suggestions = append(suggestions, Suggestion{
						Text:  tag,
						Type:  "tag",
						Score: r.Score * 0.8,
					})
					seen[key] = true
				}
			}
		}

		if len(suggestions) >= limit {
			break
		}
	}

	// Sort by score
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Score > suggestions[j].Score
	})

	if len(suggestions) > limit {
		suggestions = suggestions[:limit]
	}

	return suggestions, nil
}

// Suggestion is an autocomplete suggestion.
type Suggestion struct {
	Text  string  `json:"text"`
	Type  string  `json:"type"`
	Score float64 `json:"score"`
}

// ExportIndex exports the index to JSON.
func (s *SemanticSearch) ExportIndex() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	docs := make([]*JobDocument, 0)
	s.index.ForEach(func(id string, doc *IndexDocument) {
		if job, ok := doc.Data.(*JobDocument); ok {
			docs = append(docs, job)
		}
	})

	return json.Marshal(docs)
}

// ImportIndex imports jobs from JSON.
func (s *SemanticSearch) ImportIndex(ctx context.Context, data []byte) error {
	var docs []*JobDocument
	if err := json.Unmarshal(data, &docs); err != nil {
		return err
	}

	for _, doc := range docs {
		if err := s.IndexJob(ctx, doc); err != nil {
			return err
		}
	}

	return nil
}
