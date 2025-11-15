package worker

import (
	// "agenda-automator-api/internal/crypto" // <-- VERWIJDERD
	"agenda-automator-api/internal/domain"
	"agenda-automator-api/internal/store"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// VERWIJDERD:
// var ErrTokenRevoked = fmt.Errorf("token access has been revoked by user")

// Worker is de struct voor onze achtergrond-processor
type Worker struct {
	store store.Storer
	// googleOAuthConfig *oauth2.Config // <-- VERWIJDERD (zit nu in store)
}

// NewWorker (AANGEPAST)
func NewWorker(s store.Storer) (*Worker, error) {
	return &Worker{
		store: s,
	}, nil
}

// Start lanceert de worker in een aparte goroutine
func (w *Worker) Start() {
	log.Println("Starting worker...")

	go w.run()
}

// run is de hoofdloop die periodiek de accounts controleert (real-time monitoring)
func (w *Worker) run() {
	// Verhoogd interval om API-limieten te respecteren
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	// Draai één keer direct bij het opstarten
	w.doWork()

	for {
		<-ticker.C
		w.doWork()
	}
}

// doWork is de daadwerkelijke werklading
func (w *Worker) doWork() {
	log.Println("[Worker] Running work cycle...")

	ctx, cancel := context.WithTimeout(context.Background(), 110*time.Second) // Iets korter dan ticker
	defer cancel()

	err := w.checkAccounts(ctx)
	if err != nil {
		log.Printf("[Worker] ERROR checking accounts: %v", err)
	}
}

// checkAccounts haalt alle accounts op, beheert tokens, en start de verwerking (parallel)
func (w *Worker) checkAccounts(ctx context.Context) error {
	accounts, err := w.store.GetActiveAccounts(ctx)
	if err != nil {
		return fmt.Errorf("could not get active accounts: %w", err)
	}

	log.Printf("[Worker] Found %d active accounts to check.", len(accounts))

	var wg sync.WaitGroup
	for _, acc := range accounts {
		wg.Add(1)
		go func(acc domain.ConnectedAccount) {
			defer wg.Done()
			w.processAccount(ctx, acc)
		}(acc)
	}
	wg.Wait()

	return nil
}

// processAccount (ZWAAR VEREENVOUDIGD)
func (w *Worker) processAccount(ctx context.Context, acc domain.ConnectedAccount) {
	// 1. Haal een gegarandeerd geldig token op.
	// De store regelt de decryptie, check, refresh, en update.
	token, err := w.store.GetValidTokenForAccount(ctx, acc.ID)
	if err != nil {
		// De store heeft de 'revoked' status al ingesteld,
		// we hoeven hier alleen nog maar te loggen en stoppen.
		if errors.Is(err, store.ErrTokenRevoked) {
			log.Printf("[Worker] Account %s is revoked. Stopping processing.", acc.ID)
		} else {
			log.Printf("[Worker] ERROR: Kon geen geldig token krijgen voor account %s: %v", acc.ID, err)
		}
		return // Stop verwerking voor dit account
	}

	// 2. Process calendar
	log.Printf("[Worker] Token for account %s is valid. Processing calendar...", acc.ID)
	if err := w.processCalendarEvents(ctx, acc, token); err != nil {
		log.Printf("[Worker] ERROR processing calendar for account %s: %v", acc.ID, err)
	}

	// 3. Update last_checked
	if err := w.store.UpdateAccountLastChecked(ctx, acc.ID); err != nil {
		log.Printf("[Worker] ERROR updating last_checked for account %s: %v", acc.ID, err)
	}
}

// VERWIJDERD: getTokenForAccount
// VERWIJDERD: refreshAccountToken

// eventExists (Bestaande code)
func eventExists(srv *calendar.Service, start, end time.Time, title string) bool {
	// Zoek in een iets ruimer venster om afrondingsfouten te vangen
	timeMin := start.Add(-1 * time.Minute).Format(time.RFC3339)
	timeMax := end.Add(1 * time.Minute).Format(time.RFC3339)

	events, err := srv.Events.List("primary").
		TimeMin(timeMin).
		TimeMax(timeMax).
		SingleEvents(true).
		Do()

	if err != nil {
		log.Printf("[Worker] ERROR checking for existing event: %v", err)
		// Veilige aanname: ga ervan uit dat het niet bestaat
		return false
	}

	for _, item := range events.Items {
		// Controleer op exacte titel-match
		if item.Summary == title {
			return true
		}
	}

	return false
}

// processCalendarEvents (Bestaande code)
func (w *Worker) processCalendarEvents(ctx context.Context, acc domain.ConnectedAccount, token *oauth2.Token) error {

	// De OAuth client is nu niet meer nodig in de worker,
	// maar wel om de calendar service te maken.
	// We halen hem op uit de store config.
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))

	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("could not create calendar service: %w", err)
	}

	// Ongelimiteerd: Haal alle events op
	tMin := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	tMax := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)

	log.Printf("[Worker] Fetching all calendar events for %s (unlimited)", acc.Email)

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

	rules, err := w.store.GetRulesForAccount(ctx, acc.ID)
	if err != nil {
		return fmt.Errorf("could not fetch automation rules: %w", err)
	}

	if len(events.Items) == 0 || len(rules) == 0 {
		log.Printf("[Worker] No upcoming events or no rules found for %s. Skipping.", acc.Email)
		return nil
	}

	log.Printf("[Worker] Checking %d events against %d rules for %s...", len(events.Items), len(rules), acc.Email)

	for _, event := range events.Items {

		// Voorkom dat we reageren op onze eigen aangemaakte events
		if strings.HasPrefix(event.Description, "Automatische reminder voor:") {
			continue
		}

		// (Aangepast: filter op actieve regels gebeurt nu in de DB query)
		for _, rule := range rules {
			// (Nieuwe check: de query haalt nu *alle* regels op, we moeten inactieve skippen)
			if !rule.IsActive {
				continue
			}

			var trigger domain.TriggerConditions
			if err := json.Unmarshal(rule.TriggerConditions, &trigger); err != nil {
				log.Printf("[Worker] ERROR unmarshaling trigger for rule %s: %v", rule.ID, err)
				continue
			}

			// --- 1. CHECK TRIGGERS ---
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

			// --- 1.5. CHECK LOGS (OPTIMALISATIE 1) ---
			hasLogged, err := w.store.HasLogForTrigger(ctx, rule.ID, event.Id)
			if err != nil {
				log.Printf("[Worker] ERROR checking logs for event %s / rule %s: %v", event.Id, rule.ID, err)
			}
			if hasLogged {
				continue
			}
			// --- EINDE OPTIMALISATIE 1.5 ---

			// --- 2. VOER ACTIE UIT ---
			log.Printf("[Worker] MATCH: Event '%s' (ID: %s) matches rule '%s'.", event.Summary, event.Id, rule.Name)

			var action domain.ActionParams
			if err := json.Unmarshal(rule.ActionParams, &action); err != nil {
				log.Printf("[Worker] ERROR unmarshaling action for rule %s: %v", rule.ID, err)
				continue
			}

			if action.NewEventTitle == "" {
				log.Printf("[Worker] ERROR: Rule %s heeft geen 'new_event_title'.", rule.ID)
				continue
			}

			startTime, err := time.Parse(time.RFC3339, event.Start.DateTime)
			if err != nil {
				log.Printf("[Worker] ERROR parsing start time: %v", err)
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

			// --- 3. CONTROLEER OP DUPLICATEN (SECUNDAIRE CHECK) ---
			if eventExists(srv, reminderTime, endTime, title) {
				log.Printf("[Worker] SKIP: Reminder event '%s' at %s already exists.", title, reminderTime)

				// --- 3.1. LOG DIT VOOR DE VOLGENDE KEER (OPTIMALISATIE 1) ---
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
				if err := w.store.CreateAutomationLog(ctx, logParams); err != nil {
					log.Printf("[Worker] ERROR saving skip log for rule %s: %v", rule.ID, err)
				}
				continue
			}

			// --- 4. MAAK EVENT AAN ---
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
				log.Printf("[Worker] ERROR creating reminder event: %v", err)

				// --- 4.1 LOG FAILURE (OPTIMALISATIE 1) ---
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
				if err := w.store.CreateAutomationLog(ctx, logParams); err != nil {
					log.Printf("[Worker] ERROR saving failure log for rule %s: %v", rule.ID, err)
				}
				continue
			}

			// --- 5. LOG SUCCESS (OPTIMALISATIE 1) ---
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
			if err := w.store.CreateAutomationLog(ctx, logParams); err != nil {
				log.Printf("[Worker] ERROR saving success log for rule %s: %v", rule.ID, err)
			}

			log.Printf("[Worker] SUCCESS: Created reminder '%s' (ID: %s) for event '%s' (ID: %s)", createdEvent.Summary, createdEvent.Id, event.Summary, event.Id)
		}
	}

	return nil
}
