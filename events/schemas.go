package events

import "embed"

// sqlSchemas embeds the SQL migration files for the events database.
//
//go:embed sqlc/migrations/*.sql
var sqlSchemas embed.FS
