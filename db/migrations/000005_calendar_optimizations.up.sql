-- Calendar-Specific Database Optimizations
-- Migration: 000005_calendar_optimizations.up.sql

-- Add GIN index for JSON queries on trigger_details (used by calendar event deduplication)
CREATE INDEX IF NOT EXISTS idx_automation_logs_trigger_details_gin ON automation_logs USING GIN (trigger_details);

-- Add functional index for Google Event ID lookups (critical for calendar deduplication)
CREATE INDEX IF NOT EXISTS idx_automation_logs_google_event_id ON automation_logs (
    (trigger_details->>'google_event_id'),
    rule_id,
    status
) WHERE trigger_details->>'google_event_id' IS NOT NULL AND status = 'success';

-- Add GIN index for action_details JSON queries (used by calendar action logging)
CREATE INDEX IF NOT EXISTS idx_automation_logs_action_details_gin ON automation_logs USING GIN (action_details);

-- Add functional index for calendar event summary lookups in trigger_details
CREATE INDEX IF NOT EXISTS idx_automation_logs_trigger_summary ON automation_logs (
    (trigger_details->>'trigger_summary'),
    connected_account_id,
    timestamp DESC
) WHERE trigger_details->>'trigger_summary' IS NOT NULL;

-- Add index for calendar rule trigger conditions (for rules with calendar-specific conditions)
CREATE INDEX IF NOT EXISTS idx_automation_rules_trigger_conditions_gin ON automation_rules USING GIN (trigger_conditions);

-- Add functional index for calendar summary matching rules
CREATE INDEX IF NOT EXISTS idx_automation_rules_summary_equals ON automation_rules (
    connected_account_id,
    (trigger_conditions->>'summary_equals')
) WHERE trigger_conditions->>'summary_equals' IS NOT NULL AND is_active = true;

-- Add GIN index for location-based rule matching
CREATE INDEX IF NOT EXISTS idx_automation_rules_location_contains ON automation_rules (
    connected_account_id
) WHERE is_active = true AND jsonb_array_length(trigger_conditions->'location_contains') > 0;

-- Note: Time-based partial indexes removed due to IMMUTABLE function requirements
-- Recent activity queries can use existing timestamp indexes with appropriate WHERE clauses

-- Add check constraint for calendar-specific validation
ALTER TABLE automation_logs ADD CONSTRAINT chk_automation_logs_calendar_event_id_format
    CHECK (trigger_details->>'google_event_id' IS NULL OR length(trigger_details->>'google_event_id') > 0);

-- Add generated column for calendar event domains (if google_event_id contains domain info)
-- Note: Google Calendar event IDs are typically just IDs, not containing domain info like emails