package worker

import (
	"agenda-automator-api/internal/domain"
	"agenda-automator-api/internal/store"
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"google.golang.org/api/calendar/v3"
)

// ConsolidationProcessor handles consolidation of calendar appointments to ensure single appointments and remove duplicates.
type ConsolidationProcessor struct {
	store store.Storer
}

// NewConsolidationProcessor creates a new consolidation processor.
func NewConsolidationProcessor(s store.Storer) *ConsolidationProcessor {
	return &ConsolidationProcessor{
		store: s,
	}
}

// Process fetches calendar events and ensures only one appointment per day per client, removing duplicates.
// Implementeert de Processor interface.
func (p *ConsolidationProcessor) Process(ctx context.Context, srv *calendar.Service, acc domain.ConnectedAccount) error {
	// Fetch all calendar events
	events, err := p.fetchCalendarEvents(ctx, srv)
	if err != nil {
		return fmt.Errorf("unable to fetch calendar events: %w", err)
	}

	log.Printf("[ConsolidationProcessor] Processing consolidation for %d events", len(events))

	// Group events by date and client
	groupedEvents := p.groupEventsByClient(events)

	// Process each group
	for groupKey, groupEvents := range groupedEvents {
		if err := p.consolidateGroup(ctx, srv, groupKey, groupEvents); err != nil {
			log.Printf("[ConsolidationProcessor] Error consolidating group %s: %v", groupKey, err)
		}
	}

	// Also clean up any orphaned duplicate appointments
	if err := p.cleanupDuplicateAppointments(ctx, srv); err != nil {
		log.Printf("[ConsolidationProcessor] Error cleaning up duplicate appointments: %v", err)
	}

	return nil
}

// fetchCalendarEvents fetches upcoming events from all calendars.
func (p *ConsolidationProcessor) fetchCalendarEvents(ctx context.Context, srv *calendar.Service) ([]*calendar.Event, error) {
	t := time.Now().Format(time.RFC3339)
	return FetchCalendarEvents(ctx, srv, t, 100)
}

// groupEventsByClient groups events by date and client (Rieshi/Abdul).
func (p *ConsolidationProcessor) groupEventsByClient(events []*calendar.Event) map[string][]*calendar.Event {
	groupedEvents := make(map[string][]*calendar.Event)

	for _, event := range events {
		groupKey := ExtractGroupKey(event)
		if groupKey != "" {
			groupedEvents[groupKey] = append(groupedEvents[groupKey], event)
		}
	}

	return groupedEvents
}

// consolidateGroup ensures only one appointment exists for a group and removes duplicates.
func (p *ConsolidationProcessor) consolidateGroup(ctx context.Context, srv *calendar.Service, groupKey string, events []*calendar.Event) error {
	if len(events) == 0 {
		return nil
	}

	clientName := GetClientName(groupKey)
	if clientName == "" {
		return nil
	}

	log.Printf("[ConsolidationProcessor] Consolidating %d events for %s on %s", len(events), clientName, groupKey)

	// Sort events by start time
	sort.Slice(events, func(i, j int) bool {
		startI := GetEventStartTime(events[i])
		startJ := GetEventStartTime(events[j])
		return startI.Before(startJ)
	})

	// Use the earliest event for the appointment
	primaryEvent := events[0]

	// Determine if early or late service
	isEarly := IsEarlyService(primaryEvent.Start.DateTime)

	var title string
	if isEarly {
		title = fmt.Sprintf("Vroege dienst %s", clientName)
	} else {
		title = fmt.Sprintf("Late dienst %s", clientName)
	}

	// Calculate appointment time using config
	config := GetDefaultProcessorConfig()
	originalStart, _ := time.Parse(time.RFC3339, primaryEvent.Start.DateTime)
	newStart := originalStart.Add(-time.Duration(config.AppointmentOffsetHours) * time.Hour)
	newEnd := newStart.Add(time.Duration(config.AppointmentDurationHours) * time.Hour)

	// Find existing appointments for this time slot
	existingAppointments, err := p.findAppointmentsInTimeRange(ctx, srv, newStart, newEnd, title)
	if err != nil {
		return fmt.Errorf("error finding existing appointments: %w", err)
	}

	if len(existingAppointments) == 0 {
		// No appointment exists, create one
		newEvent := CreateAppointmentEvent(clientName, isEarly, primaryEvent, config)
		// Override description for consolidated appointments
		newEvent.Description = fmt.Sprintf("Geconsolideerde afspraak voor %s dienst: %s\nOrigineel event: %s", clientName, primaryEvent.Summary, primaryEvent.Id)

		createdEvent, err := srv.Events.Insert("primary", newEvent).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("failed to create consolidated appointment: %w", err)
		}

		log.Printf("[ConsolidationProcessor] Created consolidated appointment for %s: %s (ID: %s)", clientName, title, createdEvent.Id)

	} else if len(existingAppointments) > 1 {
		// Multiple appointments exist, remove duplicates keeping the first one
		for i := 1; i < len(existingAppointments); i++ {
			err := srv.Events.Delete("primary", existingAppointments[i].Id).Context(ctx).Do()
			if err != nil {
				log.Printf("[ConsolidationProcessor] ERROR: Failed to delete duplicate appointment %s: %v", existingAppointments[i].Id, err)
			} else {
				log.Printf("[ConsolidationProcessor] Deleted duplicate appointment: %s", existingAppointments[i].Id)
			}
		}
	} else {
		// Exactly one appointment exists, nothing to do
		log.Printf("[ConsolidationProcessor] Appointment already exists for %s: %s", clientName, title)
	}

	return nil
}

