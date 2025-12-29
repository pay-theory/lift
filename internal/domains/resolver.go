// Package domains provides domain resolution for Lift CLI deployments.
// It computes stage root domains and service domains based on the
// lift.yaml configuration and the target stage.
package domains

import (
	"fmt"

	"github.com/pay-theory/lift/internal/liftconfig"
)

// ValidStages are the fixed stage names supported by Lift CLI.
var ValidStages = []string{"dev", "staging", "live"}

// ResolvedDomains contains all resolved domain information for a stage.
type ResolvedDomains struct {
	BaseDomain      string            `json:"baseDomain,omitempty"`
	StageRootDomain string            `json:"stageRootDomain,omitempty"`
	Services        map[string]string `json:"services,omitempty"`
}

// ValidateStage validates that a stage name is one of dev, staging, or live.
func ValidateStage(stage string) error {
	for _, s := range ValidStages {
		if s == stage {
			return nil
		}
	}
	return &InvalidStageError{Stage: stage}
}

// InvalidStageError is returned when an invalid stage is specified.
type InvalidStageError struct {
	Stage string
}

func (e *InvalidStageError) Error() string {
	return fmt.Sprintf("invalid stage %q: must be one of dev, staging, or live", e.Stage)
}

// DomainsEnabled returns true if the configuration has domains configured.
func DomainsEnabled(cfg *liftconfig.Config) bool {
	return cfg.Domains != nil && cfg.Domains.BaseDomain != ""
}

// Resolve computes the full domain information for a given stage.
// Returns nil if domains are not enabled.
func Resolve(cfg *liftconfig.Config, stage string) (*ResolvedDomains, error) {
	if err := ValidateStage(stage); err != nil {
		return nil, err
	}

	if !DomainsEnabled(cfg) {
		return nil, nil
	}

	baseDomain := cfg.Domains.BaseDomain
	stageRootDomain := resolveStageRootDomain(cfg, stage, baseDomain)

	services := make(map[string]string)
	for name, svc := range cfg.Services {
		if svc != nil && svc.Subdomain != "" {
			services[name] = fmt.Sprintf("%s.%s", svc.Subdomain, stageRootDomain)
		}
	}

	return &ResolvedDomains{
		BaseDomain:      baseDomain,
		StageRootDomain: stageRootDomain,
		Services:        services,
	}, nil
}

// resolveStageRootDomain returns the root domain for a stage.
// It checks for an explicit override in stages.<stage>.root_domain first,
// then falls back to the default derivation from base_domain.
func resolveStageRootDomain(cfg *liftconfig.Config, stage, baseDomain string) string {
	// Check for explicit override
	if cfg.Stages != nil {
		if stageConfig, ok := cfg.Stages[stage]; ok && stageConfig.RootDomain != "" {
			return stageConfig.RootDomain
		}
	}

	// Default derivation
	switch stage {
	case "live":
		return baseDomain
	case "dev":
		return "dev." + baseDomain
	case "staging":
		return "staging." + baseDomain
	default:
		return stage + "." + baseDomain
	}
}
