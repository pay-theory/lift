// Package templates provides embedded template files for Lift project scaffolding.
package templates

import "embed"

// BasicAPIFS embeds all template files for the basic-api template.
//
//go:embed basic-api/*
//go:embed basic-api/**/*
//go:embed basic-api/.gitignore.tmpl
//go:embed basic-api/.github/workflows/*.tmpl
var BasicAPIFS embed.FS
