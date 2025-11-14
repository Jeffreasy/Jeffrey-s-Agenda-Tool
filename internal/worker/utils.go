package worker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/api/calendar/v3"
)

// Shared utilities for calendar processing

// ClientConfig holds client identification patterns
type ClientConfig struct {
	Code        string
	SummaryKeys []string
	LocationKey string
	ManualKeys  []string
	Name        string
}

// ProcessorConfig holds configuration for calendar processing
type ProcessorConfig struct {
	MaxEventsPerCalendar     int64
	EarlyServiceThreshold    int // Hour of day (0-23) that separates early/late services
	AppointmentOffsetHours   int // Hours before event to schedule appointment
	AppointmentDurationHours int // Duration of appointment in hours
	BatchSize                int // Number of events to process in a batch (0 = no batching)
}

// GetClientConfigs returns the configuration for all supported clients
func GetClientConfigs() map[string]ClientConfig {
	return map[string]ClientConfig{
		"Rieshi": {
			Code:        "20270",
			SummaryKeys: []string{"Appartementen"},
			LocationKey: "Appartementen",
			ManualKeys:  []string{"Vroeg R", "Laat R"},
			Name:        "Rieshi",
		},
		"Abdul": {
			Code:        "21964",
			SummaryKeys: []string{"AA"},
			LocationKey: "AA",
			ManualKeys:  []string{"Vroeg A", "Laat A", "Abdul"},
			Name:        "Abdul",
		},
	}
}

// GetDefaultProcessorConfig returns default configuration for processors
func GetDefaultProcessorConfig() ProcessorConfig {
	return ProcessorConfig{
		MaxEventsPerCalendar:     100,
		EarlyServiceThreshold:    13, // 13:00 separates early/late services
		AppointmentOffsetHours:   1,  // 1 hour before event
		AppointmentDurationHours: 1,  // 1 hour duration
		BatchSize:                0,  // 0 = process all at once (no batching)
	}
}

// ExtractGroupKey determines the group key for an event (shared logic)
func ExtractGroupKey(event *calendar.Event) string {
	// Skip events that are created by our automation system
	if strings.Contains(event.Description, "Automatische afspraak") ||
		strings.Contains(event.Description, "Origineel event ID") ||
		strings.Contains(event.Description, "Geconsolideerde afspraak") {
		return ""
	}

	var startDate time.Time
	var err error

	if event.Start.DateTime != "" {
		startDate, err = time.Parse(time.RFC3339, event.Start.DateTime)
	} else if event.Start.Date != "" {
		startDate, err = time.Parse("2006-01-02", event.Start.Date)
	} else {
		return ""
	}

	if err != nil {
		return ""
	}

	clientSuffix := ""

	configs := GetClientConfigs()
	for clientName, config := range configs {
		if strings.Contains(event.Description, config.Code) ||
			containsAny(event.Summary, config.SummaryKeys) ||
			strings.Contains(event.Location, config.LocationKey) ||
			containsAny(event.Summary, config.ManualKeys) {

			clientSuffix = clientName
			break
		}
	}

	if clientSuffix == "" {
		return ""
	}

	return fmt.Sprintf("%s-%s", startDate.Format("2006-01-02"), clientSuffix)
}

// GetClientName extracts client name from group key
func GetClientName(groupKey string) string {
	if strings.HasSuffix(groupKey, "Rieshi") {
		return "Rieshi"
	} else if strings.HasSuffix(groupKey, "Abdul") {
		return "Abdul"
	}
	return ""
}

// IsEarlyService determines if an event represents early service using default config
func IsEarlyService(dateTime string) bool {
	config := GetDefaultProcessorConfig()
	return IsEarlyServiceWithConfig(dateTime, config)
}

// IsEarlyServiceWithConfig determines if an event represents early service with custom config
func IsEarlyServiceWithConfig(dateTime string, config ProcessorConfig) bool {
	t, _ := time.Parse(time.RFC3339, dateTime)
	return t.Hour() < config.EarlyServiceThreshold
}

// GetEventStartTime extracts start time from calendar event
func GetEventStartTime(event *calendar.Event) time.Time {
	if event.Start.DateTime != "" {
		t, _ := time.Parse(time.RFC3339, event.Start.DateTime)
		return t
	}
	if event.Start.Date != "" {
		t, _ := time.Parse("2006-01-02", event.Start.Date)
		return t
	}
	return time.Now()
}

