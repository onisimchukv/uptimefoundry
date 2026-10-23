package migrations

import (
	"embed"
)

// FS contains all embedded SQL migration scripts.
//
//go:embed *.sql
var FS embed.FS
