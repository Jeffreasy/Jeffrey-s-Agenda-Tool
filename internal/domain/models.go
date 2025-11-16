package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// User represents a user account
type User struct {
	BaseEntity
	Email string  `db:"email"     json:"email"`
	Name  *string `db:"name"      json:"name,omitempty"` // AANGEPAST: van 'string' naar '*string'
}

// ConnectedAccount represents a connected third-party account
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
	// Gmail-specific fields
	GmailHistoryID   *string    `db:"gmail_history_id"    json:"gmail_history_id,omitempty"`
	GmailLastSync    *time.Time `db:"gmail_last_sync"     json:"gmail_last_sync,omitempty"`
	GmailSyncEnabled bool       `db:"gmail_sync_enabled"  json:"gmail_sync_enabled"`
}

// AutomationRule represents an automation rule
type AutomationRule struct {
	BaseAutomationRule
}

// AutomationLog represents a log entry for automation execution
type AutomationLog struct {
	ID                 int64               `db:"id"                     json:"id"`
	ConnectedAccountID uuid.UUID           `db:"connected_account_id"   json:"connected_account_id"`
	RuleID             uuid.UUID           `db:"rule_id"                json:"rule_id"`
	Timestamp          time.Time           `db:"timestamp"              json:"timestamp"`
	Status             AutomationLogStatus `db:"status"                 json:"status"`
	TriggerDetails     json.RawMessage     `db:"trigger_details"        json:"trigger_details"`
	ActionDetails      json.RawMessage     `db:"action_details"         json:"action_details"`
	ErrorMessage       string              `db:"error_message"           json:"error_message"`
}

// TriggerConditions represents conditions for triggering automation
type TriggerConditions struct {
	SummaryEquals    string   `json:"summary_equals,omitempty"`
	SummaryContains  []string `json:"summary_contains,omitempty"`
	LocationContains []string `json:"location_contains,omitempty"`
}

// ActionParams represents parameters for automation actions
type ActionParams struct {
	OffsetMinutes int    `json:"offset_minutes"`
	NewEventTitle string `json:"new_event_title"`
	DurationMin   int    `json:"duration_min"`
}

// TriggerLogDetails represents details of a trigger event
type TriggerLogDetails struct {
	GoogleEventID  string    `json:"google_event_id"`
	TriggerSummary string    `json:"trigger_summary"`
	TriggerTime    time.Time `json:"trigger_time"`
}

// ActionLogDetails represents details of an action execution
type ActionLogDetails struct {
	CreatedEventID      string    `json:"created_event_id"`
	CreatedEventSummary string    `json:"created_event_summary"`
	ReminderTime        time.Time `json:"reminder_time"`
}

