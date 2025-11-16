-- Rollback Connected Accounts Critical Index Optimization
-- Migration: 000006_connected_accounts_optimization.down.sql

-- Remove critical indexes for connected accounts
DROP INDEX IF EXISTS idx_connected_accounts_status;
DROP INDEX IF EXISTS idx_connected_accounts_user_status;
DROP INDEX IF EXISTS idx_connected_accounts_provider_status;