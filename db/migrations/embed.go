// db/migrations/embed.go

package migrations

import "embed"

//go:embed 000001_initial_schema.up.sql
var InitialSchemaUp string

//go:embed 000001_initial_schema.down.sql
var InitialSchemaDown string

// Optioneel: als je ALLE sql-bestanden als een bestandssysteem wilt:
//go:embed *.sql
var SQLFiles embed.FS
