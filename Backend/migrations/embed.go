package migrations

import "embed"

// FS embeds all SQL migration files so they can be run in any environment
// (standalone binary, Docker container, test harness) without relying on local filesystem paths.
//
//go:embed *.sql
var FS embed.FS

