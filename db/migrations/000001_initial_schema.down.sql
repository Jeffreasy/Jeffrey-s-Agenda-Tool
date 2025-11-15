-- Rollback initial schema migration

DROP INDEX IF EXISTS idx_automation_logs_rule_id;
DROP INDEX IF EXISTS idx_automation_logs_account_id_timestamp;
DROP TABLE IF EXISTS automation_logs;

DROP INDEX IF EXISTS idx_automation_rules_account_id;
DROP TABLE IF EXISTS automation_rules;

DROP INDEX IF EXISTS idx_connected_accounts_user_id;
DROP TABLE IF EXISTS connected_accounts;

DROP TABLE IF EXISTS users;

DROP TYPE IF EXISTS automation_log_status;
DROP TYPE IF EXISTS account_status;
DROP TYPE IF EXISTS provider_type;