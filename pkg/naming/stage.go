package naming

import "strings"

// NormalizeStage maps common stage aliases to canonical Pay Theory conventions.
//
// Canonical stages:
//   - lab   (dev)
//   - study (sandbox)
//   - live  (prod)
func NormalizeStage(stage string) string {
	s := strings.ToLower(strings.TrimSpace(stage))
	switch s {
	case "dev", "development", "lab":
		return "lab"
	case "sandbox", "study":
		return "study"
	case "prod", "production", "live":
		return "live"
	default:
		return s
	}
}
