package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"agenda-automator-api/internal/domain"
	"agenda-automator-api/internal/store"
)

// CalendarProcessor handles calendar event processing
type CalendarProcessor struct {
	store store.Storer
}

// NewCalendarProcessor creates a new calendar processor
func NewCalendarProcessor(s store.Storer) *CalendarProcessor {
	return &CalendarProcessor{
		store: s,
	}
}

// ProcessEvents processes calendar events for automation rules.
func (cp *CalendarProcessor) ProcessEvents(
	ctx context.Context,
	acc *domain.ConnectedAccount,
	token *oauth2.Token,
) error {
	// Create calendar service
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))
	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("could not create calendar service: %w", err)
	}

	// Fetch all events
	tMin := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	tMax := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)

	log.Printf("[Calendar] Fetching all calendar events for %s (unlimited)", acc.Email)

	events, err := srv.Events.List("primary").
		TimeMin(tMin).
		TimeMax(tMax).
		SingleEvents(true).
		OrderBy("startTime").
		MaxResults(2500).
		Do()
	if err != nil {
		return fmt.Errorf("could not fetch calendar events: %w", err)
	}

	rules, err := cp.store.GetRulesForAccount(ctx, acc.ID)
	if err != nil {
		return fmt.Errorf("could not fetch automation rules: %w", err)
	}

	if len(events.Items) == 0 || len(rules) == 0 {
		log.Printf("[Calendar] No upcoming events or no rules found for %s. Skipping.", acc.Email)
		return nil
	}

	log.Printf("[Calendar] Checking %d events against %d rules for %s...", len(events.Items), len(rules), acc.Email)

	for _, event := range events.Items {
		// Skip own created events
		if strings.HasPrefix(event.Description, "Automatische reminder voor:") {
			continue
		}

		for _, rule := range rules {
			if !rule.IsActive {
				continue
			}

			var trigger domain.TriggerConditions
			if err = json.Unmarshal(rule.TriggerConditions, &trigger); err != nil {
				log.Printf("[Calendar] ERROR unmarshaling trigger for rule %s: %v", rule.ID, err)
				continue
			}

			// Check triggers
			summaryMatch := false
			if trigger.SummaryEquals != "" && event.Summary == trigger.SummaryEquals {
				summaryMatch = true
			}
			if !summaryMatch && len(trigger.SummaryContains) > 0 {
				for _, contain := range trigger.SummaryContains {
					if strings.Contains(event.Summary, contain) {
						summaryMatch = true
						break
					}
				}
			}
			if !summaryMatch {
				continue
			}

			locationMatch := false
			if len(trigger.LocationContains) == 0 {
				locationMatch = true
			} else {
				eventLocationLower := strings.ToLower(event.Location)
				for _, loc := range trigger.LocationContains {
					if strings.Contains(eventLocationLower, strings.ToLower(loc)) {
						locationMatch = true
						break
					}
				}
			}
			if !locationMatch {
				continue
			}

			// Check logs
			hasLogged, err := cp.store.HasLogForTrigger(ctx, rule.ID, event.Id)
			if err != nil {
				log.Printf("[Calendar] ERROR checking logs for event %s / rule %s: %v", event.Id, rule.ID, err)
			}
			if hasLogged {
				continue
			}

			log.Printf("[Calendar] MATCH: Event '%s' (ID: %s) matches rule '%s'.", event.Summary, event.Id, rule.Name)

			var action domain.ActionParams
			if err = json.Unmarshal(rule.ActionParams, &action); err != nil {
				log.Printf("[Calendar] ERROR unmarshaling action for rule %s: %v", rule.ID, err)
				continue
			}

			if action.NewEventTitle == "" {
				log.Printf("[Calendar] ERROR: Rule %s heeft geen 'new_event_title'.", rule.ID)
				continue
			}

			startTime, err := time.Parse(time.RFC3339, event.Start.DateTime)
			if err != nil {
				log.Printf("[Calendar] ERROR parsing start time: %v", err)
				continue
			}

			offset := action.OffsetMinutes
			if offset == 0 {
				offset = -60
			}
			reminderTime := startTime.Add(time.Duration(offset) * time.Minute)

			durMin := action.DurationMin
			if durMin == 0 {
				durMin = 5
			}
			endTime := reminderTime.Add(time.Duration(durMin) * time.Minute)

			title := action.NewEventTitle

			// Check for duplicates
			if cp.eventExists(srv, reminderTime, endTime, title) {
				log.Printf("[Calendar] SKIP: Reminder event '%s' at %s already exists.", title, reminderTime)

				triggerDetailsJSON, _ := json.Marshal(domain.TriggerLogDetails{
					GoogleEventID:  event.Id,
					TriggerSummary: event.Summary,
					TriggerTime:    startTime,
				})
				actionDetailsJSON, _ := json.Marshal(domain.ActionLogDetails{
					CreatedEventID:      "unknown-pre-existing",
					CreatedEventSummary: title,
					ReminderTime:        reminderTime,
				})
				logParams := store.CreateLogParams{
					ConnectedAccountID: acc.ID,
					RuleID:             rule.ID,
					Status:             domain.LogSkipped,
					TriggerDetails:     triggerDetailsJSON,
					ActionDetails:      actionDetailsJSON,
				}
				if err = cp.store.CreateAutomationLog(ctx, logParams); err != nil {
					log.Printf("[Calendar] ERROR saving skip log for rule %s: %v", rule.ID, err)
				}
				continue
			}

			// Create event
			newEvent := &calendar.Event{
				Summary: title,
				Start: &calendar.EventDateTime{
					DateTime: reminderTime.Format(time.RFC3339),
					TimeZone: event.Start.TimeZone,
				},
				End: &calendar.EventDateTime{
					DateTime: endTime.Format(time.RFC3339),
					TimeZone: event.End.TimeZone,
				},
				Description: fmt.Sprintf("Automatische reminder voor: %s\nGemaakt door regel: %s", event.Summary, rule.Name),
			}

			createdEvent, err := srv.Events.Insert("primary", newEvent).Do()
			if err != nil {
				log.Printf("[Calendar] ERROR creating reminder event: %v", err)

				triggerDetailsJSON, _ := json.Marshal(domain.TriggerLogDetails{
					GoogleEventID:  event.Id,
					TriggerSummary: event.Summary,
					TriggerTime:    startTime,
				})
				logParams := store.CreateLogParams{
					ConnectedAccountID: acc.ID,
					RuleID:             rule.ID,
					Status:             domain.LogFailure,
					TriggerDetails:     triggerDetailsJSON,
					ErrorMessage:       err.Error(),
				}
				if err = cp.store.CreateAutomationLog(ctx, logParams); err != nil {
					log.Printf("[Calendar] ERROR saving failure log for rule %s: %v", rule.ID, err)
				}
				continue
			}

			// Log success
			triggerDetailsJSON, _ := json.Marshal(domain.TriggerLogDetails{
				GoogleEventID:  event.Id,
				TriggerSummary: event.Summary,
				TriggerTime:    startTime,
			})
			actionDetailsJSON, _ := json.Marshal(domain.ActionLogDetails{
				CreatedEventID:      createdEvent.Id,
				CreatedEventSummary: createdEvent.Summary,
				ReminderTime:        reminderTime,
			})

			logParams := store.CreateLogParams{
				ConnectedAccountID: acc.ID,
				RuleID:             rule.ID,
				Status:             domain.LogSuccess,
				TriggerDetails:     triggerDetailsJSON,
				ActionDetails:      actionDetailsJSON,
			}
			if err = cp.store.CreateAutomationLog(ctx, logParams); err != nil {
				log.Printf("[Calendar] ERROR saving success log for rule %s: %v", rule.ID, err)
			}

			log.Printf(
				"[Calendar] SUCCESS: Created reminder '%s' (ID: %s) for event '%s' (ID: %s)",
				createdEvent.Summary,
				createdEvent.Id,
				event.Summary,
				event.Id,
			)
		}
	}

	return nil
}

// eventExists checks if an event with the same title exists in the given time window
func (cp *CalendarProcessor) eventExists(srv *calendar.Service, start, end time.Time, title string) bool {
	timeMin := start.Add(-1 * time.Minute).Format(time.RFC3339)
	timeMax := end.Add(1 * time.Minute).Format(time.RFC3339)

	events, err := srv.Events.List("primary").
		TimeMin(timeMin).
		TimeMax(timeMax).
		SingleEvents(true).
		Do()

	if err != nil {
		log.Printf("[Calendar] ERROR checking for existing event: %v", err)
		return false
	}

	for _, item := range events.Items {
		if item.Summary == title {
			return true
		}
	}

	return false
}
