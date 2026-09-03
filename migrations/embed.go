package migrations

import "embed"

// Files contains the forward-only SQLite migrations in filename order.
//
//go:embed *.sql
var Files embed.FS
