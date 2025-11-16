-- Rollback Database Performance Optimizations
-- Migration: 000003_optimization_indexes.down.sql

-- Drop GIN indexes for array fields
DROP INDEX IF EXISTS idx_gmail_messages_labels;
DROP INDEX IF EXISTS idx_gmail_threads_labels;

-- Drop indexes for search fields
DROP INDEX IF EXISTS idx_gmail_messages_sender;
DROP INDEX IF EXISTS idx_gmail_messages_subject;
DROP INDEX IF EXISTS idx_gmail_contacts_display_name;

-- Drop partial index for starred messages
DROP INDEX IF EXISTS idx_gmail_messages_starred;

-- Drop additional lookup indexes
DROP INDEX IF EXISTS idx_gmail_messages_message_id;
DROP INDEX IF EXISTS idx_gmail_threads_thread_id;
DROP INDEX IF EXISTS idx_gmail_logs_message_id_only;
DROP INDEX IF EXISTS idx_gmail_contacts_frequent;
DROP INDEX IF EXISTS idx_gmail_labels_name;
DROP INDEX IF EXISTS idx_gmail_messages_attachments;