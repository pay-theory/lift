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

// BasicAPIPTFS embeds PT-mode devops files (buildspec.yml, shell/*, cdk/*, lift.yaml).
// These are generated instead of .github/workflows/* when --pt flag is used.
//
//go:embed basic-api-pt/*
//go:embed basic-api-pt/shell/*
//go:embed basic-api-pt/cdk/*
var BasicAPIPTFS embed.FS
