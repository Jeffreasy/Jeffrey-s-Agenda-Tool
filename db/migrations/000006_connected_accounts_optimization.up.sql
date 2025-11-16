-- Connected Accounts Critical Index Optimization
-- Migration: 000006_connected_accounts_optimization.up.sql

-- Add critical index for active accounts lookup (used by worker)
CREATE INDEX IF NOT EXISTS idx_connected_accounts_status ON connected_accounts(status) WHERE status = 'active';

-- Add composite index for ownership verification queries
CREATE INDEX IF NOT EXISTS idx_connected_accounts_user_status ON connected_accounts(user_id, status);

-- Add index for provider-based queries
CREATE INDEX IF NOT EXISTS idx_connected_accounts_provider_status ON connected_accounts(provider, status) WHERE status = 'active';