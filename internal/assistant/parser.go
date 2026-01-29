// Package assistant provides AI-powered job creation assistance for Chronos.
package assistant

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ParseResult contains the parsed job configuration.
type ParseResult struct {
	Schedule    string            `json:"schedule"`
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
	Timezone    string            `json:"timezone,omitempty"`
	Confidence  float64           `json:"confidence"` // 0.0-1.0
	Suggestions []string          `json:"suggestions,omitempty"`
	Parsed      map[string]string `json:"parsed"`
}

// Assistant parses natural language job descriptions.
type Assistant struct {
	defaultTimezone string
}

// New creates a new assistant.
func New(defaultTimezone string) *Assistant {
	if defaultTimezone == "" {
		defaultTimezone = "UTC"
	}
	return &Assistant{defaultTimezone: defaultTimezone}
}

// Parse parses a natural language job description into a cron schedule.
func (a *Assistant) Parse(input string) (*ParseResult, error) {
	input = strings.ToLower(strings.TrimSpace(input))

	result := &ParseResult{
		Parsed:      make(map[string]string),
		Confidence:  0.0,
		Suggestions: []string{},
	}

	// Extract timezone
	result.Timezone = a.extractTimezone(input)

	// Try to parse common patterns
	if schedule, conf := a.parseEveryPattern(input); schedule != "" {
		result.Schedule = schedule
		result.Confidence = conf
		return result, nil
	}

	if schedule, conf := a.parseDailyPattern(input); schedule != "" {
		result.Schedule = schedule
		result.Confidence = conf
		return result, nil
	}

	if schedule, conf := a.parseWeeklyPattern(input); schedule != "" {
		result.Schedule = schedule
		result.Confidence = conf
		return result, nil
	}

	if schedule, conf := a.parseMonthlyPattern(input); schedule != "" {
		result.Schedule = schedule
		result.Confidence = conf
		return result, nil
	}

	if schedule, conf := a.parseTimePattern(input); schedule != "" {
		result.Schedule = schedule
		result.Confidence = conf
		return result, nil
	}

	// Low confidence guess
	result.Schedule = "0 * * * *" // Default to hourly
	result.Confidence = 0.1
	result.Suggestions = append(result.Suggestions,
		"Could not parse schedule. Defaulting to hourly.",
		"Try: 'every 5 minutes', 'daily at 9am', 'every Monday at 3pm'")

	return result, nil
}

// parseEveryPattern handles "every X minutes/hours" patterns.
func (a *Assistant) parseEveryPattern(input string) (string, float64) {
	patterns := []struct {
		regex    *regexp.Regexp
		schedule func(match []string) string
	}{
		{
			regexp.MustCompile(`every\s+(\d+)\s*min(ute)?s?`),
			func(m []string) string {
				n, _ := strconv.Atoi(m[1])
				if n > 0 && n < 60 {
					return "*/" + m[1] + " * * * *"
				}
				return ""
			},
		},
		{
			regexp.MustCompile(`every\s+(\d+)\s*hours?`),
			func(m []string) string {
				n, _ := strconv.Atoi(m[1])
				if n > 0 && n < 24 {
					return "0 */" + m[1] + " * * *"
				}
				return ""
			},
		},
		{
			regexp.MustCompile(`every\s+minute`),
			func(m []string) string { return "* * * * *" },
		},
		{
			regexp.MustCompile(`every\s+hour`),
			func(m []string) string { return "0 * * * *" },
		},
		{
			regexp.MustCompile(`every\s+day`),
			func(m []string) string { return "0 0 * * *" },
		},
		{
			regexp.MustCompile(`every\s+week`),
			func(m []string) string { return "0 0 * * 0" },
		},
		{
			regexp.MustCompile(`every\s+month`),
			func(m []string) string { return "0 0 1 * *" },
		},
	}

	for _, p := range patterns {
		if match := p.regex.FindStringSubmatch(input); match != nil {
			if schedule := p.schedule(match); schedule != "" {
				return schedule, 0.9
			}
		}
	}

	return "", 0
}

// parseDailyPattern handles "daily at X" patterns.
func (a *Assistant) parseDailyPattern(input string) (string, float64) {
	patterns := []struct {
		regex    *regexp.Regexp
		schedule func(match []string) string
	}{
		{
			regexp.MustCompile(`daily\s+at\s+(\d{1,2}):?(\d{2})?\s*(am|pm)?`),
			func(m []string) string {
				return a.buildTimeSchedule(m[1], m[2], m[3], "* * *")
			},
		},
		{
			regexp.MustCompile(`every\s+day\s+at\s+(\d{1,2}):?(\d{2})?\s*(am|pm)?`),
			func(m []string) string {
				return a.buildTimeSchedule(m[1], m[2], m[3], "* * *")
			},
		},
		{
			regexp.MustCompile(`at\s+(\d{1,2}):?(\d{2})?\s*(am|pm)?\s+every\s+day`),
			func(m []string) string {
				return a.buildTimeSchedule(m[1], m[2], m[3], "* * *")
			},
		},
	}

	for _, p := range patterns {
		if match := p.regex.FindStringSubmatch(input); match != nil {
			if schedule := p.schedule(match); schedule != "" {
				return schedule, 0.85
			}
		}
	}

	return "", 0
}

