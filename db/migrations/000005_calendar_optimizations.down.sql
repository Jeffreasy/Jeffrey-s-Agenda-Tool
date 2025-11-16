-- Rollback Calendar-Specific Database Optimizations
-- Migration: 000005_calendar_optimizations.down.sql

-- Remove GIN indexes for JSON queries
DROP INDEX IF EXISTS idx_automation_logs_trigger_details_gin;
DROP INDEX IF EXISTS idx_automation_logs_action_details_gin;
DROP INDEX IF EXISTS idx_automation_rules_trigger_conditions_gin;

-- Remove functional indexes for Google Event ID lookups
DROP INDEX IF EXISTS idx_automation_logs_google_event_id;
DROP INDEX IF EXISTS idx_automation_logs_trigger_summary;
DROP INDEX IF EXISTS idx_automation_rules_summary_equals;
DROP INDEX IF EXISTS idx_automation_rules_location_contains;

-- Remove check constraint
ALTER TABLE automation_logs DROP CONSTRAINT IF EXISTS chk_automation_logs_calendar_event_id_format;