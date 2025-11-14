package worker

import (
	"agenda-automator-api/internal/crypto"
	"agenda-automator-api/internal/domain"
	"agenda-automator-api/internal/store"
	"context"
	"encoding/json"
	"errors" // NIEUW
	"fmt"
	"log"
	"strings"
	"sync" // NIEUW voor parallel
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// NIEUW: (Optimalisatie 2) Specifieke error voor revoked tokens
var ErrTokenRevoked = fmt.Errorf("token access has been revoked by user")

// Worker is de struct voor onze achtergrond-processor
type Worker struct {
	store             store.Storer
	googleOAuthConfig *oauth2.Config
}

// NewWorker ...
func NewWorker(s store.Storer, oauthCfg *oauth2.Config) (*Worker, error) {
	if oauthCfg == nil {
		return nil, fmt.Errorf("oauth config mag niet nil zijn")
	}

	return &Worker{
		store:             s,
		googleOAuthConfig: oauthCfg,
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

// processAccount (AANGEPAST met Optimalisatie 2)
func (w *Worker) processAccount(ctx context.Context, acc domain.ConnectedAccount) {
	// Maak een 'token' object van de data in de DB
	token, err := w.getTokenForAccount(acc)
	if err != nil {
		log.Printf("[Worker] ERROR: Kon token niet voorbereiden voor account %s: %v", acc.ID, err)
		return
	}

	// Controleer of het token (bijna) verlopen is
	if !token.Valid() {
		log.Printf("[Worker] Token for account %s (User %s) is expired. Refreshing...", acc.ID, acc.UserID)

		newToken, err := w.refreshAccountToken(ctx, token)
		if err != nil {
			// --- OPTIMALISATIE 2: ERROR HANDLING ---
			if errors.Is(err, ErrTokenRevoked) {
				log.Printf("[Worker] FATAL: Access for account %s has been revoked. Setting status to 'revoked'.", acc.ID)
				// Zet account op 'revoked' in DB zodat we het niet opnieuw proberen
				if err := w.store.UpdateAccountStatus(ctx, acc.ID, domain.StatusRevoked); err != nil {
					log.Printf("[Worker] ERROR: Failed to update status for revoked account %s: %v", acc.ID, err)
				}
			} else {
				// Andere, tijdelijke refresh error
				log.Printf("[Worker] ERROR refreshing account %s: %v", acc.ID, err)
			}
			return // Stop verwerking voor dit account
			// --- EINDE OPTIMALISATIE 2 ---
		}

		err = w.store.UpdateAccountTokens(ctx, store.UpdateAccountTokensParams{
			AccountID:       acc.ID,
			NewAccessToken:  newToken.AccessToken,
			NewRefreshToken: newToken.RefreshToken, // Zorg dat we de nieuwe refresh token opslaan
			NewTokenExpiry:  newToken.Expiry,
		})
		if err != nil {
			log.Printf("[Worker] ERROR updating refreshed token for account %s: %v", acc.ID, err)
			return
		}

		token = newToken
	}

	// Process calendar
	log.Printf("[Worker] Token for account %s is valid. Processing calendar...", acc.ID)
	if err := w.processCalendarEvents(ctx, acc, token); err != nil {
		log.Printf("[Worker] ERROR processing calendar for account %s: %v", acc.ID, err)
	}

	// Update last_checked
	if err := w.store.UpdateAccountLastChecked(ctx, acc.ID); err != nil {
		log.Printf("[Worker] ERROR updating last_checked for account %s: %v", acc.ID, err)
	}
}

// getTokenForAccount haalt de versleutelde tokens op en maakt een *oauth2.Token (decrypt both)
func (w *Worker) getTokenForAccount(acc domain.ConnectedAccount) (*oauth2.Token, error) {
	plaintextAccessToken, err := crypto.Decrypt(acc.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("could not decrypt access token: %w", err)
	}

	var plaintextRefreshToken []byte
	if len(acc.RefreshToken) > 0 {
		plaintextRefreshToken, err = crypto.Decrypt(acc.RefreshToken)
		if err != nil {
			return nil, fmt.Errorf("could not decrypt refresh token: %w", err)
		}
	}

	return &oauth2.Token{
		AccessToken:  string(plaintextAccessToken),
		RefreshToken: string(plaintextRefreshToken),
		Expiry:       acc.TokenExpiry,
		TokenType:    "Bearer",
	}, nil
}

// refreshAccountToken (AANGEPAST met Optimalisatie 2)
func (w *Worker) refreshAccountToken(ctx context.Context, expiredToken *oauth2.Token) (*oauth2.Token, error) {
	ts := w.googleOAuthConfig.TokenSource(ctx, expiredToken)

	newToken, err := ts.Token()
	if err != nil {
		// --- OPTIMALISATIE 2: Vang 'invalid_grant' ---
		// Dit gebeurt als de gebruiker de toegang intrekt
		if strings.Contains(err.Error(), "invalid_grant") {
			return nil, ErrTokenRevoked
		}
		// --- EINDE OPTIMALISATIE 2 ---
		return nil, fmt.Errorf("could not refresh token: %w", err)
	}

	// Als we GEEN nieuwe refresh token krijgen, hergebruik dan de oude
	if newToken.RefreshToken == "" {
		newToken.RefreshToken = expiredToken.RefreshToken
	}

	return newToken, nil
}

// eventExists controleert of een event met dezelfde titel al bestaat in de tijdsslot
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

// processCalendarEvents (AANGEPAST met Optimalisatie 1: Logging)
func (w *Worker) processCalendarEvents(ctx context.Context, acc domain.ConnectedAccount, token *oauth2.Token) error {

	client := w.googleOAuthConfig.Client(ctx, token)

	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("could not create calendar service: %w", err)
	}

	// Aangepast: Check 3 maanden vooruit.
	tMin := time.Now().Format(time.RFC3339)
	tMax := time.Now().AddDate(0, 3, 0).Format(time.RFC3339) // 3 maanden vooruit

	log.Printf("[Worker] Fetching calendar events for %s between %s and %s", acc.Email, tMin, tMax)

	events, err := srv.Events.List("primary").
		TimeMin(tMin).
		TimeMax(tMax).
		SingleEvents(true).
		OrderBy("startTime").
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

		for _, rule := range rules {
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
			// Sla de rest over als we deze trigger event al succesvol hebben verwerkt.
			// Dit voorkomt dubbel werk en onnodige eventExists API calls.
			hasLogged, err := w.store.HasLogForTrigger(ctx, rule.ID, event.Id)
			if err != nil {
				log.Printf("[Worker] ERROR checking logs for event %s / rule %s: %v", event.Id, rule.ID, err)
				// Ga door (veilige aanname), de eventExists check vangt het wel
			}
			if hasLogged {
				// log.Printf("[Worker] SKIP: Trigger event %s already processed by rule %s.", event.Id, rule.ID)
				continue // Ga naar de volgende regel
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
				// We vonden geen log (stap 1.5), maar het event bestaat wel (stap 3).
				// We loggen het nu alsnog om toekomstige API-checks te voorkomen.
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
					Status:             domain.LogSkipped, // We skippen, want het bestond al
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
