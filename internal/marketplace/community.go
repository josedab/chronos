// Package marketplace provides workflow templates for Chronos.
package marketplace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Common errors.
var (
	ErrPublisherNotFound     = errors.New("publisher not found")
	ErrPublisherExists       = errors.New("publisher already exists")
	ErrUnauthorized          = errors.New("unauthorized")
	ErrRatingExists          = errors.New("user has already rated this template")
	ErrInvalidRating         = errors.New("rating must be between 1 and 5")
	ErrReviewRequired        = errors.New("comment is required for ratings below 3")
)

// Publisher represents a verified template publisher.
type Publisher struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	DisplayName     string          `json:"display_name"`
	Email           string          `json:"email"`
	Website         string          `json:"website,omitempty"`
	Description     string          `json:"description,omitempty"`
	AvatarURL       string          `json:"avatar_url,omitempty"`
	GithubUsername  string          `json:"github_username,omitempty"`
	Verified        bool            `json:"verified"`
	VerifiedAt      *time.Time      `json:"verified_at,omitempty"`
	VerificationID  string          `json:"verification_id,omitempty"`
	TrustScore      float64         `json:"trust_score"`
	TemplateCount   int             `json:"template_count"`
	TotalDownloads  int             `json:"total_downloads"`
	TotalRatings    int             `json:"total_ratings"`
	AverageRating   float64         `json:"average_rating"`
	Badges          []PublisherBadge `json:"badges,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// PublisherBadge represents achievements or certifications.
type PublisherBadge struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Icon        string    `json:"icon"`
	AwardedAt   time.Time `json:"awarded_at"`
}

// Rating represents a user rating for a template.
type Rating struct {
	ID          string     `json:"id"`
	TemplateID  string     `json:"template_id"`
	UserID      string     `json:"user_id"`
	Score       int        `json:"score"` // 1-5
	Title       string     `json:"title,omitempty"`
	Comment     string     `json:"comment,omitempty"`
	Helpful     int        `json:"helpful"`
	NotHelpful  int        `json:"not_helpful"`
	Verified    bool       `json:"verified"` // Verified purchase/install
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Response    *PublisherResponse `json:"response,omitempty"`
}

// PublisherResponse is a publisher's response to a rating.
type PublisherResponse struct {
	Content     string    `json:"content"`
	RespondedAt time.Time `json:"responded_at"`
}

// RatingStats contains aggregated rating statistics.
type RatingStats struct {
	TemplateID    string           `json:"template_id"`
	AverageRating float64          `json:"average_rating"`
	TotalRatings  int              `json:"total_ratings"`
	Distribution  map[int]int      `json:"distribution"` // score -> count
	TrendScore    float64          `json:"trend_score"`  // Recent rating trend
	LastRatingAt  *time.Time       `json:"last_rating_at,omitempty"`
}

// MarketplaceService provides public registry API.
type MarketplaceService struct {
	registry   *WorkflowRegistry
	publishers map[string]*Publisher
	ratings    map[string][]*Rating      // templateID -> ratings
	userRatings map[string]map[string]*Rating // userID -> templateID -> rating
	submissions []*TemplateSubmission
	mu         sync.RWMutex
}

// NewMarketplaceService creates a new marketplace service.
func NewMarketplaceService(registry *WorkflowRegistry) *MarketplaceService {
	return &MarketplaceService{
		registry:    registry,
		publishers:  make(map[string]*Publisher),
		ratings:     make(map[string][]*Rating),
		userRatings: make(map[string]map[string]*Rating),
		submissions: make([]*TemplateSubmission, 0),
	}
}

// RegisterPublisher registers a new publisher.
func (s *MarketplaceService) RegisterPublisher(pub *Publisher) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if pub.ID == "" {
		pub.ID = uuid.New().String()
	}

	// Check if publisher name exists
	for _, p := range s.publishers {
		if p.Name == pub.Name {
			return ErrPublisherExists
		}
	}

	pub.CreatedAt = time.Now()
	pub.UpdatedAt = time.Now()
	pub.TrustScore = 0.5 // Start with neutral trust
	s.publishers[pub.ID] = pub

	return nil
}

// GetPublisher retrieves a publisher by ID.
func (s *MarketplaceService) GetPublisher(id string) (*Publisher, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pub, ok := s.publishers[id]
	if !ok {
		return nil, ErrPublisherNotFound
	}
	return pub, nil
}

// VerifyPublisher verifies a publisher.
func (s *MarketplaceService) VerifyPublisher(ctx context.Context, publisherID string, verification PublisherVerification) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pub, ok := s.publishers[publisherID]
	if !ok {
		return ErrPublisherNotFound
	}

	// Verify based on method
	verified := false
	switch verification.Method {
	case VerificationMethodDomain:
		verified = s.verifyDomain(verification.DomainProof)
	case VerificationMethodGitHub:
		verified = s.verifyGitHub(verification.GitHubToken, pub.GithubUsername)
	case VerificationMethodEmail:
		verified = s.verifyEmail(verification.EmailCode)
	case VerificationMethodManual:
		// Manual review by admin
		verified = verification.AdminApproved
	}

	if verified {
		now := time.Now()
		pub.Verified = true
		pub.VerifiedAt = &now
		pub.VerificationID = uuid.New().String()
		pub.TrustScore = 0.8 // Bump trust score on verification
		pub.UpdatedAt = now

		// Award verification badge
		pub.Badges = append(pub.Badges, PublisherBadge{
			ID:          "verified",
			Name:        "Verified Publisher",
			Description: "This publisher has verified their identity",
			Icon:        "✓",
			AwardedAt:   now,
		})
	}

	return nil
}

// PublisherVerification contains verification details.
type PublisherVerification struct {
	Method        VerificationMethod `json:"method"`
	DomainProof   string            `json:"domain_proof,omitempty"`
	GitHubToken   string            `json:"github_token,omitempty"`
	EmailCode     string            `json:"email_code,omitempty"`
	AdminApproved bool              `json:"admin_approved,omitempty"`
}

// VerificationMethod represents verification methods.
type VerificationMethod string

const (
	VerificationMethodDomain VerificationMethod = "domain"
	VerificationMethodGitHub VerificationMethod = "github"
	VerificationMethodEmail  VerificationMethod = "email"
	VerificationMethodManual VerificationMethod = "manual"
)

func (s *MarketplaceService) verifyDomain(proof string) bool {
	// In production: Verify DNS TXT record or meta tag
	return proof != ""
}

func (s *MarketplaceService) verifyGitHub(token, username string) bool {
	// In production: Verify GitHub OAuth token
	return token != "" && username != ""
}

func (s *MarketplaceService) verifyEmail(code string) bool {
	// In production: Verify email confirmation code
	return code != ""
}

// SubmitRating submits a rating for a template.
func (s *MarketplaceService) SubmitRating(userID, templateID string, score int, title, comment string) (*Rating, error) {
	if score < 1 || score > 5 {
		return nil, ErrInvalidRating
	}

	if score < 3 && comment == "" {
		return nil, ErrReviewRequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if user already rated
	if userRatings, ok := s.userRatings[userID]; ok {
		if _, exists := userRatings[templateID]; exists {
			return nil, ErrRatingExists
		}
	}

	// Verify template exists
	if _, err := s.registry.GetWorkflowTemplate(templateID); err != nil {
		return nil, ErrTemplateNotFound
	}

	rating := &Rating{
		ID:         uuid.New().String(),
		TemplateID: templateID,
		UserID:     userID,
		Score:      score,
		Title:      title,
		Comment:    comment,
		Verified:   s.hasVerifiedInstall(userID, templateID),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// Store rating
	s.ratings[templateID] = append(s.ratings[templateID], rating)
	if s.userRatings[userID] == nil {
		s.userRatings[userID] = make(map[string]*Rating)
	}
	s.userRatings[userID][templateID] = rating

	// Update template rating
	s.updateTemplateRating(templateID)

	return rating, nil
}

// UpdateRating updates an existing rating.
func (s *MarketplaceService) UpdateRating(userID, templateID string, score int, title, comment string) (*Rating, error) {
	if score < 1 || score > 5 {
		return nil, ErrInvalidRating
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Find existing rating
	userRatings, ok := s.userRatings[userID]
	if !ok {
		return nil, errors.New("no rating found")
	}
	rating, ok := userRatings[templateID]
	if !ok {
		return nil, errors.New("no rating found for this template")
	}

	rating.Score = score
	rating.Title = title
	rating.Comment = comment
	rating.UpdatedAt = time.Now()

	// Recalculate template rating
	s.updateTemplateRating(templateID)

	return rating, nil
}

// GetRatings retrieves ratings for a template.
func (s *MarketplaceService) GetRatings(templateID string, opts RatingQueryOptions) ([]*Rating, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ratings, ok := s.ratings[templateID]
	if !ok {
		return []*Rating{}, nil
	}

	// Copy and sort
	result := make([]*Rating, len(ratings))
	copy(result, ratings)

	switch opts.SortBy {
	case "newest":
		sort.Slice(result, func(i, j int) bool {
			return result[i].CreatedAt.After(result[j].CreatedAt)
		})
	case "oldest":
		sort.Slice(result, func(i, j int) bool {
			return result[i].CreatedAt.Before(result[j].CreatedAt)
		})
	case "highest":
		sort.Slice(result, func(i, j int) bool {
			return result[i].Score > result[j].Score
		})
	case "lowest":
		sort.Slice(result, func(i, j int) bool {
			return result[i].Score < result[j].Score
		})
	case "helpful":
		sort.Slice(result, func(i, j int) bool {
			return result[i].Helpful > result[j].Helpful
		})
	}

	// Filter
	if opts.MinScore > 0 {
		filtered := result[:0]
		for _, r := range result {
			if r.Score >= opts.MinScore {
				filtered = append(filtered, r)
			}
		}
		result = filtered
	}

	if opts.VerifiedOnly {
		filtered := result[:0]
		for _, r := range result {
			if r.Verified {
				filtered = append(filtered, r)
			}
		}
		result = filtered
	}

	// Pagination
	if opts.Offset > 0 && opts.Offset < len(result) {
		result = result[opts.Offset:]
	}
	if opts.Limit > 0 && opts.Limit < len(result) {
		result = result[:opts.Limit]
	}

	return result, nil
}

// RatingQueryOptions configures rating queries.
type RatingQueryOptions struct {
	SortBy       string `json:"sort_by"`       // newest, oldest, highest, lowest, helpful
	MinScore     int    `json:"min_score"`
	VerifiedOnly bool   `json:"verified_only"`
	Limit        int    `json:"limit"`
	Offset       int    `json:"offset"`
}

// GetRatingStats returns rating statistics for a template.
func (s *MarketplaceService) GetRatingStats(templateID string) (*RatingStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ratings, ok := s.ratings[templateID]
	if !ok || len(ratings) == 0 {
		return &RatingStats{
			TemplateID:   templateID,
			Distribution: map[int]int{1: 0, 2: 0, 3: 0, 4: 0, 5: 0},
		}, nil
	}

	stats := &RatingStats{
		TemplateID:   templateID,
		TotalRatings: len(ratings),
		Distribution: map[int]int{1: 0, 2: 0, 3: 0, 4: 0, 5: 0},
	}

	var total int
	var lastRating *time.Time
	var recentTotal, recentCount int

	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)

	for _, r := range ratings {
		total += r.Score
		stats.Distribution[r.Score]++

		if lastRating == nil || r.CreatedAt.After(*lastRating) {
			lastRating = &r.CreatedAt
		}

		// Calculate trend from recent ratings
		if r.CreatedAt.After(thirtyDaysAgo) {
			recentTotal += r.Score
			recentCount++
		}
	}

	stats.AverageRating = float64(total) / float64(len(ratings))
	stats.LastRatingAt = lastRating

	// Calculate trend (positive if recent ratings are above average)
	if recentCount > 0 {
		recentAvg := float64(recentTotal) / float64(recentCount)
		stats.TrendScore = recentAvg - stats.AverageRating
	}

	return stats, nil
}

// MarkHelpful marks a rating as helpful/not helpful.
func (s *MarketplaceService) MarkHelpful(ratingID string, helpful bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, ratings := range s.ratings {
		for _, r := range ratings {
			if r.ID == ratingID {
				if helpful {
					r.Helpful++
				} else {
					r.NotHelpful++
				}
				return nil
			}
		}
	}

	return errors.New("rating not found")
}

// RespondToRating allows publisher to respond to a rating.
func (s *MarketplaceService) RespondToRating(publisherID, ratingID, response string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, ratings := range s.ratings {
		for _, r := range ratings {
			if r.ID == ratingID {
				// Verify publisher owns the template
				template, err := s.registry.GetWorkflowTemplate(r.TemplateID)
				if err != nil {
					return err
				}

				pub, ok := s.publishers[publisherID]
				if !ok || template.Author != pub.Name {
					return ErrUnauthorized
				}

				r.Response = &PublisherResponse{
					Content:     response,
					RespondedAt: time.Now(),
				}
				return nil
			}
		}
	}

	return errors.New("rating not found")
}

func (s *MarketplaceService) updateTemplateRating(templateID string) {
	ratings := s.ratings[templateID]
	if len(ratings) == 0 {
		return
	}

	var total int
	for _, r := range ratings {
		total += r.Score
	}

	avg := float64(total) / float64(len(ratings))

	// Update registry (this would need access to modify the template)
	s.registry.mu.Lock()
	if t, ok := s.registry.templates[templateID]; ok {
		t.Rating = avg
		t.RatingCount = len(ratings)
	}
	if t, ok := s.registry.community[templateID]; ok {
		t.Rating = avg
		t.RatingCount = len(ratings)
	}
	s.registry.mu.Unlock()
}

func (s *MarketplaceService) hasVerifiedInstall(userID, templateID string) bool {
	// In production: Check if user has actually installed/used the template
	return true
}

// SearchTemplates performs advanced search on templates.
func (s *MarketplaceService) SearchTemplates(query SearchQuery) (*SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filter := WorkflowFilter{
		Category:   query.Category,
		Tags:       query.Tags,
		Author:     query.Author,
		Verified:   query.Verified,
		Featured:   query.Featured,
		MinRating:  query.MinRating,
		SearchTerm: query.SearchTerm,
	}

	templates := s.registry.ListWorkflowTemplates(filter)

	// Sort results
	switch query.SortBy {
	case "downloads":
		sort.Slice(templates, func(i, j int) bool {
			return templates[i].Downloads > templates[j].Downloads
		})
	case "rating":
		sort.Slice(templates, func(i, j int) bool {
			return templates[i].Rating > templates[j].Rating
		})
	case "newest":
		sort.Slice(templates, func(i, j int) bool {
			return templates[i].CreatedAt.After(templates[j].CreatedAt)
		})
	case "updated":
		sort.Slice(templates, func(i, j int) bool {
			return templates[i].UpdatedAt.After(templates[j].UpdatedAt)
		})
	default:
		// Relevance - weighted score
		sort.Slice(templates, func(i, j int) bool {
			scoreI := s.calculateRelevanceScore(templates[i])
			scoreJ := s.calculateRelevanceScore(templates[j])
			return scoreI > scoreJ
		})
	}

	result := &SearchResult{
		TotalCount: len(templates),
		Templates:  templates,
	}

	// Pagination
	if query.Offset > 0 && query.Offset < len(result.Templates) {
		result.Templates = result.Templates[query.Offset:]
	}
	if query.Limit > 0 && query.Limit < len(result.Templates) {
		result.Templates = result.Templates[:query.Limit]
	}

	return result, nil
}

func (s *MarketplaceService) calculateRelevanceScore(t *WorkflowTemplate) float64 {
	score := 0.0

	// Rating weight (40%)
	score += (t.Rating / 5.0) * 40

	// Downloads weight (30%)
	downloadScore := float64(t.Downloads) / 1000.0 // Normalize
	if downloadScore > 1 {
		downloadScore = 1
	}
	score += downloadScore * 30

	// Verified weight (15%)
	if t.Verified {
		score += 15
	}

	// Featured weight (10%)
	if t.Featured {
		score += 10
	}

	// Recency weight (5%)
	daysSinceUpdate := time.Since(t.UpdatedAt).Hours() / 24
	recencyScore := 1.0 - (daysSinceUpdate / 365.0)
	if recencyScore < 0 {
		recencyScore = 0
	}
	score += recencyScore * 5

	return score
}

// SearchQuery defines search parameters.
type SearchQuery struct {
	SearchTerm string           `json:"search_term"`
	Category   WorkflowCategory `json:"category,omitempty"`
	Tags       []string         `json:"tags,omitempty"`
	Author     string           `json:"author,omitempty"`
	Verified   *bool            `json:"verified,omitempty"`
	Featured   *bool            `json:"featured,omitempty"`
	MinRating  float64          `json:"min_rating,omitempty"`
	SortBy     string           `json:"sort_by"` // relevance, downloads, rating, newest, updated
	Limit      int              `json:"limit"`
	Offset     int              `json:"offset"`
}

// SearchResult contains search results.
type SearchResult struct {
	TotalCount int                 `json:"total_count"`
	Templates  []*WorkflowTemplate `json:"templates"`
}

// GetTrendingTemplates returns trending templates.
func (s *MarketplaceService) GetTrendingTemplates(limit int) []*WorkflowTemplate {
	s.mu.RLock()
	defer s.mu.RUnlock()

	all := s.registry.ListWorkflowTemplates(WorkflowFilter{})

	// Sort by recent downloads/ratings trend
	sort.Slice(all, func(i, j int) bool {
		// Simplified: use downloads * rating
		scoreI := float64(all[i].Downloads) * all[i].Rating
		scoreJ := float64(all[j].Downloads) * all[j].Rating
		return scoreI > scoreJ
	})

	if limit > 0 && limit < len(all) {
		all = all[:limit]
	}

	return all
}

// GetFeaturedTemplates returns featured templates.
func (s *MarketplaceService) GetFeaturedTemplates() []*WorkflowTemplate {
	featured := true
	return s.registry.ListWorkflowTemplates(WorkflowFilter{Featured: &featured})
}

// HTTPHandler returns HTTP handlers for the marketplace API.
func (s *MarketplaceService) HTTPHandler() http.Handler {
	r := chi.NewRouter()

	// Templates
	r.Get("/templates", s.handleListTemplates)
	r.Get("/templates/search", s.handleSearchTemplates)
	r.Get("/templates/trending", s.handleTrendingTemplates)
	r.Get("/templates/featured", s.handleFeaturedTemplates)
	r.Get("/templates/{id}", s.handleGetTemplate)
	r.Get("/templates/{id}/ratings", s.handleGetRatings)
	r.Get("/templates/{id}/stats", s.handleGetRatingStats)
	r.Post("/templates/{id}/rate", s.handleRateTemplate)

	// Publishers
	r.Get("/publishers", s.handleListPublishers)
	r.Get("/publishers/{id}", s.handleGetPublisher)
	r.Get("/publishers/{id}/templates", s.handleGetPublisherTemplates)

	// Ratings
	r.Post("/ratings/{id}/helpful", s.handleMarkHelpful)
	r.Post("/ratings/{id}/respond", s.handleRespondToRating)

	return r
}

func (s *MarketplaceService) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	filter := WorkflowFilter{}
	// Parse query params
	if cat := r.URL.Query().Get("category"); cat != "" {
		filter.Category = WorkflowCategory(cat)
	}

	templates := s.registry.ListWorkflowTemplates(filter)
	s.writeJSON(w, http.StatusOK, templates)
}

func (s *MarketplaceService) handleSearchTemplates(w http.ResponseWriter, r *http.Request) {
	query := SearchQuery{
		SearchTerm: r.URL.Query().Get("q"),
		SortBy:     r.URL.Query().Get("sort"),
		Limit:      20,
	}

	if cat := r.URL.Query().Get("category"); cat != "" {
		query.Category = WorkflowCategory(cat)
	}

	result, err := s.SearchTemplates(query)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, result)
}

func (s *MarketplaceService) handleTrendingTemplates(w http.ResponseWriter, r *http.Request) {
	templates := s.GetTrendingTemplates(10)
	s.writeJSON(w, http.StatusOK, templates)
}

func (s *MarketplaceService) handleFeaturedTemplates(w http.ResponseWriter, r *http.Request) {
	templates := s.GetFeaturedTemplates()
	s.writeJSON(w, http.StatusOK, templates)
}

func (s *MarketplaceService) handleGetTemplate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	template, err := s.registry.GetWorkflowTemplate(id)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "Template not found")
		return
	}
	s.writeJSON(w, http.StatusOK, template)
}

func (s *MarketplaceService) handleGetRatings(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	opts := RatingQueryOptions{
		SortBy: r.URL.Query().Get("sort"),
		Limit:  20,
	}

	ratings, err := s.GetRatings(id, opts)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, ratings)
}

func (s *MarketplaceService) handleGetRatingStats(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	stats, err := s.GetRatingStats(id)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, stats)
}

func (s *MarketplaceService) handleRateTemplate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		s.writeError(w, http.StatusUnauthorized, "User ID required")
		return
	}

	var req struct {
		Score   int    `json:"score"`
		Title   string `json:"title"`
		Comment string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	rating, err := s.SubmitRating(userID, id, req.Score, req.Title, req.Comment)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, ErrRatingExists) {
			status = http.StatusConflict
		}
		s.writeError(w, status, err.Error())
		return
	}

	s.writeJSON(w, http.StatusCreated, rating)
}

func (s *MarketplaceService) handleListPublishers(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	publishers := make([]*Publisher, 0, len(s.publishers))
	for _, p := range s.publishers {
		publishers = append(publishers, p)
	}

	s.writeJSON(w, http.StatusOK, publishers)
}

func (s *MarketplaceService) handleGetPublisher(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	pub, err := s.GetPublisher(id)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "Publisher not found")
		return
	}
	s.writeJSON(w, http.StatusOK, pub)
}

func (s *MarketplaceService) handleGetPublisherTemplates(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	pub, err := s.GetPublisher(id)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "Publisher not found")
		return
	}

	templates := s.registry.ListWorkflowTemplates(WorkflowFilter{Author: pub.Name})
	s.writeJSON(w, http.StatusOK, templates)
}

func (s *MarketplaceService) handleMarkHelpful(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	helpful := r.URL.Query().Get("helpful") != "false"

	if err := s.MarkHelpful(id, helpful); err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *MarketplaceService) handleRespondToRating(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	publisherID := r.Header.Get("X-Publisher-ID")
	if publisherID == "" {
		s.writeError(w, http.StatusUnauthorized, "Publisher ID required")
		return
	}

	var req struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	if err := s.RespondToRating(publisherID, id, req.Response); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, ErrUnauthorized) {
			status = http.StatusForbidden
		}
		s.writeError(w, status, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *MarketplaceService) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *MarketplaceService) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, map[string]string{"error": message})
}

// GenerateTemplateHash generates a content hash for a template.
func GenerateTemplateHash(t *WorkflowTemplate) string {
	data, _ := json.Marshal(struct {
		Name    string
		Steps   []WorkflowStepTemplate
		Version string
	}{t.Name, t.Steps, t.Version})
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
