package worker

import (
	"agenda-automator-api/internal/crypto"
	"agenda-automator-api/internal/domain"
	"agenda-automator-api/internal/store"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync" // NIEUW voor parallel
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

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
	ticker := time.NewTicker(30 * time.Second) // Real-time: elke 30 seconden
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

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
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

// processAccount (nieuw: per account goroutine logica)
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
			log.Printf("[Worker] ERROR refreshing account %s: %v", acc.ID, err)
			// TODO: Markeer als 'error'
			return
		}

		err = w.store.UpdateAccountTokens(ctx, store.UpdateAccountTokensParams{
			AccountID:       acc.ID,
			NewAccessToken:  newToken.AccessToken,
			NewRefreshToken: newToken.RefreshToken,
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

// refreshAccountToken ververst een verlopen token en geeft het nieuwe terug
func (w *Worker) refreshAccountToken(ctx context.Context, expiredToken *oauth2.Token) (*oauth2.Token, error) {
	ts := w.googleOAuthConfig.TokenSource(ctx, expiredToken)

	newToken, err := ts.Token()
	if err != nil {
		return nil, fmt.Errorf("could not refresh token (access possibly revoked): %w", err)
	}

	return newToken, nil
}

// processCalendarEvents (AANGEPAST voor shift automation logica)
func (w *Worker) processCalendarEvents(ctx context.Context, acc domain.ConnectedAccount, token *oauth2.Token) error {

	client := w.googleOAuthConfig.Client(ctx, token)

	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("could not create calendar service: %w", err)
	}

	// Real-time monitoring: Check alle events voor komende jaar
	tMin := time.Now().Format(time.RFC3339)
	tMax := time.Now().AddDate(1, 0, 0).Format(time.RFC3339) // 1 jaar vooruit

	log.Printf("[Worker] Real-time monitoring: Fetching calendar events for %s between %s and %s", acc.Email, tMin, tMax)

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

	if len(events.Items) > 0 {
		log.Printf("[Worker] FOUND %d UPCOMING EVENTS FOR %s!", len(events.Items), acc.Email)
		log.Printf("[Worker] Account has %d rules to check against.", len(rules))

		for _, event := range events.Items {
			for _, rule := range rules {
				var trigger domain.TriggerConditions
				if err := json.Unmarshal(rule.TriggerConditions, &trigger); err != nil {
					log.Printf("[Worker] ERROR unmarshaling trigger for rule %s: %v", rule.ID, err)
					continue
				}

				var action domain.ActionParams
				if err := json.Unmarshal(rule.ActionParams, &action); err != nil {
					log.Printf("[Worker] ERROR unmarshaling action for rule %s: %v", rule.ID, err)
					continue
				}

				// Check trigger: exact summary match (nieuw voor "Dienst")
				if trigger.SummaryEquals != "" && event.Summary != trigger.SummaryEquals {
					continue
				}

				// Fallback op contains als exact niet gebruikt
				matched := false
				if len(trigger.SummaryContains) > 0 {
					for _, contain := range trigger.SummaryContains {
						if strings.Contains(event.Summary, contain) {
							matched = true
							break
						}
					}
					if !matched {
						continue
					}
				} else if trigger.SummaryEquals == "" {
					continue // Geen trigger gedefinieerd
				}

				log.Printf("[Worker] MATCH: Event '%s' matches rule '%s'. Creating reminder...", event.Summary, rule.Name)

				// Parse start time
				startTime, err := time.Parse(time.RFC3339, event.Start.DateTime)
				if err != nil {
					log.Printf("[Worker] ERROR parsing start time: %v", err)
					continue
				}

				// Bepaal type (Vroeg/Laat) gebaseerd op startuur
				shiftType := "Laat"
				if startTime.Hour() < 12 {
					shiftType = "Vroeg"
				}

				// Bepaal team (A/R) gebaseerd op location
				team := "R"
				locLower := strings.ToLower(event.Location)
				if strings.Contains(locLower, "aa") || strings.Contains(locLower, "appartementen") {
					team = "A"
				}

				// Title samenstellen
				title := fmt.Sprintf("%s %s", shiftType, team)

				// Calculate reminder time
				offset := action.OffsetMinutes
				if offset == 0 {
					offset = -60 // Default 1 uur voor
				}
				reminderTime := startTime.Add(time.Duration(offset) * time.Minute)

				// Duur (default 5 min)
				durMin := action.DurationMin
				if durMin == 0 {
					durMin = 5
				}
				endTime := reminderTime.Add(time.Duration(durMin) * time.Minute)

				// Create new event
				newEvent := &calendar.Event{
					Summary: title,
					Start: &calendar.EventDateTime{
						DateTime: reminderTime.Format(time.RFC3339),
					},
					End: &calendar.EventDateTime{
						DateTime: endTime.Format(time.RFC3339),
					},
				}

				createdEvent, err := srv.Events.Insert("primary", newEvent).Do()
				if err != nil {
					log.Printf("[Worker] ERROR creating reminder event: %v", err)
					continue
				}

				log.Printf("[Worker] Created reminder event '%s' at %s: %s", title, reminderTime, createdEvent.Id)
			}
		}
	} else {
		log.Printf("[Worker] No upcoming events found for %s.", acc.Email)
	}

	return nil
}