// parseWeeklyPattern handles weekly schedule patterns.
func (a *Assistant) parseWeeklyPattern(input string) (string, float64) {
	days := map[string]string{
		"sunday": "0", "sun": "0",
		"monday": "1", "mon": "1",
		"tuesday": "2", "tue": "2",
		"wednesday": "3", "wed": "3",
		"thursday": "4", "thu": "4",
		"friday": "5", "fri": "5",
		"saturday": "6", "sat": "6",
	}

	pattern := regexp.MustCompile(`every\s+(sunday|sun|monday|mon|tuesday|tue|wednesday|wed|thursday|thu|friday|fri|saturday|sat)\s+at\s+(\d{1,2}):?(\d{2})?\s*(am|pm)?`)

	if match := pattern.FindStringSubmatch(input); match != nil {
		dayNum := days[match[1]]
		timeSchedule := a.buildTimeSchedule(match[2], match[3], match[4], "* *")
		if timeSchedule != "" {
			return timeSchedule + " " + dayNum, 0.85
		}
	}

	// Also handle "on Mondays" etc.
	for dayName, dayNum := range days {
		if strings.Contains(input, dayName) || strings.Contains(input, dayName+"s") {
			timeMatch := regexp.MustCompile(`at\s+(\d{1,2}):?(\d{2})?\s*(am|pm)?`).FindStringSubmatch(input)
			if timeMatch != nil {
				timeSchedule := a.buildTimeSchedule(timeMatch[1], timeMatch[2], timeMatch[3], "* *")
				if timeSchedule != "" {
					return timeSchedule + " " + dayNum, 0.8
				}
			}
		}
	}

	return "", 0
}

// parseMonthlyPattern handles monthly schedule patterns.
func (a *Assistant) parseMonthlyPattern(input string) (string, float64) {
	patterns := []struct {
		regex    *regexp.Regexp
		schedule func(match []string) string
	}{
		{
			regexp.MustCompile(`on\s+the\s+(\d{1,2})(st|nd|rd|th)?\s+of\s+every\s+month\s+at\s+(\d{1,2}):?(\d{2})?\s*(am|pm)?`),
			func(m []string) string {
				day := m[1]
				return a.buildTimeSchedule(m[3], m[4], m[5], day+" * *")
			},
		},
		{
			regexp.MustCompile(`monthly\s+on\s+day\s+(\d{1,2})\s+at\s+(\d{1,2}):?(\d{2})?\s*(am|pm)?`),
			func(m []string) string {
				day := m[1]
				return a.buildTimeSchedule(m[2], m[3], m[4], day+" * *")
			},
		},
	}

	for _, p := range patterns {
		if match := p.regex.FindStringSubmatch(input); match != nil {
			if schedule := p.schedule(match); schedule != "" {
				return schedule, 0.8
			}
		}
	}

	return "", 0
}

// parseTimePattern handles simple time patterns.
func (a *Assistant) parseTimePattern(input string) (string, float64) {
	pattern := regexp.MustCompile(`at\s+(\d{1,2}):?(\d{2})?\s*(am|pm)?`)
	if match := pattern.FindStringSubmatch(input); match != nil {
		schedule := a.buildTimeSchedule(match[1], match[2], match[3], "* * *")
		if schedule != "" {
			return schedule, 0.6
		}
	}
	return "", 0
}

// buildTimeSchedule constructs a cron schedule from time components.
func (a *Assistant) buildTimeSchedule(hourStr, minStr, ampm, suffix string) string {
	hour, err := strconv.Atoi(hourStr)
	if err != nil || hour < 0 || hour > 23 {
		return ""
	}

	minute := 0
	if minStr != "" {
		minute, _ = strconv.Atoi(minStr)
	}

	// Handle AM/PM
	if ampm == "pm" && hour < 12 {
		hour += 12
	} else if ampm == "am" && hour == 12 {
		hour = 0
	}

	return strconv.Itoa(minute) + " " + strconv.Itoa(hour) + " " + suffix
}

