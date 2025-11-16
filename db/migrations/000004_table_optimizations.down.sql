-- Rollback Table Structure and Data Type Optimizations
-- Migration: 000004_table_optimizations.down.sql

-- Remove case-insensitive indexes
DROP INDEX IF EXISTS idx_users_email_lower;
DROP INDEX IF EXISTS idx_connected_accounts_email_lower;
DROP INDEX IF EXISTS idx_gmail_contacts_email_lower;

-- Remove optimized indexes with fill factor
DROP INDEX IF EXISTS idx_connected_accounts_updated_at;
DROP INDEX IF EXISTS idx_gmail_messages_updated_at;
DROP INDEX IF EXISTS idx_gmail_threads_updated_at;

-- Remove partial indexes for status
DROP INDEX IF EXISTS idx_automation_logs_success;
DROP INDEX IF EXISTS idx_automation_logs_failure;
DROP INDEX IF EXISTS idx_gmail_logs_success;
DROP INDEX IF EXISTS idx_gmail_logs_failure;

-- Note: Generated columns for email domains were not added in this migration
-- to avoid conflicts with column type alterations

-- Remove check constraints
ALTER TABLE automation_rules DROP CONSTRAINT IF EXISTS chk_automation_rules_name_not_empty;
ALTER TABLE gmail_automation_rules DROP CONSTRAINT IF EXISTS chk_gmail_rules_name_not_empty;
ALTER TABLE gmail_automation_rules DROP CONSTRAINT IF EXISTS chk_gmail_rules_priority_range;
ALTER TABLE gmail_messages DROP CONSTRAINT IF EXISTS chk_gmail_messages_attachment_count;
ALTER TABLE gmail_messages DROP CONSTRAINT IF EXISTS chk_gmail_messages_size_estimate;
ALTER TABLE gmail_threads DROP CONSTRAINT IF EXISTS chk_gmail_threads_message_count;
ALTER TABLE gmail_drafts DROP CONSTRAINT IF EXISTS chk_gmail_drafts_has_attachments_consistency;

-- Revert varchar fields back to text (safe operation)
ALTER TABLE users ALTER COLUMN name TYPE text;
ALTER TABLE connected_accounts ALTER COLUMN email TYPE text;
ALTER TABLE connected_accounts ALTER COLUMN provider_user_id TYPE text;
ALTER TABLE automation_rules ALTER COLUMN name TYPE text;
ALTER TABLE automation_logs ALTER COLUMN error_message TYPE text;
ALTER TABLE gmail_messages ALTER COLUMN subject TYPE text;
ALTER TABLE gmail_messages ALTER COLUMN sender TYPE text;
ALTER TABLE gmail_messages ALTER COLUMN snippet TYPE text;
ALTER TABLE gmail_threads ALTER COLUMN subject TYPE text;
ALTER TABLE gmail_threads ALTER COLUMN snippet TYPE text;
ALTER TABLE gmail_automation_rules ALTER COLUMN name TYPE text;
ALTER TABLE gmail_automation_rules ALTER COLUMN description TYPE text;
ALTER TABLE gmail_automation_logs ALTER COLUMN error_message TYPE text;
ALTER TABLE gmail_drafts ALTER COLUMN subject TYPE text;
ALTER TABLE gmail_contacts ALTER COLUMN email TYPE text;
ALTER TABLE gmail_contacts ALTER COLUMN display_name TYPE text;
ALTER TABLE gmail_contacts ALTER COLUMN photo_url TYPE text;