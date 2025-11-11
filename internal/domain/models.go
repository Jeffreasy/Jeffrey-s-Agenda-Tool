// Vervang hiermee internal/domain/models.go
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

type LogStatus string

const (
	LogPending LogStatus = "pending"
	LogSuccess LogStatus = "success"
	LogFailure LogStatus = "failure"
	LogSkipped LogStatus = "skipped"
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
	ID                 int64     `db:"id"` // bigserial mapt naar int64
	ConnectedAccountID uuid.UUID `db:"connected_account_id"`
	RuleID             uuid.UUID `db:"rule_id"` // pgx kan 'NULL' UUID's aan
	Timestamp          time.Time `db:"timestamp"`
	Status             LogStatus `db:"status"`
	TriggerDetails     []byte    `db:"trigger_details"` // JSONB as []byte
	ActionDetails      []byte    `db:"action_details"`  // JSONB as []byte
	ErrorMessage       string    `db:"error_message"`   // pgx mapt 'NULL' text naar ""
}
