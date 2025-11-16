-- Gmail Integration Schema Extension
-- Migration: 000002_gmail_schema.up.sql

-- Add Gmail-specific enums
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'gmail_message_status') THEN
        CREATE TYPE gmail_message_status AS ENUM (
            'unread',
            'read',
            'archived',
            'trashed',
            'spam'
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'gmail_rule_trigger_type') THEN
        CREATE TYPE gmail_rule_trigger_type AS ENUM (
            'new_message',
            'sender_match',
            'subject_match',
            'label_added',
            'starred'
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'gmail_rule_action_type') THEN
        CREATE TYPE gmail_rule_action_type AS ENUM (
            'auto_reply',
            'forward',
            'add_label',
            'remove_label',
            'mark_read',
            'mark_unread',
            'archive',
            'trash',
            'star',
            'unstar'
        );
    END IF;
END$$;

-- Gmail Labels table
CREATE TABLE IF NOT EXISTS gmail_labels (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    connected_account_id uuid NOT NULL REFERENCES connected_accounts(id) ON DELETE CASCADE,
    gmail_label_id text NOT NULL,
    name text NOT NULL,
    label_type text NOT NULL DEFAULT 'user', -- 'user' or 'system'
    color jsonb, -- Store color information
    is_hidden boolean NOT NULL DEFAULT false,
    message_count integer DEFAULT 0,
    last_synced timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (connected_account_id, gmail_label_id)
);
CREATE INDEX IF NOT EXISTS idx_gmail_labels_account_id ON gmail_labels(connected_account_id);

-- Gmail Messages table (metadata only, not full content for storage efficiency)
CREATE TABLE IF NOT EXISTS gmail_messages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    connected_account_id uuid NOT NULL REFERENCES connected_accounts(id) ON DELETE CASCADE,
    gmail_message_id text NOT NULL,
    gmail_thread_id text NOT NULL,
    subject text,
    sender text,
    recipients text[], -- Array of recipient emails
    cc_recipients text[], -- Array of CC recipient emails
    bcc_recipients text[], -- Array of BCC recipient emails
    snippet text, -- Gmail's message snippet
    status gmail_message_status NOT NULL DEFAULT 'unread',
    is_starred boolean NOT NULL DEFAULT false,
    has_attachments boolean NOT NULL DEFAULT false,
    attachment_count integer DEFAULT 0,
    size_estimate bigint, -- Size in bytes
    received_at timestamptz NOT NULL,
    labels text[], -- Array of label IDs
    last_synced timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (connected_account_id, gmail_message_id)
);
CREATE INDEX IF NOT EXISTS idx_gmail_messages_account_id ON gmail_messages(connected_account_id);
CREATE INDEX IF NOT EXISTS idx_gmail_messages_thread_id ON gmail_messages(connected_account_id, gmail_thread_id);
CREATE INDEX IF NOT EXISTS idx_gmail_messages_received_at ON gmail_messages(connected_account_id, received_at DESC);
CREATE INDEX IF NOT EXISTS idx_gmail_messages_status ON gmail_messages(connected_account_id, status);

-- Gmail Threads table
CREATE TABLE IF NOT EXISTS gmail_threads (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    connected_account_id uuid NOT NULL REFERENCES connected_accounts(id) ON DELETE CASCADE,
    gmail_thread_id text NOT NULL,
    subject text,
    snippet text,
    message_count integer NOT NULL DEFAULT 1,
    has_unread boolean NOT NULL DEFAULT true,
    last_message_at timestamptz NOT NULL,
    labels text[], -- Array of label IDs
    last_synced timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (connected_account_id, gmail_thread_id)
);
CREATE INDEX IF NOT EXISTS idx_gmail_threads_account_id ON gmail_threads(connected_account_id);
CREATE INDEX IF NOT EXISTS idx_gmail_threads_last_message_at ON gmail_threads(connected_account_id, last_message_at DESC);

-- Gmail Automation Rules table (separate from calendar rules)
CREATE TABLE IF NOT EXISTS gmail_automation_rules (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    connected_account_id uuid NOT NULL REFERENCES connected_accounts(id) ON DELETE CASCADE,
    name text NOT NULL,
    description text,
    is_active boolean NOT NULL DEFAULT true,
    trigger_type gmail_rule_trigger_type NOT NULL,
    trigger_conditions jsonb NOT NULL, -- Conditions for when to trigger
    action_type gmail_rule_action_type NOT NULL,
    action_params jsonb NOT NULL, -- Parameters for the action
    priority integer NOT NULL DEFAULT 0, -- For rule ordering
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_gmail_rules_account_id ON gmail_automation_rules(connected_account_id) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_gmail_rules_priority ON gmail_automation_rules(connected_account_id, priority DESC);

-- Gmail Automation Logs table
CREATE TABLE IF NOT EXISTS gmail_automation_logs (
    id bigserial PRIMARY KEY,
    connected_account_id uuid NOT NULL REFERENCES connected_accounts(id) ON DELETE CASCADE,
    rule_id uuid REFERENCES gmail_automation_rules(id) ON DELETE SET NULL,
    gmail_message_id text, -- Reference to the message that triggered the rule
    gmail_thread_id text, -- Reference to the thread
    timestamp timestamptz NOT NULL DEFAULT now(),
    status automation_log_status NOT NULL,
    trigger_details jsonb,
    action_details jsonb,
    error_message text
);
CREATE INDEX IF NOT EXISTS idx_gmail_logs_account_id_timestamp ON gmail_automation_logs(connected_account_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_gmail_logs_rule_id ON gmail_automation_logs(rule_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_gmail_logs_message_id ON gmail_automation_logs(connected_account_id, gmail_message_id);

-- Gmail Drafts table
CREATE TABLE IF NOT EXISTS gmail_drafts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    connected_account_id uuid NOT NULL REFERENCES connected_accounts(id) ON DELETE CASCADE,
    gmail_draft_id text NOT NULL,
    subject text,
    to_recipients text[],
    cc_recipients text[],
    bcc_recipients text[],
    body_html text,
    body_plain text,
    has_attachments boolean NOT NULL DEFAULT false,
    attachment_ids text[], -- References to Drive file IDs
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (connected_account_id, gmail_draft_id)
);
CREATE INDEX IF NOT EXISTS idx_gmail_drafts_account_id ON gmail_drafts(connected_account_id);

-- Gmail Contacts/Addresses cache (from People API)
CREATE TABLE IF NOT EXISTS gmail_contacts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    connected_account_id uuid NOT NULL REFERENCES connected_accounts(id) ON DELETE CASCADE,
    email text NOT NULL,
    display_name text,
    photo_url text,
    is_frequent boolean NOT NULL DEFAULT false,
    last_contacted timestamptz,
    contact_source text NOT NULL DEFAULT 'gmail', -- 'gmail', 'people_api', etc.
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (connected_account_id, email)
);
CREATE INDEX IF NOT EXISTS idx_gmail_contacts_account_id ON gmail_contacts(connected_account_id);
CREATE INDEX IF NOT EXISTS idx_gmail_contacts_email ON gmail_contacts(connected_account_id, email);

-- Update connected_accounts to track Gmail sync state
ALTER TABLE connected_accounts
ADD COLUMN IF NOT EXISTS gmail_history_id text,
ADD COLUMN IF NOT EXISTS gmail_last_sync timestamptz,
ADD COLUMN IF NOT EXISTS gmail_sync_enabled boolean NOT NULL DEFAULT true;

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_connected_accounts_gmail_sync ON connected_accounts(gmail_sync_enabled) WHERE gmail_sync_enabled = true;