// extractTimezone extracts timezone from input.
func (a *Assistant) extractTimezone(input string) string {
	tzPatterns := []struct {
		pattern  string
		timezone string
	}{
		{"est", "America/New_York"},
		{"eastern", "America/New_York"},
		{"pst", "America/Los_Angeles"},
		{"pacific", "America/Los_Angeles"},
		{"cst", "America/Chicago"},
		{"central", "America/Chicago"},
		{"mst", "America/Denver"},
		{"mountain", "America/Denver"},
		{"utc", "UTC"},
		{"gmt", "UTC"},
		{"london", "Europe/London"},
		{"paris", "Europe/Paris"},
		{"berlin", "Europe/Berlin"},
		{"tokyo", "Asia/Tokyo"},
		{"sydney", "Australia/Sydney"},
	}

	for _, tz := range tzPatterns {
		if strings.Contains(input, tz.pattern) {
			return tz.timezone
		}
	}

	return a.defaultTimezone
}

// Suggest generates schedule suggestions based on common patterns.
func (a *Assistant) Suggest(jobType string) []Suggestion {
	suggestions := []Suggestion{
		{
			Description: "Every 5 minutes",
			Schedule:    "*/5 * * * *",
			UseCase:     "Health checks, monitoring",
		},
		{
			Description: "Every hour",
			Schedule:    "0 * * * *",
			UseCase:     "Data sync, cache refresh",
		},
		{
			Description: "Daily at midnight",
			Schedule:    "0 0 * * *",
			UseCase:     "Daily reports, cleanup",
		},
		{
			Description: "Daily at 9 AM",
			Schedule:    "0 9 * * *",
			UseCase:     "Morning notifications",
		},
		{
			Description: "Every Monday at 9 AM",
			Schedule:    "0 9 * * 1",
			UseCase:     "Weekly reports",
		},
		{
			Description: "First of month at midnight",
			Schedule:    "0 0 1 * *",
			UseCase:     "Monthly billing, reports",
		},
		{
			Description: "Every weekday at 6 PM",
			Schedule:    "0 18 * * 1-5",
			UseCase:     "End of day processing",
		},
	}

	// Filter based on job type
	switch jobType {
	case "monitoring":
		return suggestions[:3]
	case "reporting":
		return []Suggestion{suggestions[2], suggestions[4], suggestions[5]}
	default:
		return suggestions
	}
}

// Suggestion is a schedule suggestion.
type Suggestion struct {
	Description string `json:"description"`
	Schedule    string `json:"schedule"`
	UseCase     string `json:"use_case"`
}

// ValidateCron validates a cron expression.
func ValidateCron(expr string) error {
	parts := strings.Fields(expr)
	if len(parts) < 5 || len(parts) > 6 {
		return ErrInvalidCron
	}
	return nil
}

// ErrInvalidCron is returned for invalid cron expressions.
var ErrInvalidCron = &CronError{Message: "invalid cron expression"}

// CronError represents a cron parsing error.
type CronError struct {
	Message string
}

func (e *CronError) Error() string {
	return e.Message
}

// ExplainCron returns a human-readable explanation of a cron expression.
func ExplainCron(expr string) string {
	parts := strings.Fields(expr)
	if len(parts) < 5 {
		return "Invalid cron expression"
	}

	minute, hour, dom, month, dow := parts[0], parts[1], parts[2], parts[3], parts[4]

	var explanation []string

	// Minute
	if minute == "*" {
		explanation = append(explanation, "every minute")
	} else if strings.HasPrefix(minute, "*/") {
		explanation = append(explanation, "every "+minute[2:]+" minutes")
	} else {
		explanation = append(explanation, "at minute "+minute)
	}

	// Hour
	if hour == "*" {
		explanation = append(explanation, "of every hour")
	} else if strings.HasPrefix(hour, "*/") {
		explanation = append(explanation, "every "+hour[2:]+" hours")
	} else {
		h, _ := strconv.Atoi(hour)
		ampm := "AM"
		if h >= 12 {
			ampm = "PM"
			if h > 12 {
				h -= 12
			}
		}
		if h == 0 {
			h = 12
		}
		explanation = append(explanation, "at "+strconv.Itoa(h)+ampm)
	}

	// Day of month
	if dom != "*" {
		explanation = append(explanation, "on day "+dom)
	}

	// Month
	if month != "*" {
		explanation = append(explanation, "in month "+month)
	}

	// Day of week
	if dow != "*" {
		days := map[string]string{
			"0": "Sunday", "1": "Monday", "2": "Tuesday",
			"3": "Wednesday", "4": "Thursday", "5": "Friday", "6": "Saturday",
		}
		if day, ok := days[dow]; ok {
			explanation = append(explanation, "on "+day+"s")
		} else {
			explanation = append(explanation, "on days "+dow)
		}
	}

	return strings.Join(explanation, " ")
}

// NextRuns returns the next N run times for a cron expression.
func NextRuns(expr string, count int, tz string) []time.Time {
	// Simplified implementation - in production use a proper cron library
	var runs []time.Time
	// This would require full cron parsing
	return runs
}
