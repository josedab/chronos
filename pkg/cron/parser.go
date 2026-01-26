// Package cron provides cron expression parsing and scheduling.
package cron

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Schedule represents a parsed cron schedule.
type Schedule interface {
	// Next returns the next activation time after the given time.
	Next(t time.Time) time.Time
}

// Parser parses cron expressions.
type Parser struct {
	options ParseOption
}

// ParseOption represents parsing options.
type ParseOption int

const (
	Second      ParseOption = 1 << iota // Seconds field (6 fields)
	Minute                              // Minute field
	Hour                                // Hour field
	Dom                                 // Day of month field
	Month                               // Month field
	Dow                                 // Day of week field
	Descriptor                          // Support @hourly, @daily, etc.
)

// Standard is a parser for the standard 5-field cron format.
var Standard = NewParser(Minute | Hour | Dom | Month | Dow | Descriptor)

// Extended is a parser for the extended 6-field cron format (with seconds).
var Extended = NewParser(Second | Minute | Hour | Dom | Month | Dow | Descriptor)

// NewParser creates a new Parser with the given options.
func NewParser(options ParseOption) *Parser {
	return &Parser{options: options}
}

// Parse parses a cron expression.
func Parse(spec string) (Schedule, error) {
	return Standard.Parse(spec)
}

// ParseStandard parses a standard 5-field cron expression.
func ParseStandard(spec string) (Schedule, error) {
	return Standard.Parse(spec)
}

// ParseExtended parses an extended 6-field cron expression (with seconds).
func ParseExtended(spec string) (Schedule, error) {
	return Extended.Parse(spec)
}

// Parse parses a cron expression using this parser's options.
func (p *Parser) Parse(spec string) (Schedule, error) {
	spec = strings.TrimSpace(spec)

	// Handle descriptors
	if strings.HasPrefix(spec, "@") {
		return p.parseDescriptor(spec)
	}

	// Split fields
	fields := strings.Fields(spec)

	// Determine expected field count
	expectedFields := 5
	if p.options&Second != 0 {
		expectedFields = 6
	}

	if len(fields) != expectedFields {
		return nil, fmt.Errorf("expected %d fields, got %d", expectedFields, len(fields))
	}

	var sched CronSchedule
	var err error
	fieldIdx := 0

	// Parse seconds if enabled
	if p.options&Second != 0 {
		sched.Second, err = parseField(fields[fieldIdx], 0, 59)
		if err != nil {
			return nil, fmt.Errorf("invalid second field: %w", err)
		}
		fieldIdx++
	} else {
		sched.Second = 1 << 0 // Default to second 0
	}

	// Parse minute
	sched.Minute, err = parseField(fields[fieldIdx], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("invalid minute field: %w", err)
	}
	fieldIdx++

	// Parse hour
	sched.Hour, err = parseField(fields[fieldIdx], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("invalid hour field: %w", err)
	}
	fieldIdx++

	// Parse day of month
	sched.Dom, err = parseField(fields[fieldIdx], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("invalid day-of-month field: %w", err)
	}
	fieldIdx++

	// Parse month
	sched.Month, err = parseField(fields[fieldIdx], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("invalid month field: %w", err)
	}
	fieldIdx++

	// Parse day of week
	sched.Dow, err = parseField(fields[fieldIdx], 0, 6)
	if err != nil {
		return nil, fmt.Errorf("invalid day-of-week field: %w", err)
	}

	return &sched, nil
}

// parseDescriptor parses a cron descriptor like @hourly, @daily, etc.
func (p *Parser) parseDescriptor(spec string) (Schedule, error) {
	switch strings.ToLower(spec) {
	case "@yearly", "@annually":
		return &CronSchedule{
			Second: 1 << 0,
			Minute: 1 << 0,
			Hour:   1 << 0,
			Dom:    1 << 1,
			Month:  1 << 1,
			Dow:    allBits(0, 6),
		}, nil
	case "@monthly":
		return &CronSchedule{
			Second: 1 << 0,
			Minute: 1 << 0,
			Hour:   1 << 0,
			Dom:    1 << 1,
			Month:  allBits(1, 12),
			Dow:    allBits(0, 6),
		}, nil
	case "@weekly":
		return &CronSchedule{
			Second: 1 << 0,
			Minute: 1 << 0,
			Hour:   1 << 0,
			Dom:    allBits(1, 31),
			Month:  allBits(1, 12),
			Dow:    1 << 0,
		}, nil
	case "@daily", "@midnight":
		return &CronSchedule{
			Second: 1 << 0,
			Minute: 1 << 0,
			Hour:   1 << 0,
			Dom:    allBits(1, 31),
			Month:  allBits(1, 12),
			Dow:    allBits(0, 6),
		}, nil
	case "@hourly":
		return &CronSchedule{
			Second: 1 << 0,
			Minute: 1 << 0,
			Hour:   allBits(0, 23),
			Dom:    allBits(1, 31),
			Month:  allBits(1, 12),
			Dow:    allBits(0, 6),
		}, nil
	}

	// Handle @every syntax
	if strings.HasPrefix(strings.ToLower(spec), "@every ") {
		return parseEvery(spec[7:])
	}

	return nil, fmt.Errorf("unrecognized descriptor: %s", spec)
}

// parseEvery parses an @every duration expression.
func parseEvery(spec string) (Schedule, error) {
	dur, err := time.ParseDuration(spec)
	if err != nil {
		return nil, fmt.Errorf("invalid duration: %w", err)
	}
	if dur < time.Second {
		return nil, fmt.Errorf("duration must be at least 1 second")
	}
	return &EverySchedule{Interval: dur}, nil
}

