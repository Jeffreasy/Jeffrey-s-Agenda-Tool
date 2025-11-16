package domain

import (
	"encoding/json"
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

// --- Base Structs ---

type BaseEntity struct {
	ID        uuid.UUID `db:"id"        json:"id"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

type AccountEntity struct {
	BaseEntity
	ConnectedAccountID uuid.UUID `db:"connected_account_id" json:"connected_account_id"`
}

type BaseAutomationRule struct {
	AccountEntity
	Name              string          `db:"name"              json:"name"`
	IsActive          bool            `db:"is_active"         json:"is_active"`
	TriggerConditions json.RawMessage `db:"trigger_conditions" json:"trigger_conditions"`
	ActionParams      json.RawMessage `db:"action_params"     json:"action_params"`
}

type BaseAutomationLog struct {
	AccountEntity
	Timestamp      time.Time           `db:"timestamp"       json:"timestamp"`
	Status         AutomationLogStatus `db:"status"          json:"status"`
	TriggerDetails json.RawMessage     `db:"trigger_details" json:"trigger_details"`
	ActionDetails  json.RawMessage     `db:"action_details"  json:"action_details"`
	ErrorMessage   string              `db:"error_message"    json:"error_message"`
}

