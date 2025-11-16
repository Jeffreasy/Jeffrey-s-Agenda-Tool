// db/migrations/embed.go

package migrations

import "embed"

// InitialSchemaUp contains the up migration for the initial schema.
//
//go:embed 000001_initial_schema.up.sql
var InitialSchemaUp string

// InitialSchemaDown contains the down migration for the initial schema.
//
//go:embed 000001_initial_schema.down.sql
var InitialSchemaDown string

// GmailSchemaUp contains the up migration for the Gmail schema.
//
//go:embed 000002_gmail_schema.up.sql
var GmailSchemaUp string

// GmailSchemaDown contains the down migration for the Gmail schema.
//
//go:embed 000002_gmail_schema.down.sql
var GmailSchemaDown string

// OptimizationIndexesUp contains the up migration for optimization indexes.
//
//go:embed 000003_optimization_indexes.up.sql
var OptimizationIndexesUp string

// OptimizationIndexesDown contains the down migration for optimization indexes.
//
//go:embed 000003_optimization_indexes.down.sql
var OptimizationIndexesDown string

// TableOptimizationsUp contains the up migration for table optimizations.
//
//go:embed 000004_table_optimizations.up.sql
var TableOptimizationsUp string

// TableOptimizationsDown contains the down migration for table optimizations.
//
//go:embed 000004_table_optimizations.down.sql
var TableOptimizationsDown string

// CalendarOptimizationsUp contains the up migration for calendar optimizations.
//
//go:embed 000005_calendar_optimizations.up.sql
var CalendarOptimizationsUp string

// CalendarOptimizationsDown contains the down migration for calendar optimizations.
//
//go:embed 000005_calendar_optimizations.down.sql
var CalendarOptimizationsDown string

// ConnectedAccountsOptimizationUp contains the up migration for connected accounts optimization.
//
//go:embed 000006_connected_accounts_optimization.up.sql
var ConnectedAccountsOptimizationUp string

// ConnectedAccountsOptimizationDown contains the down migration for connected accounts optimization.
//
//go:embed 000006_connected_accounts_optimization.down.sql
var ConnectedAccountsOptimizationDown string

// SQLFiles optionally contains all SQL files as a filesystem:
//
//go:embed *.sql
var SQLFiles embed.FS