// findAppointmentsInTimeRange finds appointments in a specific time range with a specific title.
func (p *ConsolidationProcessor) findAppointmentsInTimeRange(ctx context.Context, srv *calendar.Service, start, end time.Time, title string) ([]*calendar.Event, error) {
	events, err := srv.Events.List("primary").
		Context(ctx).
		TimeMin(start.Format(time.RFC3339)).
		TimeMax(end.Format(time.RFC3339)).
		Q(title).
		Do()
	if err != nil {
		return nil, err
	}
	return events.Items, nil
}

// cleanupDuplicateAppointments finds and removes duplicate appointments that don't match our consolidation logic.
func (p *ConsolidationProcessor) cleanupDuplicateAppointments(ctx context.Context, srv *calendar.Service) error {
	log.Printf("[ConsolidationProcessor] Starting cleanupDuplicateAppointments with new logic")
	// Get all events from the past week to now + 1 month
	now := time.Now()
	timeMin := now.Add(-7 * 24 * time.Hour).Format(time.RFC3339)
	timeMax := now.Add(30 * 24 * time.Hour).Format(time.RFC3339)

	events, err := srv.Events.List("primary").
		Context(ctx).
		TimeMin(timeMin).
		TimeMax(timeMax).
		ShowDeleted(false). // Zorg dat we geen verwijderde items ophalen
		SingleEvents(true).
		Do()
	if err != nil {
		return fmt.Errorf("error fetching events for cleanup: %w", err)
	}

	// Group appointments by title and time slot
	appointmentGroups := make(map[string][]*calendar.Event)

	for _, event := range events.Items {
		if !strings.Contains(event.Summary, "dienst") {
			continue // Only process our appointment events
		}

		if event.Start == nil || event.Start.DateTime == "" {
			continue // Geen starttijd, negeren
		}

		// Create a key based on title and start time (rounded to hour)
		startTime, err := time.Parse(time.RFC3339, event.Start.DateTime)
		if err != nil {
			continue
		}

		// --- NIEUWE OPRAAKLOGICA START ---
		// Verwijder "Vroege dienst" afspraken die te vroeg zijn (vóór 06:00)
		if strings.Contains(event.Summary, "Vroege") && startTime.Hour() < 6 {
			log.Printf("[ConsolidationProcessor] Removing 'Vroege dienst' at invalid time: %s (%s)", event.Summary, event.Start.DateTime)
			err := srv.Events.Delete("primary", event.Id).Context(ctx).Do()
			if err != nil {
				log.Printf("[ConsolidationProcessor] ERROR: Could not delete invalid 'Vroege dienst' %s: %v", event.Id, err)
			}
			continue // Ga naar het volgende event, dit is verwijderd
		}

		// Verwijder "Late dienst" afspraken die te vroeg zijn (vóór 13:00)
		// De geplande afspraak hoort om 13:30/13:45 te zijn, dus alles voor 13:00 is fout.
		if strings.Contains(event.Summary, "Late") && startTime.Hour() < 13 {
			log.Printf("[ConsolidationProcessor] Removing 'Late dienst' at invalid time: %s (%s)", event.Summary, event.Start.DateTime)
			err := srv.Events.Delete("primary", event.Id).Context(ctx).Do()
			if err != nil {
				log.Printf("[ConsolidationProcessor] ERROR: Could not delete invalid 'Late dienst' %s: %v", event.Id, err)
			}
			continue // Ga naar het volgende event, dit is verwijderd
		}
		// --- NIEUWE OPRAAKLOGICA EIND ---

		key := fmt.Sprintf("%s_%s", event.Summary, startTime.Format("2006-01-02_15"))

		appointmentGroups[key] = append(appointmentGroups[key], event)
	}

	// Remove duplicates in each group (dit ruimt nu alleen nog duplicaten op de *juiste* tijden op)
	for key, appointments := range appointmentGroups {
		if len(appointments) > 1 {
			log.Printf("[ConsolidationProcessor] Found %d duplicate appointments for key %s", len(appointments), key)
			// Keep the first one, delete the rest
			for i := 1; i < len(appointments); i++ {
				err := srv.Events.Delete("primary", appointments[i].Id).Context(ctx).Do()
				if err != nil {
					log.Printf("[ConsolidationProcessor] ERROR: Failed to delete duplicate appointment %s: %v", appointments[i].Id, err)
				} else {
					log.Printf("[ConsolidationProcessor] Deleted duplicate appointment: %s", appointments[i].Id)
				}
			}
		}
	}

	return nil
}
