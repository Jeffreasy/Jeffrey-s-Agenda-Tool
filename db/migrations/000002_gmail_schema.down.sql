-- Gmail Integration Schema Extension - Down Migration
-- Migration: 000002_gmail_schema.down.sql

-- Remove Gmail-related columns from connected_accounts
ALTER TABLE connected_accounts
DROP COLUMN IF EXISTS gmail_history_id,
DROP COLUMN IF EXISTS gmail_last_sync,
DROP COLUMN IF EXISTS gmail_sync_enabled;

-- Drop Gmail tables in reverse order (due to foreign key constraints)
DROP TABLE IF EXISTS gmail_automation_logs;
DROP TABLE IF EXISTS gmail_drafts;
DROP TABLE IF EXISTS gmail_contacts;
DROP TABLE IF EXISTS gmail_messages;
DROP TABLE IF EXISTS gmail_threads;
DROP TABLE IF EXISTS gmail_labels;
DROP TABLE IF EXISTS gmail_automation_rules;

-- Drop Gmail-specific enums
DROP TYPE IF EXISTS gmail_message_status;
DROP TYPE IF EXISTS gmail_rule_trigger_type;
DROP TYPE IF EXISTS gmail_rule_action_type;