// Event represents a calendar event
type Event struct {
	ID          string    `json:"id"`
	Summary     string    `json:"summary"`
	Description string    `json:"description"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
	CalendarId  string    `json:"calendarId"`
}

// GmailMessageStatus represents the status of a Gmail message
type GmailMessageStatus string

const (
	GmailUnread   GmailMessageStatus = "unread"
	GmailRead     GmailMessageStatus = "read"
	GmailArchived GmailMessageStatus = "archived"
	GmailTrashed  GmailMessageStatus = "trashed"
	GmailSpam     GmailMessageStatus = "spam"
)

// GmailRuleTriggerType represents types of triggers for Gmail rules
type GmailRuleTriggerType string

const (
	GmailTriggerNewMessage   GmailRuleTriggerType = "new_message"
	GmailTriggerSenderMatch  GmailRuleTriggerType = "sender_match"
	GmailTriggerSubjectMatch GmailRuleTriggerType = "subject_match"
	GmailTriggerLabelAdded   GmailRuleTriggerType = "label_added"
	GmailTriggerStarred      GmailRuleTriggerType = "starred"
)

// GmailRuleActionType represents types of actions for Gmail rules
type GmailRuleActionType string

const (
	GmailActionAutoReply   GmailRuleActionType = "auto_reply"
	GmailActionForward     GmailRuleActionType = "forward"
	GmailActionAddLabel    GmailRuleActionType = "add_label"
	GmailActionRemoveLabel GmailRuleActionType = "remove_label"
	GmailActionMarkRead    GmailRuleActionType = "mark_read"
	GmailActionMarkUnread  GmailRuleActionType = "mark_unread"
	GmailActionArchive     GmailRuleActionType = "archive"
	GmailActionTrash       GmailRuleActionType = "trash"
	GmailActionStar        GmailRuleActionType = "star"
	GmailActionUnstar      GmailRuleActionType = "unstar"
)

// GmailAutomationRule represents a Gmail automation rule
type GmailAutomationRule struct {
	BaseAutomationRule
	Description *string              `db:"description"            json:"description,omitempty"`
	TriggerType GmailRuleTriggerType `db:"trigger_type"           json:"trigger_type"`
	ActionType  GmailRuleActionType  `db:"action_type"            json:"action_type"`
	Priority    int                  `db:"priority"               json:"priority"`
}

// GmailAutomationLog represents a log entry for Gmail automation execution
type GmailAutomationLog struct {
	ID                 int64               `db:"id"                     json:"id"`
	ConnectedAccountID uuid.UUID           `db:"connected_account_id"   json:"connected_account_id"`
	RuleID             *uuid.UUID          `db:"rule_id"                json:"rule_id,omitempty"`
	GmailMessageID     string              `db:"gmail_message_id"       json:"gmail_message_id"`
	GmailThreadID      string              `db:"gmail_thread_id"        json:"gmail_thread_id"`
	Timestamp          time.Time           `db:"timestamp"              json:"timestamp"`
	Status             AutomationLogStatus `db:"status"                 json:"status"`
	TriggerDetails     json.RawMessage     `db:"trigger_details"        json:"trigger_details"`
	ActionDetails      json.RawMessage     `db:"action_details"         json:"action_details"`
	ErrorMessage       string              `db:"error_message"           json:"error_message"`
}

// GmailMessage represents a Gmail message
type GmailMessage struct {
	AccountEntity
	GmailMessageID  string             `db:"gmail_message_id"   json:"gmail_message_id"`
	GmailThreadID   string             `db:"gmail_thread_id"    json:"gmail_thread_id"`
	Subject         *string            `db:"subject"            json:"subject,omitempty"`
	Sender          *string            `db:"sender"             json:"sender,omitempty"`
	Recipients      []string           `db:"recipients"         json:"recipients"`
	CcRecipients    []string           `db:"cc_recipients"      json:"cc_recipients"`
	BccRecipients   []string           `db:"bcc_recipients"     json:"bcc_recipients"`
	Snippet         *string            `db:"snippet"            json:"snippet,omitempty"`
	Status          GmailMessageStatus `db:"status"             json:"status"`
	IsStarred       bool               `db:"is_starred"         json:"is_starred"`
	HasAttachments  bool               `db:"has_attachments"    json:"has_attachments"`
	AttachmentCount int                `db:"attachment_count"   json:"attachment_count"`
	SizeEstimate    *int64             `db:"size_estimate"      json:"size_estimate,omitempty"`
	ReceivedAt      time.Time          `db:"received_at"        json:"received_at"`
	Labels          []string           `db:"labels"             json:"labels"`
	LastSynced      time.Time          `db:"last_synced"        json:"last_synced"`
}

// GmailThread represents a Gmail thread
type GmailThread struct {
	AccountEntity
	GmailThreadID string    `db:"gmail_thread_id"   json:"gmail_thread_id"`
	Subject       *string   `db:"subject"           json:"subject,omitempty"`
	Snippet       *string   `db:"snippet"           json:"snippet,omitempty"`
	MessageCount  int       `db:"message_count"     json:"message_count"`
	HasUnread     bool      `db:"has_unread"        json:"has_unread"`
	LastMessageAt time.Time `db:"last_message_at"   json:"last_message_at"`
	Labels        []string  `db:"labels"            json:"labels"`
	LastSynced    time.Time `db:"last_synced"       json:"last_synced"`
}

// GmailLabel represents a Gmail label
type GmailLabel struct {
	AccountEntity
	GmailLabelID string          `db:"gmail_label_id"    json:"gmail_label_id"`
	Name         string          `db:"name"              json:"name"`
	LabelType    string          `db:"label_type"        json:"label_type"`
	Color        json.RawMessage `db:"color"             json:"color,omitempty"`
	IsHidden     bool            `db:"is_hidden"         json:"is_hidden"`
	MessageCount int             `db:"message_count"     json:"message_count"`
	LastSynced   *time.Time      `db:"last_synced"       json:"last_synced,omitempty"`
}

// GmailDraft represents a Gmail draft
type GmailDraft struct {
	AccountEntity
	GmailDraftID   string   `db:"gmail_draft_id"    json:"gmail_draft_id"`
	Subject        *string  `db:"subject"           json:"subject,omitempty"`
	ToRecipients   []string `db:"to_recipients"     json:"to_recipients"`
	CcRecipients   []string `db:"cc_recipients"     json:"cc_recipients"`
	BccRecipients  []string `db:"bcc_recipients"    json:"bcc_recipients"`
	BodyHTML       *string  `db:"body_html"         json:"body_html,omitempty"`
	BodyPlain      *string  `db:"body_plain"        json:"body_plain,omitempty"`
	HasAttachments bool     `db:"has_attachments"   json:"has_attachments"`
	AttachmentIds  []string `db:"attachment_ids"    json:"attachment_ids"`
}

// GmailContact represents a Gmail contact
type GmailContact struct {
	AccountEntity
	Email         string     `db:"email"             json:"email"`
	DisplayName   *string    `db:"display_name"      json:"display_name,omitempty"`
	PhotoURL      *string    `db:"photo_url"         json:"photo_url,omitempty"`
	IsFrequent    bool       `db:"is_frequent"       json:"is_frequent"`
	LastContacted *time.Time `db:"last_contacted"    json:"last_contacted,omitempty"`
	ContactSource string     `db:"contact_source"    json:"contact_source"`
}

