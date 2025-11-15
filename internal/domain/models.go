package domain

import (
	"encoding/json" // Zorg dat deze import er is
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

type AutomationLogStatus string

const (
	LogPending AutomationLogStatus = "pending"
	LogSuccess AutomationLogStatus = "success"
	LogFailure AutomationLogStatus = "failure"
	LogSkipped AutomationLogStatus = "skipped"
)

// --- Tabel Structs (met JSON tags) ---

type User struct {
	ID        uuid.UUID `db:"id"        json:"id"`
	Email     string    `db:"email"     json:"email"`
	Name      *string   `db:"name"      json:"name,omitempty"` // AANGEPAST: van 'string' naar '*string'
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

type ConnectedAccount struct {
	ID             uuid.UUID     `db:"id"                 json:"id"`
	UserID         uuid.UUID     `db:"user_id"            json:"user_id"`
	Provider       ProviderType  `db:"provider"           json:"provider"`
	Email          string        `db:"email"              json:"email"`
	ProviderUserID string        `db:"provider_user_id"   json:"provider_user_id"`
	AccessToken    []byte        `db:"access_token"       json:"-"`
	RefreshToken   []byte        `db:"refresh_token"      json:"-"`
	TokenExpiry    time.Time     `db:"token_expiry"       json:"token_expiry"`
	Scopes         []string      `db:"scopes"             json:"scopes"`
	Status         AccountStatus `db:"status"             json:"status"`
	CreatedAt      time.Time     `db:"created_at"         json:"created_at"`
	UpdatedAt      time.Time     `db:"updated_at"         json:"updated_at"`
	LastChecked    *time.Time    `db:"last_checked"       json:"last_checked"`
}

type AutomationRule struct {
	ID                 uuid.UUID       `db:"id"                     json:"id"`
	ConnectedAccountID uuid.UUID       `db:"connected_account_id"   json:"connected_account_id"`
	Name               string          `db:"name"                   json:"name"`
	IsActive           bool            `db:"is_active"              json:"is_active"`
	TriggerConditions  json.RawMessage `db:"trigger_conditions"     json:"trigger_conditions"`
	ActionParams       json.RawMessage `db:"action_params"          json:"action_params"`
	CreatedAt          time.Time       `db:"created_at"             json:"created_at"`
	UpdatedAt          time.Time       `db:"updated_at"             json:"updated_at"`
}

type AutomationLog struct {
	ID                 int64               `db:"id"                     json:"id"`
	ConnectedAccountID uuid.UUID           `db:"connected_account_id"   json:"connected_account_id"`
	RuleID             uuid.UUID           `db:"rule_id"                json:"rule_id"`
	Timestamp          time.Time           `db:"timestamp"              json:"timestamp"`
	Status             AutomationLogStatus `db:"status"                 json:"status"`
	TriggerDetails     json.RawMessage     `db:"trigger_details"        json:"trigger_details"`
	ActionDetails      json.RawMessage     `db:"action_details"         json:"action_details"`
	ErrorMessage       string              `db:"error_message"          json:"error_message"`
}

// ... rest van het bestand (TriggerConditions, ActionParams, etc.) ...
// (Deze hoeven niet aangepast te worden)
type TriggerConditions struct {
	SummaryEquals    string   `json:"summary_equals,omitempty"`
	SummaryContains  []string `json:"summary_contains,omitempty"`
	LocationContains []string `json:"location_contains,omitempty"`
}

type ActionParams struct {
	OffsetMinutes int    `json:"offset_minutes"`
	NewEventTitle string `json:"new_event_title"`
	DurationMin   int    `json:"duration_min"`
}

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

type Event struct {
	ID          string    `json:"id"`
	Summary     string    `json:"summary"`
	Description string    `json:"description"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
	CalendarId  string    `json:"calendarId"`
}
