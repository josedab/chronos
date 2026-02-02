# Community Marketplace

The Chronos Marketplace is a community-driven registry of workflow templates, allowing users to share and discover pre-built automation solutions.

## Overview

- **Browse Templates**: Discover workflows for common use cases
- **Ratings & Reviews**: Community feedback on template quality
- **Verified Publishers**: Trust indicators for template authors
- **One-Click Install**: Deploy templates to your workspace instantly

## Template Categories

| Category | Description | Examples |
|----------|-------------|----------|
| `etl` | Data extraction, transformation, and loading | Database sync, API ingestion |
| `ci_cd` | Continuous integration and deployment | Build triggers, deployment gates |
| `monitoring` | System and application monitoring | Health checks, metric collection |
| `maintenance` | System maintenance tasks | Cleanup jobs, backup verification |
| `reporting` | Automated report generation | Daily summaries, weekly analytics |
| `incident_response` | Incident management automation | Alert escalation, runbook execution |
| `data_pipeline` | Data processing workflows | ETL pipelines, data validation |
| `notification` | Notification and alerting | Slack alerts, email digests |

## API Reference

### Search Templates

```http
GET /api/v1/marketplace/templates/search?q=backup&category=maintenance&sort=rating
```

Query Parameters:
- `q` - Search term
- `category` - Filter by category
- `tags` - Filter by tags (comma-separated)
- `verified` - Only verified publishers (true/false)
- `featured` - Only featured templates (true/false)
- `min_rating` - Minimum rating (1-5)
- `sort` - Sort by: `relevance`, `downloads`, `rating`, `newest`, `updated`
- `limit` - Results per page (default: 20, max: 100)
- `offset` - Pagination offset

Response:
```json
{
  "total_count": 42,
  "templates": [
    {
      "id": "tmpl_abc123",
      "name": "Daily Database Backup",
      "description": "Automated PostgreSQL backup with S3 upload",
      "category": "maintenance",
      "author": "chronos-official",
      "version": "1.2.0",
      "rating": 4.8,
      "rating_count": 156,
      "downloads": 12500,
      "verified": true,
      "featured": true,
      "tags": ["backup", "postgresql", "s3"]
    }
  ]
}
```

### Get Template

```http
GET /api/v1/marketplace/templates/{id}
```

### Trending Templates

```http
GET /api/v1/marketplace/templates/trending?limit=10
```

### Featured Templates

```http
GET /api/v1/marketplace/templates/featured
```

### Template Ratings

#### Get Ratings

```http
GET /api/v1/marketplace/templates/{id}/ratings?sort=helpful&limit=20
```

Sort options: `newest`, `oldest`, `highest`, `lowest`, `helpful`

Response:
```json
{
  "ratings": [
    {
      "id": "rating_xyz789",
      "user_id": "user_123",
      "score": 5,
      "title": "Excellent template!",
      "comment": "Easy to configure and works perfectly with our setup.",
      "helpful": 24,
      "not_helpful": 2,
      "verified": true,
      "created_at": "2024-01-10T15:30:00Z",
      "response": {
        "content": "Thanks for the feedback!",
        "responded_at": "2024-01-11T09:00:00Z"
      }
    }
  ]
}
```

#### Get Rating Stats

```http
GET /api/v1/marketplace/templates/{id}/stats
```

Response:
```json
{
  "template_id": "tmpl_abc123",
  "average_rating": 4.8,
  "total_ratings": 156,
  "distribution": {
    "1": 2,
    "2": 3,
    "3": 8,
    "4": 28,
    "5": 115
  },
  "trend_score": 0.15,
  "last_rating_at": "2024-01-15T10:00:00Z"
}
```

#### Submit Rating

```http
POST /api/v1/marketplace/templates/{id}/rate
Content-Type: application/json
X-User-ID: user_123

{
  "score": 5,
  "title": "Great template!",
  "comment": "Works perfectly for our use case."
}
```

Note: Ratings below 3 stars require a comment.

#### Mark Rating Helpful

```http
POST /api/v1/marketplace/ratings/{ratingId}/helpful?helpful=true
```

### Publishers

#### List Publishers

