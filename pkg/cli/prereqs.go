// Package cli provides the Lift CLI command implementations.
package cli

import (
	"errors"
	"fmt"
	"os/exec"
)

// LookPathFunc is a function type that checks if a binary is found in PATH.
// This enables dependency injection for testing.
type LookPathFunc func(name string) (string, error)

// DefaultLookPath wraps exec.LookPath with the LookPathFunc signature.
func DefaultLookPath(name string) (string, error) {
	return exec.LookPath(name)
}

// PrereqError represents a missing prerequisite with actionable remediation.
type PrereqError struct {
	Binary  string
	Message string
}

func (e *PrereqError) Error() string {
	return e.Message
}

// CheckGo verifies that the "go" binary is available.
// It returns an actionable error if Go is not found.
func CheckGo(lookPath LookPathFunc) error {
	if lookPath == nil {
		lookPath = DefaultLookPath
	}

	_, err := lookPath("go")
	if err != nil {
		return &PrereqError{
			Binary: "go",
			Message: `go not found in PATH

Go is required to build Lambda functions.

Install Go from: https://go.dev/doc/install

After installation, ensure 'go' is in your PATH:
  export PATH=$PATH:/usr/local/go/bin`,
		}
	}
	return nil
}

// CheckCDK verifies that the "cdk" binary is available.
// It returns an actionable error if CDK is not found.
func CheckCDK(lookPath LookPathFunc) error {
	if lookPath == nil {
		lookPath = DefaultLookPath
	}

	_, err := lookPath("cdk")
	if err != nil {
		return &PrereqError{
			Binary: "cdk",
			Message: `cdk not found in PATH

AWS CDK is required to deploy infrastructure.

Install CDK via npm:
  npm install -g aws-cdk

Or via npx (no global install):
  npx aws-cdk --version

For more information: https://docs.aws.amazon.com/cdk/v2/guide/getting_started.html`,
		}
	}
	return nil
}

// CheckNode verifies that the "node" binary is available.
// It returns an actionable error if Node.js is not found.
// Note: This is optional; CDK has Node.js as a dependency.
func CheckNode(lookPath LookPathFunc) error {
	if lookPath == nil {
		lookPath = DefaultLookPath
	}

	_, err := lookPath("node")
	if err != nil {
		return &PrereqError{
			Binary: "node",
			Message: `node not found in PATH

Node.js is required for AWS CDK.

Install Node.js from: https://nodejs.org/
Or via nvm: https://github.com/nvm-sh/nvm

Recommended: Node.js 18 LTS or later`,
		}
	}
	return nil
}

// CheckPrereqs validates multiple prerequisites and returns the first error found.
func CheckPrereqs(lookPath LookPathFunc, binaries ...string) error {
	if lookPath == nil {
		lookPath = DefaultLookPath
	}

	var errs []error

	for _, bin := range binaries {
		switch bin {
		case "go":
			if err := CheckGo(lookPath); err != nil {
				errs = append(errs, err)
			}
		case "cdk":
			if err := CheckCDK(lookPath); err != nil {
				errs = append(errs, err)
			}
		case "node":
			if err := CheckNode(lookPath); err != nil {
				errs = append(errs, err)
			}
		default:
			_, err := lookPath(bin)
			if err != nil {
				errs = append(errs, &PrereqError{
					Binary:  bin,
					Message: fmt.Sprintf("%s not found in PATH", bin),
				})
			}
		}
	}

	if len(errs) > 0 {
		return errs[0] // Return first error for simplicity
	}
	return nil
}

// IsPrereqError checks if the error is a PrereqError.
func IsPrereqError(err error) bool {
	var prereqErr *PrereqError
	return errors.As(err, &prereqErr)
}
