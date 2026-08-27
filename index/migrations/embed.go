package migrations

import "embed"

// FS contains the standalone Index database migrations.
//
//go:embed *.sql
var FS embed.FS
