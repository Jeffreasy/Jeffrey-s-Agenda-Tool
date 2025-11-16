-- Table Structure and Data Type Optimizations
-- Migration: 000004_table_optimizations.up.sql

-- Add length limits to text fields for better performance and data integrity
ALTER TABLE users ALTER COLUMN name TYPE varchar(255);
ALTER TABLE connected_accounts ALTER COLUMN email TYPE varchar(255);
ALTER TABLE connected_accounts ALTER COLUMN provider_user_id TYPE varchar(255);
ALTER TABLE automation_rules ALTER COLUMN name TYPE varchar(255);
ALTER TABLE automation_logs ALTER COLUMN error_message TYPE varchar(1000);
ALTER TABLE gmail_messages ALTER COLUMN subject TYPE varchar(500);
ALTER TABLE gmail_messages ALTER COLUMN sender TYPE varchar(255);
ALTER TABLE gmail_messages ALTER COLUMN snippet TYPE varchar(2000);
ALTER TABLE gmail_threads ALTER COLUMN subject TYPE varchar(500);
ALTER TABLE gmail_threads ALTER COLUMN snippet TYPE varchar(2000);
ALTER TABLE gmail_automation_rules ALTER COLUMN name TYPE varchar(255);
ALTER TABLE gmail_automation_rules ALTER COLUMN description TYPE varchar(500);
ALTER TABLE gmail_automation_logs ALTER COLUMN error_message TYPE varchar(1000);
ALTER TABLE gmail_drafts ALTER COLUMN subject TYPE varchar(500);
ALTER TABLE gmail_contacts ALTER COLUMN email TYPE varchar(255);
ALTER TABLE gmail_contacts ALTER COLUMN display_name TYPE varchar(255);
ALTER TABLE gmail_contacts ALTER COLUMN photo_url TYPE varchar(500);

-- Add check constraints for data integrity
ALTER TABLE automation_rules ADD CONSTRAINT chk_automation_rules_name_not_empty CHECK (length(trim(name)) > 0);
ALTER TABLE gmail_automation_rules ADD CONSTRAINT chk_gmail_rules_name_not_empty CHECK (length(trim(name)) > 0);
ALTER TABLE gmail_automation_rules ADD CONSTRAINT chk_gmail_rules_priority_range CHECK (priority >= 0 AND priority <= 100);
ALTER TABLE gmail_messages ADD CONSTRAINT chk_gmail_messages_attachment_count CHECK (attachment_count >= 0);
ALTER TABLE gmail_messages ADD CONSTRAINT chk_gmail_messages_size_estimate CHECK (size_estimate IS NULL OR size_estimate >= 0);
ALTER TABLE gmail_threads ADD CONSTRAINT chk_gmail_threads_message_count CHECK (message_count >= 0);
ALTER TABLE gmail_drafts ADD CONSTRAINT chk_gmail_drafts_has_attachments_consistency CHECK (
    (has_attachments = true AND attachment_ids IS NOT NULL AND array_length(attachment_ids, 1) > 0) OR
    (has_attachments = false AND (attachment_ids IS NULL OR array_length(attachment_ids, 1) = 0))
);

-- Note: Generated columns for email domains are handled separately to avoid type alteration conflicts
-- They will be added in a separate migration if needed

-- Add partial indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_automation_logs_success ON automation_logs(connected_account_id, timestamp DESC) WHERE status = 'success';
CREATE INDEX IF NOT EXISTS idx_automation_logs_failure ON automation_logs(connected_account_id, timestamp DESC) WHERE status = 'failure';
CREATE INDEX IF NOT EXISTS idx_gmail_logs_success ON gmail_automation_logs(connected_account_id, timestamp DESC) WHERE status = 'success';
CREATE INDEX IF NOT EXISTS idx_gmail_logs_failure ON gmail_automation_logs(connected_account_id, timestamp DESC) WHERE status = 'failure';

-- Optimize indexes with fill factor for frequently updated tables
-- (Lower fill factor leaves room for updates without page splits)
CREATE INDEX IF NOT EXISTS idx_connected_accounts_updated_at ON connected_accounts(updated_at DESC) WITH (fillfactor = 70);
CREATE INDEX IF NOT EXISTS idx_gmail_messages_updated_at ON gmail_messages(connected_account_id, updated_at DESC) WITH (fillfactor = 70);
CREATE INDEX IF NOT EXISTS idx_gmail_threads_updated_at ON gmail_threads(connected_account_id, updated_at DESC) WITH (fillfactor = 70);

-- Add case-insensitive indexes for email searches
CREATE INDEX IF NOT EXISTS idx_users_email_lower ON users(lower(email));
CREATE INDEX IF NOT EXISTS idx_connected_accounts_email_lower ON connected_accounts(lower(email));
CREATE INDEX IF NOT EXISTS idx_gmail_contacts_email_lower ON gmail_contacts(connected_account_id, lower(email));