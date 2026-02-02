// Package sdk provides helper functions and common patterns.
package sdk

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Every creates common schedule expressions.
type Every struct{}

// EverySchedule provides common schedule helpers.
var EverySchedule = Every{}

// Second returns a cron expression for every N seconds.
func (Every) Second(n int) string {
	return fmt.Sprintf("*/%d * * * * *", n)
}

// Minute returns a cron expression for every N minutes.
func (Every) Minute(n int) string {
	return fmt.Sprintf("0 */%d * * * *", n)
}

// Hour returns a cron expression for every N hours.
func (Every) Hour(n int) string {
	return fmt.Sprintf("0 0 */%d * * *", n)
}

// Day returns a cron expression for every day at the specified time.
func (Every) Day(hour, minute int) string {
	return fmt.Sprintf("0 %d %d * * *", minute, hour)
}

// DayAt is an alias for Day.
func (Every) DayAt(hour, minute int) string {
	return fmt.Sprintf("0 %d %d * * *", minute, hour)
}

// Monday returns a cron expression for every Monday at the specified time.
func (Every) Monday(hour, minute int) string {
	return fmt.Sprintf("0 %d %d * * 1", minute, hour)
}

// Tuesday returns a cron expression for every Tuesday at the specified time.
func (Every) Tuesday(hour, minute int) string {
	return fmt.Sprintf("0 %d %d * * 2", minute, hour)
}

// Wednesday returns a cron expression for every Wednesday at the specified time.
func (Every) Wednesday(hour, minute int) string {
	return fmt.Sprintf("0 %d %d * * 3", minute, hour)
}

// Thursday returns a cron expression for every Thursday at the specified time.
func (Every) Thursday(hour, minute int) string {
	return fmt.Sprintf("0 %d %d * * 4", minute, hour)
}

// Friday returns a cron expression for every Friday at the specified time.
func (Every) Friday(hour, minute int) string {
	return fmt.Sprintf("0 %d %d * * 5", minute, hour)
}

// Saturday returns a cron expression for every Saturday at the specified time.
func (Every) Saturday(hour, minute int) string {
	return fmt.Sprintf("0 %d %d * * 6", minute, hour)
}

// Sunday returns a cron expression for every Sunday at the specified time.
func (Every) Sunday(hour, minute int) string {
	return fmt.Sprintf("0 %d %d * * 0", minute, hour)
}

// Weekday returns a cron expression for Monday-Friday at the specified time.
func (Every) Weekday(hour, minute int) string {
	return fmt.Sprintf("0 %d %d * * 1-5", minute, hour)
}

// Weekend returns a cron expression for Saturday-Sunday at the specified time.
func (Every) Weekend(hour, minute int) string {
	return fmt.Sprintf("0 %d %d * * 0,6", minute, hour)
}

// FirstOfMonth returns a cron expression for the 1st of every month.
func (Every) FirstOfMonth(hour, minute int) string {
	return fmt.Sprintf("0 %d %d 1 * *", minute, hour)
}

// LastDayOfMonth returns a cron expression for the last day of every month.
func (Every) LastDayOfMonth(hour, minute int) string {
	return fmt.Sprintf("0 %d %d L * *", minute, hour)
}

// Midnight returns a cron expression for midnight every day.
func (Every) Midnight() string {
	return "0 0 0 * * *"
}

// Noon returns a cron expression for noon every day.
func (Every) Noon() string {
	return "0 0 12 * * *"
}

// CronBuilder helps build cron expressions.
type CronBuilder struct {
	second     string
	minute     string
	hour       string
	dayOfMonth string
	month      string
	dayOfWeek  string
}

// NewCronBuilder creates a new cron builder.
func NewCronBuilder() *CronBuilder {
	return &CronBuilder{
		second:     "*",
		minute:     "*",
		hour:       "*",
		dayOfMonth: "*",
		month:      "*",
		dayOfWeek:  "*",
	}
}

// Second sets the second field.
func (b *CronBuilder) Second(s string) *CronBuilder {
	b.second = s
	return b
}

// Minute sets the minute field.
func (b *CronBuilder) Minute(s string) *CronBuilder {
	b.minute = s
	return b
}

// Hour sets the hour field.
func (b *CronBuilder) Hour(s string) *CronBuilder {
	b.hour = s
	return b
}

// DayOfMonth sets the day of month field.
func (b *CronBuilder) DayOfMonth(s string) *CronBuilder {
	b.dayOfMonth = s
	return b
}

// Month sets the month field.
func (b *CronBuilder) Month(s string) *CronBuilder {
	b.month = s
	return b
}

// DayOfWeek sets the day of week field.
func (b *CronBuilder) DayOfWeek(s string) *CronBuilder {
	b.dayOfWeek = s
	return b
}

// AtSecond sets execution at a specific second.
func (b *CronBuilder) AtSecond(sec int) *CronBuilder {
	b.second = fmt.Sprintf("%d", sec)
	return b
}

// AtMinute sets execution at a specific minute.
func (b *CronBuilder) AtMinute(min int) *CronBuilder {
	b.minute = fmt.Sprintf("%d", min)
	return b
}