```http
GET /api/v1/marketplace/publishers
```

#### Get Publisher

```http
GET /api/v1/marketplace/publishers/{id}
```

Response:
```json
{
  "id": "pub_abc123",
  "name": "chronos-official",
  "display_name": "Chronos Official",
  "description": "Official templates from the Chronos team",
  "verified": true,
  "verified_at": "2024-01-01T00:00:00Z",
  "trust_score": 0.95,
  "template_count": 25,
  "total_downloads": 150000,
  "average_rating": 4.7,
  "badges": [
    {
      "id": "verified",
      "name": "Verified Publisher",
      "icon": "✓"
    },
    {
      "id": "top_contributor",
      "name": "Top Contributor",
      "icon": "🏆"
    }
  ]
}
```

#### Get Publisher Templates

```http
GET /api/v1/marketplace/publishers/{id}/templates
```

## Publisher Verification

Publishers can be verified through multiple methods:

| Method | Description |
|--------|-------------|
| `domain` | Verify ownership via DNS TXT record |
| `github` | OAuth verification with GitHub account |
| `email` | Email confirmation with code |
| `manual` | Manual review by Chronos team |

Verification benefits:
- ✅ Verified badge on profile and templates
- 📈 Higher visibility in search results
- 🏆 Eligible for featured placement
- 🔒 Increased trust score

## Template Submission

### Submit New Template

```http
POST /api/v1/marketplace/templates/submit
Content-Type: application/json
X-Publisher-ID: pub_123

{
  "name": "My Workflow Template",
  "description": "Description of what this template does",
  "category": "monitoring",
  "tags": ["health-check", "alerting"],
  "steps": [...],
  "variables": [...],
  "license": "MIT",
  "repository": "https://github.com/user/repo"
}
```

### Submission Review Process

1. **Automated Checks**
   - Schema validation
   - Security scanning
   - Dependency verification

2. **Manual Review**
   - Code quality assessment
   - Documentation completeness
   - Best practices compliance

3. **Approval/Feedback**
   - Approved → Published to marketplace
   - Needs Revision → Feedback provided
   - Rejected → Reason provided

## Code Integration

### Basic Usage

```go
import (
    "github.com/chronos/chronos/internal/marketplace"
)

// Create registry and marketplace service
registry := marketplace.NewWorkflowRegistry()
svc := marketplace.NewMarketplaceService(registry)

// Search templates
result, _ := svc.SearchTemplates(marketplace.SearchQuery{
    Category: marketplace.WorkflowCategoryMonitoring,
    MinRating: 4.0,
    SortBy: "downloads",
})

// Get ratings
ratings, _ := svc.GetRatings("tmpl_abc123", marketplace.RatingQueryOptions{
    SortBy: "helpful",
    Limit: 10,
})

// Submit rating
rating, _ := svc.SubmitRating("user_123", "tmpl_abc123", 5, "Great!", "Works perfectly")
```

### Mount Routes

```go
import (
    "github.com/chronos/chronos/internal/api"
    "github.com/chronos/chronos/internal/marketplace"
)

registry := marketplace.NewWorkflowRegistry()
svc := marketplace.NewMarketplaceService(registry)

router := api.NewRouter(handler, logger)
api.MountMarketplaceRoutes(router, svc.HTTPHandler())
```

## Relevance Scoring

Search results are ranked by relevance score:

| Factor | Weight | Description |
|--------|--------|-------------|
| Rating | 40% | Average user rating (1-5 stars) |
| Downloads | 30% | Number of installations |
| Verified | 15% | Publisher verification status |
| Featured | 10% | Chronos team featured selection |
| Recency | 5% | Time since last update |

## Best Practices for Template Authors

1. **Clear Documentation**
   - Detailed description
   - Variable explanations
   - Example configurations

2. **Sensible Defaults**
   - Default values for all variables
   - Safe timeout and retry settings

3. **Error Handling**
   - Graceful failure handling
   - Meaningful error messages

4. **Versioning**
   - Semantic versioning (MAJOR.MINOR.PATCH)
   - Changelog in repository

5. **Testing**
   - Test with various inputs
   - Document known limitations