// containsAny checks if the target string contains any of the substrings
func containsAny(target string, substrings []string) bool {
	for _, substr := range substrings {
		if strings.Contains(target, substr) {
			return true
		}
	}
	return false
}

// RetryConfig holds configuration for retry logic
type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

// GetDefaultRetryConfig returns default retry configuration
func GetDefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		BaseDelay:  1 * time.Second,
		MaxDelay:   30 * time.Second,
	}
}

// RetryOperation retries an operation with exponential backoff
func RetryOperation(ctx context.Context, config RetryConfig, operation func() error) error {
	var lastErr error
	delay := config.BaseDelay

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err

		if attempt < config.MaxRetries {
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}

			delay *= 2 // Exponential backoff
		}
	}

	return fmt.Errorf("operation failed after %d retries: %w", config.MaxRetries, lastErr)
}

// FetchCalendarEvents fetches events from all accessible calendars
func FetchCalendarEvents(ctx context.Context, srv *calendar.Service, timeMin string, maxResults int64) ([]*calendar.Event, error) {
	config := GetDefaultProcessorConfig()
	return FetchCalendarEventsWithConfig(ctx, srv, timeMin, config)
}

// FetchCalendarEventsWithConfig fetches events from all accessible calendars with custom config
func FetchCalendarEventsWithConfig(ctx context.Context, srv *calendar.Service, timeMin string, config ProcessorConfig) ([]*calendar.Event, error) {
	calendarList, err := srv.CalendarList.List().Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve calendar list: %w", err)
	}

	var allEvents []*calendar.Event

	for _, calEntry := range calendarList.Items {
		if calEntry.AccessRole == "freeBusyReader" || calEntry.AccessRole == "none" {
			continue
		}

		events, err := srv.Events.List(calEntry.Id).
			Context(ctx).
			ShowDeleted(false).
			SingleEvents(true).
			TimeMin(timeMin).
			MaxResults(config.MaxEventsPerCalendar).
			OrderBy("startTime").Do()

		if err != nil {
			continue // Log error but continue with other calendars
		}

		allEvents = append(allEvents, events.Items...)
	}

	return allEvents, nil
}

// CreateAppointmentEvent creates a new appointment event with configurable parameters
func CreateAppointmentEvent(clientName string, isEarly bool, originalEvent *calendar.Event, config ProcessorConfig) *calendar.Event {
	var title string
	if isEarly {
		title = fmt.Sprintf("Vroege dienst %s", clientName)
	} else {
		title = fmt.Sprintf("Late dienst %s", clientName)
	}

	originalStart, _ := time.Parse(time.RFC3339, originalEvent.Start.DateTime)
	newStart := originalStart.Add(-time.Duration(config.AppointmentOffsetHours) * time.Hour)
	newEnd := newStart.Add(time.Duration(config.AppointmentDurationHours) * time.Hour)

	return &calendar.Event{
		Summary:     title,
		Description: fmt.Sprintf("Automatische afspraak voor %s dienst: %s\nOrigineel event ID: %s", clientName, originalEvent.Summary, originalEvent.Id),
		Start: &calendar.EventDateTime{
			DateTime: newStart.Format(time.RFC3339),
			TimeZone: originalEvent.Start.TimeZone,
		},
		End: &calendar.EventDateTime{
			DateTime: newEnd.Format(time.RFC3339),
			TimeZone: originalEvent.End.TimeZone,
		},
		Location: originalEvent.Location,
	}
}

// ProcessEventsInBatches processes events in configurable batches to optimize memory usage
func ProcessEventsInBatches(events []*calendar.Event, batchSize int, processor func([]*calendar.Event) error) error {
	if batchSize <= 0 || len(events) <= batchSize {
		// Process all at once if no batching or small dataset
		return processor(events)
	}

	for i := 0; i < len(events); i += batchSize {
		end := i + batchSize
		if end > len(events) {
			end = len(events)
		}

		batch := events[i:end]
		if err := processor(batch); err != nil {
			return err
		}
	}

	return nil
}
