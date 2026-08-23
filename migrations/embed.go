package migrations

import "embed"

// FS contains the database migrations applied at server startup.
//
//go:embed *.sql
var FS embed.FS
