# Chaos Engineering Test Fixes - 2025-06-12-23_31_37

## Problem Summary
The chaos engineering test files are failing because they're using the old `ChaosConfig` type but expecting `ChaosEngineeringConfig` fields. This is due to our types consolidation where we now have:

- `ChaosConfig` - Simple configuration for basic chaos testing (4 fields)
- `ChaosEngineeringConfig` - Comprehensive configuration for enterprise chaos engineering (9+ fields)

## Root Cause
1. **Type Mismatch**: Test files use `ChaosConfig` but set fields that only exist in `ChaosEngineeringConfig`
2. **Function Signature**: `NewChaosEngineeringFramework()` expects `*ChaosEngineeringConfig` but tests pass `*ChaosConfig`
3. **Struct Field**: `ChaosEngineeringFramework.config` is now `*ChaosEngineeringConfig` but tests expect `*ChaosConfig`

## Required Fixes

### 1. Update All Test ChaosConfig References
Replace all instances of `&ChaosConfig{` with `&ChaosEngineeringConfig{` in:
- `chaos_engineering_test.go` (multiple locations)
- Any other test files using chaos engineering

### 2. Fix ObservationSeverity Type Issues
Replace `InfoSeverity` with `InfoObservationSeverity` in test files:
```go
// OLD
Severity: InfoSeverity,

// NEW  
Severity: InfoObservationSeverity,
```

### 3. Remove Duplicate Types from chaos_engineering.go
The following types are duplicated between `chaos_engineering.go` and `types.go`:
- `MonitorType`
- `AlertSeverity` 
- `ChaosReportType`
- `NotificationConfig`
- `NotificationChannel`
- `NotificationRule`
- `SecurityConfig`

**Solution**: Remove these from `chaos_engineering.go` and use the ones from `types.go`

### 4. Fix Missing Type Definitions
Some types referenced in tests are not defined:
- `ChaosExperimentResult` 
- `ExperimentStatusRunning`
- `FaultTypeLatency`, `FaultTypeError`, etc.
- `errors.NewValidationError`

## Implementation Strategy

### Phase 1: Fix Test Files ✅ IN PROGRESS
```bash
# Replace all ChaosConfig with ChaosEngineeringConfig in test files
find pkg/testing/enterprise -name "*_test.go" -exec sed -i 's/&ChaosConfig{/&ChaosEngineeringConfig{/g' {} \;

# Fix severity type issues
find pkg/testing/enterprise -name "*_test.go" -exec sed -i 's/InfoSeverity/InfoObservationSeverity/g' {} \;
```

### Phase 2: Clean Up Duplicate Types
Remove duplicate type declarations from `chaos_engineering.go`:
- Lines 311-320: `MonitorType` and constants
- Lines 368-375: `AlertSeverity` and constants  
- Lines 448-456: `ChaosReportType` and constants
- Lines 476-483: `NotificationConfig`
- Lines 484-490: `NotificationChannel`
- Lines 491-499: `NotificationRule`
- Lines 500-508: `SecurityConfig`

### Phase 3: Add Missing Types
Add missing type definitions to appropriate files:
- `ChaosExperimentResult` → `chaos_engineering.go`
- Experiment status constants → `chaos_engineering.go`
- Fault type constants → `chaos_engineering.go`

## Expected Outcome
After these fixes:
- ✅ All test files will compile successfully
- ✅ No duplicate type declaration errors
- ✅ Consistent type usage across the framework
- ✅ Proper separation between simple and enterprise chaos configs

## Files to Modify
1. `pkg/testing/enterprise/chaos_engineering_test.go` - Update all ChaosConfig references
2. `pkg/testing/enterprise/chaos_engineering.go` - Remove duplicate types, add missing types
3. `pkg/testing/enterprise/chaos_kubernetes.go` - Fix missing type references
4. Any other test files using chaos engineering types

## Validation
```bash
# Test compilation
go build ./pkg/testing/enterprise

# Run tests
go test ./pkg/testing/enterprise -v
``` 