// AtHour sets execution at a specific hour.
func (b *CronBuilder) AtHour(hour int) *CronBuilder {
	b.hour = fmt.Sprintf("%d", hour)
	return b
}

// OnDay sets execution on a specific day of month.
func (b *CronBuilder) OnDay(day int) *CronBuilder {
	b.dayOfMonth = fmt.Sprintf("%d", day)
	return b
}

// InMonth sets execution in a specific month.
func (b *CronBuilder) InMonth(month int) *CronBuilder {
	b.month = fmt.Sprintf("%d", month)
	return b
}

// OnWeekday sets execution on a specific weekday (0=Sunday, 6=Saturday).
func (b *CronBuilder) OnWeekday(day int) *CronBuilder {
	b.dayOfWeek = fmt.Sprintf("%d", day)
	return b
}

// EveryNSeconds sets execution every N seconds.
func (b *CronBuilder) EveryNSeconds(n int) *CronBuilder {
	b.second = fmt.Sprintf("*/%d", n)
	return b
}

// EveryNMinutes sets execution every N minutes.
func (b *CronBuilder) EveryNMinutes(n int) *CronBuilder {
	b.minute = fmt.Sprintf("*/%d", n)
	return b
}

// EveryNHours sets execution every N hours.
func (b *CronBuilder) EveryNHours(n int) *CronBuilder {
	b.hour = fmt.Sprintf("*/%d", n)
	return b
}

// Build returns the cron expression.
func (b *CronBuilder) Build() string {
	return strings.Join([]string{
		b.second,
		b.minute,
		b.hour,
		b.dayOfMonth,
		b.month,
		b.dayOfWeek,
	}, " ")
}

// JobBuilder helps build job configurations.
type JobBuilder struct {
	scheduler *Scheduler
	name      string
	schedule  string
	handler   JobHandler
	options   []JobOption
}

// NewJobBuilder creates a new job builder.
func (s *Scheduler) NewJob(name string) *JobBuilder {
	return &JobBuilder{
		scheduler: s,
		name:      name,
		options:   make([]JobOption, 0),
	}
}

// WithSchedule sets the schedule.
func (b *JobBuilder) WithSchedule(schedule string) *JobBuilder {
	b.schedule = schedule
	return b
}

// Every sets a common schedule pattern.
func (b *JobBuilder) Every(duration string) *JobBuilder {
	switch strings.ToLower(duration) {
	case "second":
		b.schedule = "* * * * * *"
	case "minute":
		b.schedule = "0 * * * * *"
	case "hour":
		b.schedule = "0 0 * * * *"
	case "day":
		b.schedule = "0 0 0 * * *"
	case "week":
		b.schedule = "0 0 0 * * 0"
	case "month":
		b.schedule = "0 0 0 1 * *"
	default:
		b.schedule = duration
	}
	return b
}

// At sets the execution time.
func (b *JobBuilder) At(hour, minute int) *JobBuilder {
	if b.schedule == "" {
		b.schedule = fmt.Sprintf("0 %d %d * * *", minute, hour)
	}
	return b
}

// Handler sets the job handler.
func (b *JobBuilder) Handler(h JobHandler) *JobBuilder {
	b.handler = h
	return b
}

// Do sets a simple function as the handler.
func (b *JobBuilder) Do(fn func()) *JobBuilder {
	b.handler = func(ctx context.Context, job *Job) error {
		fn()
		return nil
	}
	return b
}

// DoWithContext sets a function with context as the handler.
func (b *JobBuilder) DoWithContext(fn func(context.Context) error) *JobBuilder {
	b.handler = func(ctx context.Context, job *Job) error {
		return fn(ctx)
	}
	return b
}

// Timeout sets the job timeout.
func (b *JobBuilder) Timeout(d time.Duration) *JobBuilder {
	b.options = append(b.options, WithJobTimeout(d))
	return b
}

// Retries sets retry configuration.
func (b *JobBuilder) Retries(max int, delay time.Duration) *JobBuilder {
	b.options = append(b.options, WithJobRetries(max, delay))
	return b
}

// Tags sets job tags.
func (b *JobBuilder) Tags(tags map[string]string) *JobBuilder {
	b.options = append(b.options, WithJobTags(tags))
	return b
}

// Tag adds a single tag.
func (b *JobBuilder) Tag(key, value string) *JobBuilder {
	b.options = append(b.options, func(j *Job) {
		if j.Tags == nil {
			j.Tags = make(map[string]string)
		}
		j.Tags[key] = value
	})
	return b
}

// Disabled creates the job in disabled state.
func (b *JobBuilder) Disabled() *JobBuilder {
	b.options = append(b.options, WithJobDisabled())
	return b
}

// Build schedules the job and returns it.
func (b *JobBuilder) Build() (*Job, error) {
	if b.schedule == "" {
		return nil, fmt.Errorf("schedule is required")
	}
	if b.handler == nil {
		return nil, fmt.Errorf("handler is required")
	}
	return b.scheduler.Schedule(b.name, b.schedule, b.handler, b.options...)
}

// MustBuild schedules the job and panics on error.
func (b *JobBuilder) MustBuild() *Job {
	job, err := b.Build()
	if err != nil {
		panic(err)
	}
	return job
}
