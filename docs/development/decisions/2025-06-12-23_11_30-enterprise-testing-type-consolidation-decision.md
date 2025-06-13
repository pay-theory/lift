# Enterprise Testing Package Type Consolidation Decision

**Date:** 2025-06-12-23_11_30  
**Status:** In Progress  
**Decision Maker:** AI Assistant  

## Context

The enterprise testing package has numerous duplicate type declarations across multiple files causing compilation errors. The main issues are:

1. **Severity Types:** Declared in multiple files with same names
2. **Report Types:** Duplicated across different modules  
3. **Contract Types:** Redeclared in multiple contract-related files
4. **Status Types:** Multiple definitions of test status enums

## Decision

**Approach:** Create a shared `types.go` file to consolidate common types and use type aliases where appropriate.

### Implementation Strategy

1. **Created `pkg/testing/enterprise/types.go`** with consolidated type definitions
2. **Use type aliases** for domain-specific variations (e.g., `type FaultSeverity = Severity`)
3. **Remove duplicate declarations** systematically from each file
4. **Update struct field references** to use consolidated types

### Types Consolidated

- `Severity` (with constants: CriticalSeverity, HighSeverity, MediumSeverity, LowSeverity, InfoSeverity)
- `TestStatus` (with constants: PendingStatus, RunningStatus, PassedStatus, FailedStatus, SkippedStatus)
- `ReportFormat` (JSONFormat, PDFFormat, HTMLFormat, CSVFormat)
- `SecurityCategory` (shared between compliance frameworks)
- `EvidenceType` (various evidence types)
- `TestResult`, `TestReport`, `TestSummary` (common test structures)
- `ReportTemplate`, `ReportSection`, `ReportChart`, `ReportExporter` (reporting structures)
- `ContractTest`, `ServiceContract`, `ContractInteraction` (contract testing structures)
- `ComplianceReport`, `ControlResult`, `ComplianceTestResult` (compliance structures)
- `Evidence`, `ConsentRecord` (data structures)
- `AlertingConfig`, `AlertChannelConfig`, `AlertingSystem` (alerting structures)

### Remaining Work

The following files still need duplicate type removal:

1. **chaos_engineering.go** - Remove duplicate MonitorType, AlertSeverity, ExecutorConfig, SchedulerConfig, ChaosConfig
2. **contract_infrastructure.go** - Remove duplicate severity and config types
3. **gdpr_infrastructure.go** - Remove duplicate severity and report types  
4. **infrastructure.go** - Remove duplicate severity and format types
5. **gdpr.go** - Update SecurityCategory reference
6. **patterns.go** - Remove duplicate TestResult and TestReport
7. **contracts.go** - Remove duplicate contract types
8. **contract_testing.go** - Remove duplicate contract types

### Benefits

- **Eliminates compilation errors** from duplicate declarations
- **Improves maintainability** with single source of truth for types
- **Enables consistent typing** across the enterprise testing package
- **Reduces code duplication** and potential inconsistencies

### Risks

- **Breaking changes** for any external consumers of these types
- **Potential type mismatches** during transition period
- **Increased complexity** in understanding type relationships

## Next Steps

1. Complete systematic removal of duplicate types from remaining files
2. Update all struct field references to use consolidated types
3. Test compilation to ensure all errors are resolved
4. Update any external references if needed
5. Document the new type structure for future development

## Notes

- Used type aliases (e.g., `type FaultSeverity = Severity`) to maintain domain-specific naming while using shared underlying types
- Made `ControlResult.Category` use `interface{}` to accommodate different category types (SOC2Category, GDPRCategory, SecurityCategory)
- Renamed conflicting constants (e.g., `InfoSeverity` → `InfoObservationSeverity` in chaos engineering context) 