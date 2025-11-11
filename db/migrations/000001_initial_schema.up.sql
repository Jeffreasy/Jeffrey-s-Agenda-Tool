-- Plak dit in 000001_initial_schema.up.sql
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TYPE provider_type AS ENUM (
    'google',
    'microsoft'
);

CREATE TYPE account_status AS ENUM (
    'active',
    'revoked',
    'error',
    'paused'
);

CREATE TYPE log_status AS ENUM (
    'pending',
    'success',
    'failure',
    'skipped'
);

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text NOT NULL UNIQUE CHECK (email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$'),
    name text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE connected_accounts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider provider_type NOT NULL,
    email text NOT NULL,
    provider_user_id text NOT NULL,
    access_token bytea NOT NULL,
    refresh_token bytea,
    token_expiry timestamptz NOT NULL,
    scopes text[],
    status account_status NOT NULL DEFAULT 'active',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, provider, provider_user_id)
);
CREATE INDEX idx_connected_accounts_user_id ON connected_accounts(user_id);

CREATE TABLE automation_rules (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    connected_account_id uuid NOT NULL REFERENCES connected_accounts(id) ON DELETE CASCADE,
    name text NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    trigger_conditions jsonb NOT NULL,
    action_params jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_automation_rules_account_id ON automation_rules(connected_account_id) WHERE is_active = true;

CREATE TABLE automation_logs (
    id bigserial PRIMARY KEY,
    connected_account_id uuid NOT NULL REFERENCES connected_accounts(id) ON DELETE CASCADE,
    rule_id uuid REFERENCES automation_rules(id) ON DELETE SET NULL,
    timestamp timestamptz NOT NULL DEFAULT now(),
    status log_status NOT NULL,
    trigger_details jsonb,
    action_details jsonb,
    error_message text
);
CREATE INDEX idx_automation_logs_account_id_timestamp ON automation_logs(connected_account_id, timestamp DESC);
CREATE INDEX idx_automation_logs_rule_id ON automation_logs(rule_id, timestamp DESC);