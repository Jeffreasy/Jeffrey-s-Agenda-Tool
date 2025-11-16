-- Database Performance Optimizations
-- Migration: 000003_optimization_indexes.up.sql

-- Add GIN indexes for array fields to support efficient label-based queries
CREATE INDEX IF NOT EXISTS idx_gmail_messages_labels ON gmail_messages USING GIN (labels);
CREATE INDEX IF NOT EXISTS idx_gmail_threads_labels ON gmail_threads USING GIN (labels);

-- Add indexes for search fields to improve query performance
CREATE INDEX IF NOT EXISTS idx_gmail_messages_sender ON gmail_messages (connected_account_id, sender) WHERE sender IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_gmail_messages_subject ON gmail_messages (connected_account_id, subject) WHERE subject IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_gmail_contacts_display_name ON gmail_contacts (connected_account_id, display_name) WHERE display_name IS NOT NULL;

-- Add partial index for starred messages filtering
CREATE INDEX IF NOT EXISTS idx_gmail_messages_starred ON gmail_messages (connected_account_id, received_at DESC) WHERE is_starred = true;

-- Add index for Gmail message ID lookups (separate from unique constraint for faster point queries)
CREATE INDEX IF NOT EXISTS idx_gmail_messages_message_id ON gmail_messages (gmail_message_id);

-- Add index for Gmail thread ID lookups
CREATE INDEX IF NOT EXISTS idx_gmail_threads_thread_id ON gmail_threads (gmail_thread_id);

-- Add index for Gmail automation logs by message ID for faster lookups
CREATE INDEX IF NOT EXISTS idx_gmail_logs_message_id_only ON gmail_automation_logs (gmail_message_id);

-- Add composite index for Gmail contacts frequent contacts
CREATE INDEX IF NOT EXISTS idx_gmail_contacts_frequent ON gmail_contacts (connected_account_id, last_contacted DESC) WHERE is_frequent = true;

-- Add index for Gmail labels by name for label management
CREATE INDEX IF NOT EXISTS idx_gmail_labels_name ON gmail_labels (connected_account_id, name);

-- Add index for Gmail messages by attachment status
CREATE INDEX IF NOT EXISTS idx_gmail_messages_attachments ON gmail_messages (connected_account_id, has_attachments) WHERE has_attachments = true;