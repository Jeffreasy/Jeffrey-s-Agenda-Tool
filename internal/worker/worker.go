// Vervang hiermee: internal/worker/worker.go
package worker

import (
	"agenda-automator-api/internal/crypto"
	"agenda-automator-api/internal/domain"
	"agenda-automator-api/internal/store"
	"context"
	"fmt"
	"log"

	// "os" // <-- Niet meer nodig hier
	"time"

	"golang.org/x/oauth2"
	// "golang.org/x/oauth2/google" // <-- Niet meer nodig hier

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// Worker is de struct voor onze achtergrond-processor
type Worker struct {
	store             store.Storer
	googleOAuthConfig *oauth2.Config // Blijft hetzelfde
}

// NewWorker (AANGEPAST) - accepteert nu de config
func NewWorker(s store.Storer, oauthCfg *oauth2.Config) (*Worker, error) {
	if oauthCfg == nil {
		return nil, fmt.Errorf("oauth config mag niet nil zijn")
	}

	return &Worker{
		store:             s,
		googleOAuthConfig: oauthCfg,
	}, nil
}

// Start, run, doWork, checkAccounts, getTokenForAccount,
// refreshAccountToken, processCalendarEvents
//
// ... (AL DE REST VAN JE FUNCTIES HIER) ...
// ... (Er verandert hier niets, plak je bestaande code terug) ...
//
// Start lanceert de worker in een aparte goroutine
func (w *Worker) Start() {
	log.Println("Starting worker...")

	go w.run()
}

// run is de hoofdloop die periodiek de accounts controleert
func (w *Worker) run() {
	ticker := time.NewTicker(1 * time.Minute)
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

// checkAccounts haalt alle accounts op, beheert tokens, en start de verwerking
func (w *Worker) checkAccounts(ctx context.Context) error {
	accounts, err := w.store.GetActiveAccounts(ctx)
	if err != nil {
		return fmt.Errorf("could not get active accounts: %w", err)
	}

	log.Printf("[Worker] Found %d active accounts to check.", len(accounts))

	for _, acc := range accounts {
		// Maak een 'token' object van de data in de DB
		token, err := w.getTokenForAccount(acc)
		if err != nil {
			log.Printf("[Worker] ERROR: Kon token niet voorbereiden voor account %s: %v", acc.ID, err)
			continue
		}

		// Controleer of het token (bijna) verlopen is
		if !token.Valid() { // .Valid() checkt de expiry
			log.Printf("[Worker] Token for account %s (User %s) is expired. Refreshing...", acc.ID, acc.UserID)

			// Probeer te verversen
			newToken, err := w.refreshAccountToken(ctx, token)
			if err != nil {
				log.Printf("[Worker] ERROR refreshing account %s: %v", acc.ID, err)
				// TODO: Markeer account als 'error' of 'revoked'
				continue // Ga naar het volgende account
			}

			// Sla het nieuwe token op in de DB
			err = w.store.UpdateAccountTokens(ctx, store.UpdateAccountTokensParams{
				AccountID:       acc.ID,
				NewAccessToken:  newToken.AccessToken,
				NewRefreshToken: newToken.RefreshToken, // Kan leeg zijn
				NewTokenExpiry:  newToken.Expiry,
			})
			if err != nil {
				log.Printf("[Worker] ERROR updating refreshed token for account %s: %v", acc.ID, err)
				continue
			}

			token = newToken // Gebruik het *nieuwe* token voor de rest van deze run
		}

		// --- SUCCESPAD ---
		// We hebben nu een gegarandeerd geldig (oud of net ververst) token
		log.Printf("[Worker] Token for account %s is valid. Processing calendar...", acc.ID)
		if err := w.processCalendarEvents(ctx, acc, token); err != nil {
			log.Printf("[Worker] ERROR processing calendar for account %s: %v", acc.ID, err)
		}
	}
	return nil
}

// getTokenForAccount haalt de versleutelde tokens op en maakt een *oauth2.Token
func (w *Worker) getTokenForAccount(acc domain.ConnectedAccount) (*oauth2.Token, error) {
	// We hoeven het access token niet te decrypten, de oauth2 library
	// heeft alleen het refresh token nodig om te *kunnen* verversen.
	plaintextRefreshToken, err := crypto.Decrypt(acc.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("could not decrypt refresh token: %w", err)
	}

	return &oauth2.Token{
		AccessToken:  "dummy-value-voor-valid-check", // Dit wordt genegeerd
		RefreshToken: string(plaintextRefreshToken),
		Expiry:       acc.TokenExpiry,
	}, nil
}

// refreshAccountToken ververst een verlopen token en geeft het nieuwe terug
func (w *Worker) refreshAccountToken(ctx context.Context, expiredToken *oauth2.Token) (*oauth2.Token, error) {
	ts := w.googleOAuthConfig.TokenSource(ctx, expiredToken)

	newToken, err := ts.Token() // Doet de HTTP-call
	if err != nil {
		return nil, fmt.Errorf("could not refresh token (access possibly revoked): %w", err)
	}

	return newToken, nil
}

// --- DE NIEUWE AUTOMATISERINGSFUNCTIE ---
func (w *Worker) processCalendarEvents(ctx context.Context, acc domain.ConnectedAccount, token *oauth2.Token) error {

	// 1. Maak een geauthenticeerde HTTP-client
	client := w.googleOAuthConfig.Client(ctx, token)

	// 2. Maak de Google Calendar service
	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("could not create calendar service: %w", err)
	}

	// 3. Haal de agenda-items op
	// We zoeken naar events die *net* zijn begonnen (afgelopen minuut)
	// Dit is een simpele "nieuwe afspraak" trigger
	tMin := time.Now().Add(-1 * time.Minute).Format(time.RFC3339)
	tMax := time.Now().Add(1 * time.Minute).Format(time.RFC3339)

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

	// 4. Haal de regels voor dit account op
	rules, err := w.store.GetRulesForAccount(ctx, acc.ID)
	if err != nil {
		return fmt.Errorf("could not fetch automation rules: %w", err)
	}

	// 5. De "Trigger" Logica (voor nu alleen loggen)
	if len(events.Items) > 0 {
		log.Printf("[Worker] FOUND %d NEW/ONGOING EVENTS FOR %s!", len(events.Items), acc.Email)
		log.Printf("[Worker] Account has %d rules to check against.", len(rules))

		for _, event := range events.Items {
			for _, rule := range rules {
				// HIER komt je jsonb-logica
				// bijv. json.Unmarshal(rule.TriggerConditions, &trigger)
				// if event.Summary == trigger.TitleContains ...
				log.Printf("[Worker] ...Checking Event '%s' against Rule '%s'", event.Summary, rule.Name)
			}
		}
	} else {
		log.Printf("[Worker] No new events found for %s.", acc.Email)
	}

	return nil
}
