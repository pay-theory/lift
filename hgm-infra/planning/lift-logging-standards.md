# Lift: Logging & Operational Standards (Rubric v0.1.0)

This document defines the minimum acceptable logging and operational practices for the Lift serverless framework, enforced via deterministic static analysis checks in the rubric verifier.

**Rubric Version**: v0.1.0
**Last Updated**: 2026-01-13

## Purpose

Logging standards prevent common operational security and reliability issues:
- **Log injection attacks**: Unsanitized user input in logs can forge log entries
- **Sensitive data exposure**: Raw logging can leak credentials, PII, or CHD
- **Operational opacity**: Inconsistent logging makes debugging and observability difficult
- **Performance issues**: Excessive or blocking log calls can degrade Lambda cold starts

## Enforced Standards (COM-6)

### 1. Structured Logging Required

**Rule**: All production logging must use the project's structured logger interfaces.

**Rationale**:
- Structured logs are machine-parseable for observability platforms
- Centralized logger packages can enforce sanitization and redaction policies
- Logger abstractions enable testing and log level control

**Implementation**:
- Use `pkg/logger.LiftLogger` (global singleton) or `pkg/observability.StructuredLogger` interfaces
- Use `pkg/observability/zap` for Zap-based structured logging
- Use CloudWatch logger wrappers when available

**Verification**: Verifier checks that logging goes through approved logger packages.

### 2. No Direct stdlib `log` Package Usage

**Rule**: Lambda runtime code must NOT import or use the Go standard library `log` package directly.

**Scope**: Enforced for Lambda runtime packages (pkg/lift, pkg/middleware, pkg/logger, pkg/services, etc.).

**Excluded from enforcement**:
- CLI tools (pkg/cli) - user-facing console output
- Development server (pkg/dev) - diagnostic output
- Testing frameworks (pkg/testing) - test result output
- CDK infrastructure (pkg/cdk) - synth-time warnings

**Rationale**:
- `log.Printf` family writes to stderr without structure or sanitization
- stdlib log has no log levels, filtering, or observability integration
- Raw log calls bypass centralized redaction/sanitization
- Legacy log calls create inconsistent observability data

