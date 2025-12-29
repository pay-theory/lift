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

// MicroserviceFS embeds all template files for the microservice template.
//
//go:embed microservice/*
//go:embed microservice/**/*
//go:embed microservice/.gitignore.tmpl
//go:embed microservice/.github/workflows/*.tmpl
var MicroserviceFS embed.FS

// EventDrivenFS embeds all template files for the event-driven template.
//
//go:embed event-driven/*
//go:embed event-driven/**/*
//go:embed event-driven/.gitignore.tmpl
//go:embed event-driven/.github/workflows/*.tmpl
var EventDrivenFS embed.FS

// MerchantAppFS embeds all template files for the merchant-app template.
//
//go:embed merchant-app/*
//go:embed merchant-app/**/*
//go:embed merchant-app/.gitignore.tmpl
//go:embed merchant-app/.github/workflows/*.tmpl
var MerchantAppFS embed.FS

// MicroservicePTFS embeds PT-mode devops files (buildspec.yml, shell/*, cdk/*, lift.yaml).
// These are generated instead of .github/workflows/* when --pt flag is used.
//
//go:embed microservice-pt/*
//go:embed microservice-pt/shell/*
//go:embed microservice-pt/cdk/*
var MicroservicePTFS embed.FS

// EventDrivenPTFS embeds PT-mode devops files (buildspec.yml, shell/*, cdk/*, lift.yaml).
// These are generated instead of .github/workflows/* when --pt flag is used.
//
//go:embed event-driven-pt/*
//go:embed event-driven-pt/shell/*
//go:embed event-driven-pt/cdk/*
var EventDrivenPTFS embed.FS

// MerchantAppPTFS embeds PT-mode devops files (buildspec.yml, shell/*, cdk/*, lift.yaml).
// These are generated instead of .github/workflows/* when --pt flag is used.
//
//go:embed merchant-app-pt/*
//go:embed merchant-app-pt/shell/*
//go:embed merchant-app-pt/cdk/*
var MerchantAppPTFS embed.FS
