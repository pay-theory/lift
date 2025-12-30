// Package liftstate provides state management for Lift CLI deployments.
// It reads and writes .lift/state/<stage>.json files to implement
// the domain immutability contract.
package liftstate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
)

// StateDir is the directory (relative to project root) where state files are stored.
const StateDir = ".lift/state"

// CurrentVersion is the state file schema version.
const CurrentVersion = 1

// StageState represents the persisted state for a deployed stage.
type StageState struct {
	Services        map[string]ServiceState `json:"services,omitempty"`
	Stage           string                  `json:"stage"`
	BaseDomain      string                  `json:"baseDomain,omitempty"`
	StageRootDomain string                  `json:"stageRootDomain,omitempty"`
	Version         int                     `json:"version"`
}

// ServiceState represents the persisted state for a service within a stage.
type ServiceState struct {
	Subdomain string `json:"subdomain"`
	Domain    string `json:"domain"`
}

// Load reads the state file for a given stage from the project root.
// Returns nil, nil if the state file does not exist (stage not yet deployed).
func Load(projectRoot, stage string) (*StageState, error) {
	statePath := StatePath(projectRoot, stage)

	data, err := os.ReadFile(statePath) //nolint:gosec // statePath is derived from the project root and stage name
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state StageState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	return &state, nil
}

// Save writes the state file for a given stage.
func Save(projectRoot string, state *StageState) error {
	if state.Version == 0 {
		state.Version = CurrentVersion
	}

	stateDir := filepath.Join(projectRoot, StateDir)
	if err := os.MkdirAll(stateDir, 0750); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize state: %w", err)
	}

	statePath := StatePath(projectRoot, state.Stage)
	if err := os.WriteFile(statePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// Remove deletes the state file for a given stage.
// Returns nil if the file does not exist.
func Remove(projectRoot, stage string) error {
	statePath := StatePath(projectRoot, stage)
	if err := os.Remove(statePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove state file: %w", err)
	}
	return nil
}

// StatePath returns the full path to the state file for a stage.
func StatePath(projectRoot, stage string) string {
	return filepath.Join(projectRoot, StateDir, stage+".json")
}

// DomainMismatch describes a detected mismatch between current and desired domains.
type DomainMismatch struct {
	Field   string
	Current string
	Desired string
}

// DomainLockError is returned when the resolved domains differ from the locked state.
type DomainLockError struct {
	Stage      string
	Mismatches []DomainMismatch
}

func (e *DomainLockError) Error() string {
	return fmt.Sprintf(
		"domain configuration changed for stage %q\n\n"+
			"The domains for this stage were locked after a previous successful deploy.\n"+
			"Run `lift down --stage %s` before re-deploying with new domains.\n\n"+
			"Detected changes:\n%s",
		e.Stage, e.Stage, e.formatMismatches())
}

func (e *DomainLockError) formatMismatches() string {
	var s string
	for _, m := range e.Mismatches {
		s += fmt.Sprintf("  • %s: %q → %q\n", m.Field, m.Current, m.Desired)
	}
	return s
}

// CheckDomainLock compares the current state against the desired domains.
// Returns a DomainLockError if there are mismatches.
func CheckDomainLock(current *StageState, stage, baseDomain, stageRootDomain string, services map[string]string) error {
	if current == nil {
		return nil // No existing state, nothing to compare
	}

	var mismatches []DomainMismatch

	if current.BaseDomain != baseDomain {
		mismatches = append(mismatches, DomainMismatch{
			Field:   "baseDomain",
			Current: current.BaseDomain,
			Desired: baseDomain,
		})
	}

	if current.StageRootDomain != stageRootDomain {
		mismatches = append(mismatches, DomainMismatch{
			Field:   "stageRootDomain",
			Current: current.StageRootDomain,
			Desired: stageRootDomain,
		})
	}

	// Compare services
	currentServices := make(map[string]string)
	for name, svc := range current.Services {
		currentServices[name] = svc.Domain
	}

	if !reflect.DeepEqual(currentServices, services) {
		// Find specific mismatches
		for name, domain := range services {
			if current, ok := currentServices[name]; ok {
				if current != domain {
					mismatches = append(mismatches, DomainMismatch{
						Field:   fmt.Sprintf("serviceDomain.%s", name),
						Current: current,
						Desired: domain,
					})
				}
			} else {
				mismatches = append(mismatches, DomainMismatch{
					Field:   fmt.Sprintf("serviceDomain.%s", name),
					Current: "(not set)",
					Desired: domain,
				})
			}
		}
		for name := range currentServices {
			if _, ok := services[name]; !ok {
				mismatches = append(mismatches, DomainMismatch{
					Field:   fmt.Sprintf("serviceDomain.%s", name),
					Current: currentServices[name],
					Desired: "(removed)",
				})
			}
		}
	}

	if len(mismatches) > 0 {
		return &DomainLockError{Stage: stage, Mismatches: mismatches}
	}

	return nil
}