// parseField parses a single cron field.
func parseField(field string, min, max int) (uint64, error) {
	var bits uint64

	// Handle step values
	rangeAndStep := strings.Split(field, "/")
	if len(rangeAndStep) > 2 {
		return 0, fmt.Errorf("too many slashes")
	}

	rangeSpec := rangeAndStep[0]
	step := 1

	if len(rangeAndStep) == 2 {
		var err error
		step, err = strconv.Atoi(rangeAndStep[1])
		if err != nil || step <= 0 {
			return 0, fmt.Errorf("invalid step: %s", rangeAndStep[1])
		}
	}

	// Handle comma-separated values
	parts := strings.Split(rangeSpec, ",")
	for _, part := range parts {
		partBits, err := parseRange(part, min, max, step)
		if err != nil {
			return 0, err
		}
		bits |= partBits
	}

	return bits, nil
}

// parseRange parses a range specification like "1-5" or "*" or "10".
func parseRange(spec string, min, max, step int) (uint64, error) {
	var bits uint64

	// Handle wildcard
	if spec == "*" {
		for i := min; i <= max; i += step {
			bits |= 1 << uint(i)
		}
		return bits, nil
	}

	// Handle range
	if strings.Contains(spec, "-") {
		parts := strings.Split(spec, "-")
		if len(parts) != 2 {
			return 0, fmt.Errorf("invalid range: %s", spec)
		}
		start, err := parseValue(parts[0], min, max)
		if err != nil {
			return 0, err
		}
		end, err := parseValue(parts[1], min, max)
		if err != nil {
			return 0, err
		}
		if start > end {
			return 0, fmt.Errorf("invalid range: start > end")
		}
		for i := start; i <= end; i += step {
			bits |= 1 << uint(i)
		}
		return bits, nil
	}

	// Handle single value
	val, err := parseValue(spec, min, max)
	if err != nil {
		return 0, err
	}
	bits |= 1 << uint(val)
	return bits, nil
}

// parseValue parses a single value, handling named values like "MON", "JAN".
func parseValue(spec string, min, max int) (int, error) {
	// Try numeric
	val, err := strconv.Atoi(spec)
	if err == nil {
		if val < min || val > max {
			return 0, fmt.Errorf("value %d out of range [%d, %d]", val, min, max)
		}
		return val, nil
	}

	// Try named values
	spec = strings.ToUpper(spec)

	// Months
	months := map[string]int{
		"JAN": 1, "FEB": 2, "MAR": 3, "APR": 4, "MAY": 5, "JUN": 6,
		"JUL": 7, "AUG": 8, "SEP": 9, "OCT": 10, "NOV": 11, "DEC": 12,
	}
	if v, ok := months[spec]; ok {
		return v, nil
	}

	// Days of week
	days := map[string]int{
		"SUN": 0, "MON": 1, "TUE": 2, "WED": 3, "THU": 4, "FRI": 5, "SAT": 6,
	}
	if v, ok := days[spec]; ok {
		return v, nil
	}

	return 0, fmt.Errorf("invalid value: %s", spec)
}

// allBits returns a bitmask with all bits set in the range [min, max].
func allBits(min, max int) uint64 {
	var bits uint64
	for i := min; i <= max; i++ {
		bits |= 1 << uint(i)
	}
	return bits
}

// CronSchedule represents a parsed cron schedule using bit fields.
type CronSchedule struct {
	Second uint64
	Minute uint64
	Hour   uint64
	Dom    uint64
	Month  uint64
	Dow    uint64
}

// Next returns the next activation time after the given time.
func (s *CronSchedule) Next(t time.Time) time.Time {
	// Add 1 second to get past the current time
	t = t.Add(1*time.Second - time.Duration(t.Nanosecond())*time.Nanosecond)

	// This is a simplified implementation
	// Iterate up to 5 years to find the next matching time
	maxIterations := 5 * 366 * 24 * 60 * 60 // 5 years in seconds
	for i := 0; i < maxIterations; i++ {
		if s.matches(t) {
			return t
		}
		t = t.Add(1 * time.Second)
	}

	// This should never happen for valid schedules
	return time.Time{}
}

// matches returns true if the given time matches the schedule.
func (s *CronSchedule) matches(t time.Time) bool {
	return s.Second&(1<<uint(t.Second())) != 0 &&
		s.Minute&(1<<uint(t.Minute())) != 0 &&
		s.Hour&(1<<uint(t.Hour())) != 0 &&
		s.Dom&(1<<uint(t.Day())) != 0 &&
		s.Month&(1<<uint(t.Month())) != 0 &&
		s.Dow&(1<<uint(t.Weekday())) != 0
}

// EverySchedule represents an interval-based schedule.
type EverySchedule struct {
	Interval time.Duration
}

// Next returns the next activation time after the given time.
func (s *EverySchedule) Next(t time.Time) time.Time {
	return t.Add(s.Interval)
}

// Validate checks if a cron expression is valid.
func Validate(spec string) error {
	_, err := Parse(spec)
	return err
}

// ValidateExtended checks if an extended cron expression is valid.
func ValidateExtended(spec string) error {
	_, err := ParseExtended(spec)
	return err
}

// everyRegex matches @every duration expressions.
var everyRegex = regexp.MustCompile(`^@every\s+(\d+[smhd])+$`)