**Exceptions**:
- Test files (`*_test.go`) may use `log` for debugging
- CLI tools (pkg/cli/**) - user-facing console output
- Development server (pkg/dev/**) - diagnostic output
- Testing frameworks (pkg/testing/**) - test result output
- CDK infrastructure (pkg/cdk/**) - synth-time warnings

**Verification**: Verifier greps for `"log"` imports in Lambda runtime Go files.

### 3. No Direct `fmt.Print*` Usage in Lambda Runtime Code

**Rule**: Lambda runtime code must NOT use `fmt.Printf`, `fmt.Println`, `fmt.Print`, or `println` builtin.

**Scope**: Enforced for Lambda runtime packages (pkg/lift, pkg/middleware, pkg/logger, pkg/services, etc.).

**Excluded from enforcement**:
- CLI tools (pkg/cli) - user-facing console output
- Development server (pkg/dev) - diagnostic output
- Testing frameworks (pkg/testing) - test result output
- CDK infrastructure (pkg/cdk) - synth-time warnings/output
- Commented code (excluded from detection)

**Temporary Allowlist** (technical debt to be resolved):
- `pkg/lift/connection_store_dynamodb.go` - contains 1 warning `fmt.Printf` on line 151; should be migrated to structured logger

**Rationale**:
- These functions write to stdout/stderr without sanitization
- Lambda environments capture stdout/stderr as logs, creating injection risks
- Print statements bypass log level controls
- Print calls are not structured and cannot be queried in observability tools

**Verification**: Verifier greps for `fmt.Print*`, `println` in Lambda runtime Go files (excludes commented lines and allowlisted files).

### 4. Sanitization for User-Controlled Data

**Rule**: User-controlled data (request parameters, headers, external inputs) logged in structured fields must be sanitized to prevent log injection and data leakage.

**Rationale**:
- Newlines and control characters in user input can forge log entries
- PII, CHD (cardholder data), credentials must be redacted before logging
- Compliance requirements (PCI-DSS, SOC2) mandate sensitive data redaction

**Implementation Guidance**:
- Use `pkg/logger.SanitizeLogString()` for string values
- Use `pkg/utils/sanitization.SanitizeForLog()` for maps/complex values
- Use `pkg/logger.SanitizedStringMap()` wrapper for map-based structured fields
- Never log raw `context.Context` values (may contain sensitive auth tokens)

**Verification**:
- Documented requirement (static enforcement would create false positives)
- Code review and runtime testing should validate sanitization usage
- Future: Consider adding detection for common anti-patterns (e.g., logging `ctx` directly)

### 5. No Blocking I/O in Log Paths

**Rule**: Log calls should not perform blocking network I/O or expensive computation.

**Rationale**:
- Lambda cold start time is critical for performance
- Blocking log calls can delay request processing
- Logs should be fire-and-forget with buffering

**Implementation**:
- Use async log sinks when available
- Avoid synchronous HTTP calls in logger initialization
- Buffer logs and flush asynchronously when possible

**Verification**: Design guidance (not statically enforced in COM-6)

## Verifier Implementation

The COM-6 verifier check (`logging_operational_standards_check` in `hgm-infra/verifiers/hgm-verify-rubric.sh`) enforces standards 1-3 via static analysis:

1. **Enumerates tracked .go files in Lambda runtime scope**:
   - Includes: pkg/** (except excluded dirs), cmd/**, internal/**
   - Excludes: test files, vendor, hgm-infra/, pkg/cli/, pkg/dev/, pkg/testing/, pkg/cdk/
2. **Checks for disallowed imports**: Detects `import "log"` in Lambda runtime files
3. **Checks for disallowed calls**: Detects `fmt.Print*`, `println` in Lambda runtime files (excludes commented code)
4. **Reports violations**: Outputs file paths and line numbers for remediation

**Scope Rationale**:
- CLI tools (pkg/cli/**): Legitimate user-facing console output
- Development server (pkg/dev/**): Diagnostic and debugging output
- Testing frameworks (pkg/testing/**): Test result and load test output
- CDK infrastructure (pkg/cdk/**): Stack synthesis warnings (not runtime logs)

**Fail-Closed Behavior**:
- Returns exit code 2 (BLOCKED) if git or required tools are unavailable
- Returns exit code 1 (FAIL) if violations are found in Lambda runtime code
- Returns exit code 0 (PASS) if no violations detected

**Evidence**: Results written to `hgm-infra/evidence/COM-6-output.log`

## Allowed Logging Patterns

### Good: Structured Logging with Sanitization
```go
import "github.com/pay-theory/lift/pkg/logger"

// Log with structured fields
logger.LiftLogger.Info("Processing request",
    "user_id", logger.SanitizeLogString(userID),
    "endpoint", endpoint,
)

// Log maps with sanitization
sanitizedParams := logger.SanitizedStringMap(requestParams)
logger.LiftLogger.Info("Request parameters", "params", sanitizedParams)
```

### Good: Using Context-Aware Logger
```go
import "github.com/pay-theory/lift/pkg/lift"

func Handler(ctx *lift.Context) error {
    ctx.Logger.Info("Handler invoked", "path", ctx.Request.Path)
    // Context logger is pre-configured and sanitized
    return nil
}
```

### Bad: Direct stdlib log Usage
```go
import "log"  // ❌ VIOLATION: Direct log import

func processData(data string) {
    log.Printf("Processing: %s", data)  // ❌ Unsanitized, unstructured
}
```

### Bad: Direct fmt.Print Usage
```go
import "fmt"

func handler(userInput string) {
    fmt.Printf("Received: %s\n", userInput)  // ❌ VIOLATION: stdout, unsanitized
    println("Debug:", userInput)              // ❌ VIOLATION: builtin println
}
```

## Migration Guide (If Violations Found)

If the COM-6 verifier detects violations:

1. **Replace stdlib log calls**:
   ```go
   // Before
   log.Printf("Error: %v", err)

   // After
   logger.LiftLogger.Error("Operation failed", "error", err)
   ```

2. **Replace fmt.Print* calls**:
   ```go
   // Before
   fmt.Println("Debug:", value)

   // After
   logger.LiftLogger.Debug("Debug value", "value", value)
   ```

3. **Add sanitization for user input**:
   ```go
   // Before
   logger.LiftLogger.Info("User action", "input", userInput)

   // After
   logger.LiftLogger.Info("User action",
       "input", logger.SanitizeLogString(userInput))
   ```

## Enforcement History

### 2026-01-13: Initial Enforcement (COM-6)
- Implemented deterministic verifier check for standards 1-3
- Scope refined to Lambda runtime code (excludes CLI, dev server, testing, CDK)
- Rationale: CLI/dev/testing tools require console output; CDK warnings run during synth not runtime
- Temporary allowlist: `pkg/lift/connection_store_dynamodb.go` (1 warning Printf - line 151)
  - Requires source code change (blocked in governance-only step)
  - Documented as technical debt for future remediation
- Baseline scan: Lambda runtime code clean except allowlisted file (no violations)
- Status: COM-6 = PASS

## Future Enhancements

Potential future logging standards (not yet enforced):
- Static detection of `context.Context` values in log calls
- Log level appropriateness checks (no Info in tight loops)
- Structured field naming conventions
- Maximum log message size limits
- Required fields for specific log types (trace IDs, user IDs)

## References

- Primary rubric: `hgm-infra/planning/lift-10of10-rubric.md` (COM-6)
- Evidence plan: `hgm-infra/planning/lift-evidence-plan.md`
- Verifier entrypoint: `hgm-infra/verifiers/hgm-verify-rubric.sh`
- Logger package: `pkg/logger/logger.go`
- Sanitization utilities: `pkg/utils/sanitization/`
