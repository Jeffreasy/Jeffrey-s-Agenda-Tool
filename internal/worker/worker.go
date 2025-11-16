package worker

import (
	// "agenda-automator-api/internal/crypto" // <-- VERWIJDERD
	"agenda-automator-api/internal/domain"
	// "agenda-automator-api/internal/logger" // <-- VERWIJDERD (niet meer nodig)
	"agenda-automator-api/internal/store"
	"agenda-automator-api/internal/worker/calendar"
	"agenda-automator-api/internal/worker/gmail"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// VERWIJDERD:
// var ErrTokenRevoked = fmt.Errorf("token access has been revoked by user")

// Worker is de struct voor onze achtergrond-processor
type Worker struct {
	store             store.Storer
	logger            *zap.Logger
	calendarProcessor *calendar.CalendarProcessor
	gmailProcessor    *gmail.GmailProcessor
	// googleOAuthConfig *oauth2.Config // <-- VERWIJDERD (zit nu in store)
}

// NewWorker (AANGEPAST)
func NewWorker(s store.Storer, logger *zap.Logger) (*Worker, error) {
	return &Worker{
		store:             s,
		logger:            logger,
		calendarProcessor: calendar.NewCalendarProcessor(s),
		gmailProcessor:    gmail.NewGmailProcessor(s),
	}, nil
}

// Start lanceert de worker in een aparte goroutine
func (w *Worker) Start() {
	w.logger.Info("starting worker", zap.String("component", "worker"))

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
	w.logger.Info("running work cycle", zap.String("component", "worker"))

	ctx, cancel := context.WithTimeout(context.Background(), 110*time.Second) // Iets korter dan ticker
	defer cancel()

	err := w.checkAccounts(ctx)
	if err != nil {
		w.logger.Error("failed to check accounts", zap.Error(err), zap.String("component", "worker"))
	}
}

// checkAccounts haalt alle accounts op, beheert tokens, en start de verwerking (parallel)
func (w *Worker) checkAccounts(ctx context.Context) error {
	accounts, err := w.store.GetActiveAccounts(ctx)
	if err != nil {
		return fmt.Errorf("could not get active accounts: %w", err)
	}

	w.logger.Info("found active accounts to check", zap.Int("count", len(accounts)), zap.String("component", "worker"))

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
			w.logger.Warn(
				"account token revoked, stopping processing",
				zap.String("account_id", acc.ID.String()),
				zap.String("component", "worker"),
			)
		} else {
			w.logger.Error(
				"failed to get valid token for account",
				zap.Error(err),
				zap.String("account_id", acc.ID.String()),
				zap.String("component", "worker"),
			)
		}
		return // Stop verwerking voor dit account
	}

	// 2. Process calendar
	// AANGEPAST: Gebruik w.logger
	w.logger.Info(
		"token valid, processing calendar",
		zap.String("account_id", acc.ID.String()),
		zap.String("component", "worker"),
	)
	if err := w.calendarProcessor.ProcessEvents(ctx, acc, token); err != nil {
		// AANGEPAST: Gebruik w.logger
		w.logger.Error(
			"failed to process calendar events",
			zap.Error(err),
			zap.String("account_id", acc.ID.String()),
			zap.String("component", "worker"),
		)
	}

	// 2.5. Process Gmail (only if Gmail sync is enabled)
	if acc.GmailSyncEnabled {
		// AANGEPAST: Gebruik w.logger
		w.logger.Info(
			"processing Gmail messages",
			zap.String("account_id", acc.ID.String()),
			zap.String("component", "worker"),
		)
		if err := w.gmailProcessor.ProcessMessages(ctx, acc, token); err != nil {
			// AANGEPAST: Gebruik w.logger
			w.logger.Error(
				"failed to process Gmail messages",
				zap.Error(err),
				zap.String("account_id", acc.ID.String()),
				zap.String("component", "worker"),
			)
		}
	}

	// 3. Update last_checked
	if err := w.store.UpdateAccountLastChecked(ctx, acc.ID); err != nil {
		// AANGEPAST: Gebruik w.logger
		w.logger.Error(
			"failed to update last_checked timestamp",
			zap.Error(err),
			zap.String("account_id", acc.ID.String()),
			zap.String("component", "worker"),
		)
	}
}
