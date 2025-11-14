package worker

import (
	"agenda-automator-api/internal/domain"
	"agenda-automator-api/internal/store"
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"google.golang.org/api/calendar/v3"
)

// CalendarProcessor beheert de specifieke logica voor het verwerken van agenda-events.
type CalendarProcessor struct {
	store store.Storer
	// Je kunt hier meer dependencies toevoegen als dat nodig is,
	// zoals een specifieke logger of configuratie.
}

// NewCalendarProcessor maakt een nieuwe processor.
func NewCalendarProcessor(s store.Storer) *CalendarProcessor {
	return &CalendarProcessor{
		store: s,
	}
}

// Process is de kernlogica voor het verwerken van calendar events.
// Implementeert de Processor interface.
func (p *CalendarProcessor) Process(ctx context.Context, srv *calendar.Service, acc domain.ConnectedAccount) error {

	// Fetch all events from accessible calendars
	t := time.Now().Format(time.RFC3339)
	allEvents, err := FetchCalendarEvents(ctx, srv, t, 100)
	if err != nil {
		return fmt.Errorf("unable to fetch calendar events: %w", err)
	}

	log.Printf("[CalendarProcessor] Found %d total events across all calendars.", len(allEvents))

	// --- REST VAN DE FUNCTIE BLIJFT HETZELFDE ---
	// Maar gebruikt nu 'allEvents' in plaats van 'events.Items'

	// Groepeer per datum + client (Rieshi of Abdul)
	groupedEvents := make(map[string][]*calendar.Event)

	log.Printf("[CalendarProcessor] --- STARTING EVENT ANALYSIS (Checking %d events) ---", len(allEvents))

	for i, item := range allEvents { // Gebruik 'i' voor een teller

		// --- NIEUWE DEBUG LOG ---
		// Log details van het event zodat we kunnen zien waarom het niet matcht
		// We vervangen newlines met een spatie voor een schone log
		cleanDescription := strings.ReplaceAll(item.Description, "\n", " ")
		log.Printf("[CalendarProcessor] Event %d/%d: Summary=[%s], DateTime=[%s], Date=[%s], Location=[%s], Description=[%s]",
			i+1,
			len(allEvents),
			item.Summary,
			item.Start.DateTime,
			item.Start.Date,
			item.Location,
			cleanDescription, // Gebruik de schone omschrijving
		)
		// --- EINDE DEBUG LOG ---

		groupKey := ExtractGroupKey(item) // Use shared utility function
		if groupKey != "" {
			// Log als we een match vinden!
			log.Printf("[CalendarProcessor] ^^^ MATCH FOUND! Key: %s", groupKey)
			groupedEvents[groupKey] = append(groupedEvents[groupKey], item)
		}
	}

	log.Printf("[CalendarProcessor] --- FINISHED EVENT ANALYSIS (Found %d groups) ---", len(groupedEvents))

	// Analyseer groepen en creëer afspraken per client
	for groupKey, groupEvents := range groupedEvents {
		log.Printf("[CalendarProcessor] Analyzing %d events in group %s", len(groupEvents), groupKey) // <--- DEZE ZOU JE NU MOETEN ZIEN

		for _, evt := range groupEvents {
			// ... (de rest van je logica voor 'Rieshi' en 'Abdul' blijft 100% hetzelfde) ...
			clientName := GetClientName(groupKey)
			if clientName == "" {
				continue
			}

			isEarly := IsEarlyService(evt.Start.DateTime)

			var title string
			if isEarly {
				title = fmt.Sprintf("Vroege dienst %s", clientName)
			} else {
				title = fmt.Sprintf("Late dienst %s", clientName)
			}

			config := GetDefaultProcessorConfig()
			originalStart, _ := time.Parse(time.RFC3339, evt.Start.DateTime)
			newStart := originalStart.Add(-time.Duration(config.AppointmentOffsetHours) * time.Hour)
			newEnd := newStart.Add(time.Duration(config.AppointmentDurationHours) * time.Hour)

			if !p.appointmentExists(srv, newStart, newEnd, title) {
				newEvent := CreateAppointmentEvent(clientName, isEarly, evt, config)

				createdEvent, err := srv.Events.Insert("primary", newEvent).Do() // <--- Nieuwe events in 'primary' zetten is prima
				if err != nil {
					log.Printf("[CalendarProcessor] ERROR: Failed to create appointment for %s event %s: %v", clientName, evt.Id, err)
					continue
				}
				log.Printf("[CalendarProcessor] Created new appointment for %s: %s (ID: %s)", clientName, title, createdEvent.Id)

			} else {
				log.Printf("[CalendarProcessor] Appointment already exists for %s event %s: %s", clientName, evt.Id, title)
			}
		}
	}

	return nil
}

// Helper functions are now in utils.go

// appointmentExists: Check duplicaat
func (p *CalendarProcessor) appointmentExists(srv *calendar.Service, start, end time.Time, title string) bool {
	events, _ := srv.Events.List("primary").
		TimeMin(start.Format(time.RFC3339)).
		TimeMax(end.Format(time.RFC3339)).
		Q(title).
		Do()
	return len(events.Items) > 0
}
