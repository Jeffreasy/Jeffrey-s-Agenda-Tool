package domain

import (
	"time"

	"github.com/google/uuid"
)

// --- ENUM Types ---
type ProviderType string

const (
	ProviderGoogle    ProviderType = "google"
	ProviderMicrosoft ProviderType = "microsoft"
)

type AccountStatus string

const (
	StatusActive  AccountStatus = "active"
	StatusRevoked AccountStatus = "revoked"
	StatusError   AccountStatus = "error"
	StatusPaused  AccountStatus = "paused"
)

type AutomationLogStatus string // Aangepast voor consistentie

const (
	LogPending AutomationLogStatus = "pending"
	LogSuccess AutomationLogStatus = "success"
	LogFailure AutomationLogStatus = "failure"
	LogSkipped AutomationLogStatus = "skipped"
)

// --- Tabel Structs ---
type User struct {
	ID        uuid.UUID `db:"id"`
	Email     string    `db:"email"`
	Name      string    `db:"name"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

type ConnectedAccount struct {
	ID             uuid.UUID     `db:"id"`
	UserID         uuid.UUID     `db:"user_id"`
	Provider       ProviderType  `db:"provider"`
	Email          string        `db:"email"`
	ProviderUserID string        `db:"provider_user_id"`
	AccessToken    []byte        `db:"access_token"` // Blijft []byte voor encryptie
	RefreshToken   []byte        `db:"refresh_token"`
	TokenExpiry    time.Time     `db:"token_expiry"`
	Scopes         []string      `db:"scopes"` // pgx kan []string naar text[] mappen
	Status         AccountStatus `db:"status"`
	CreatedAt      time.Time     `db:"created_at"`
	UpdatedAt      time.Time     `db:"updated_at"`
	LastChecked    time.Time     `db:"last_checked"` // NIEUW
}

type AutomationRule struct {
	ID                 uuid.UUID `db:"id"`
	ConnectedAccountID uuid.UUID `db:"connected_account_id"`
	Name               string    `db:"name"`
	IsActive           bool      `db:"is_active"`
	TriggerConditions  []byte    `db:"trigger_conditions"` // JSONB as []byte
	ActionParams       []byte    `db:"action_params"`      // JSONB as []byte
	CreatedAt          time.Time `db:"created_at"`
	UpdatedAt          time.Time `db:"updated_at"`
}

type AutomationLog struct {
	ID                 int64               `db:"id"` // bigserial mapt naar int64
	ConnectedAccountID uuid.UUID           `db:"connected_account_id"`
	RuleID             uuid.UUID           `db:"rule_id"` // pgx kan 'NULL' UUID's aan
	Timestamp          time.Time           `db:"timestamp"`
	Status             AutomationLogStatus `db:"status"`
	TriggerDetails     []byte              `db:"trigger_details"` // JSONB as []byte
	ActionDetails      []byte              `db:"action_details"`  // JSONB as []byte
	ErrorMessage       string              `db:"error_message"`   // pgx mapt 'NULL' text naar ""
}

// Aangepaste structs voor JSONB met meer flexibiliteit voor shift automation
type TriggerConditions struct {
	SummaryEquals    string   `json:"summary_equals,omitempty"`
	SummaryContains  []string `json:"summary_contains,omitempty"`
	LocationContains []string `json:"location_contains,omitempty"`
}

type ActionParams struct {
	OffsetMinutes int    `json:"offset_minutes"` // Bijv. -60 voor 1 uur voor shift
	NewEventTitle string `json:"new_event_title"`
	DurationMin   int    `json:"duration_min"` // Bijv. 5 voor reminder duur
}

// --- OPTIMALISATIE: Structs voor JSONB Logging ---
type TriggerLogDetails struct {
	GoogleEventID  string    `json:"google_event_id"`
	TriggerSummary string    `json:"trigger_summary"`
	TriggerTime    time.Time `json:"trigger_time"`
}

type ActionLogDetails struct {
	CreatedEventID      string    `json:"created_event_id"`
	CreatedEventSummary string    `json:"created_event_summary"`
	ReminderTime        time.Time `json:"reminder_time"`
}
