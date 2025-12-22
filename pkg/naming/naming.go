package naming

import (
	"os"
	"strings"
)

const (
	// EnvAppName is the application/repo name used for predictable resource naming.
	EnvAppName = "APP_NAME"
	// EnvStage is the deployment stage (preferred: lab, study, live).
	EnvStage = "STAGE"
	// EnvTenant is the optional tenant identifier (Lift commonly uses PARTNER).
	EnvTenant = "PARTNER"
)

// Context holds the minimal inputs required to deterministically name resources.
type Context struct {
	AppName string
	Stage   string
	Tenant  string
}

// FromEnv builds a Context from environment variables.
func FromEnv() Context {
	return Context{
		AppName: strings.TrimSpace(os.Getenv(EnvAppName)),
		Stage:   strings.TrimSpace(os.Getenv(EnvStage)),
		Tenant:  strings.TrimSpace(os.Getenv(EnvTenant)),
	}
}

// IsComplete returns true when the context has the minimum required fields.
func (c Context) IsComplete() bool {
	return c.AppName != "" && c.Stage != ""
}

// Normalize returns a copy with stage aliases normalized to canonical values.
func (c Context) Normalize() Context {
	normalized := c
	normalized.Stage = NormalizeStage(c.Stage)
	return normalized
}

// BaseName returns "<app>-<tenant>-<stage>" or "<app>-<stage>" when tenant is empty.
func (c Context) BaseName() string {
	c = c.Normalize()
	return joinHyphen(nonEmpty(
		strings.TrimSpace(c.AppName),
		strings.TrimSpace(c.Tenant),
		strings.TrimSpace(c.Stage),
	)...)
}

// ResourceName returns "<app>-<tenant>-<resource>-<stage>" or "<app>-<resource>-<stage>".
// Stage is always the final segment to keep names stable and sortable.
func (c Context) ResourceName(resource string) string {
	c = c.Normalize()
	return joinHyphen(nonEmpty(
		strings.TrimSpace(c.AppName),
		strings.TrimSpace(c.Tenant),
		strings.TrimSpace(resource),
		strings.TrimSpace(c.Stage),
	)...)
}

// ResourceNameFromEnv returns a resource name derived from env vars, or ("", false) if incomplete.
func ResourceNameFromEnv(resource string) (string, bool) {
	ctx := FromEnv()
	if !ctx.IsComplete() {
		return "", false
	}
	return ctx.ResourceName(resource), true
}

func nonEmpty(parts ...string) []string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func joinHyphen(parts ...string) string {
	return strings.Join(parts, "-")
}
