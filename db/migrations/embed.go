// db/migrations/embed.go

package migrations

import "embed"

//go:embed 000001_initial_schema.up.sql
var InitialSchemaUp string

//go:embed 000001_initial_schema.down.sql
var InitialSchemaDown string

// Gmail schema migrations
//go:embed 000002_gmail_schema.up.sql
var GmailSchemaUp string

//go:embed 000002_gmail_schema.down.sql
var GmailSchemaDown string

// Performance optimization migrations
//go:embed 000003_optimization_indexes.up.sql
var OptimizationIndexesUp string

//go:embed 000003_optimization_indexes.down.sql
var OptimizationIndexesDown string

// Table structure optimization migrations
//go:embed 000004_table_optimizations.up.sql
var TableOptimizationsUp string

//go:embed 000004_table_optimizations.down.sql
var TableOptimizationsDown string

// Calendar-specific optimization migrations
//go:embed 000005_calendar_optimizations.up.sql
var CalendarOptimizationsUp string

//go:embed 000005_calendar_optimizations.down.sql
var CalendarOptimizationsDown string

// Connected accounts optimization migrations
//go:embed 000006_connected_accounts_optimization.up.sql
var ConnectedAccountsOptimizationUp string

//go:embed 000006_connected_accounts_optimization.down.sql
var ConnectedAccountsOptimizationDown string

// Optioneel: als je ALLE sql-bestanden als een bestandssysteem wilt:
//go:embed *.sql
var SQLFiles embed.FS

