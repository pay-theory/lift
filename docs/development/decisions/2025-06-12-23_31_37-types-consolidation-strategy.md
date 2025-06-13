# Types Consolidation Strategy - 2025-06-12-23_31_37

## Decision Context
The enterprise testing framework has extensive duplicate type declarations across multiple files causing compilation errors. We need a systematic approach to consolidate types.

## Current Issues
- 100+ compilation errors due to duplicate type declarations
- Types scattered across multiple files: chaos.go, chaos_engineering.go, contract_testing.go, contract_infrastructure.go, contracts.go, gdpr_components.go, gdpr_infrastructure.go, infrastructure.go
- Inconsistent type definitions between files
- Maintenance burden due to scattered definitions

## Decision
**Consolidate all types into `pkg/testing/enterprise/types.go` as the single source of truth.**

## Implementation Strategy

### Phase 1: Core Type Consolidation ✅ COMPLETE
- Enhanced `types.go` with missing core types
- Added ChaosConfig, ChaosEngineeringConfig, ContractReportExporter, ConsentHistory, etc.

### Phase 2: Remove Duplicates from Contract Files 🔄 IN PROGRESS
Priority order:
1. **contracts.go** - Remove InteractionRequest, InteractionResponse, ContractValidator, ContractTestResult
2. **contract_testing.go** - Remove ContractReportExporter struct, getDefaultContractReportTemplates function
3. **contract_infrastructure.go** - Keep interface, remove duplicate function

### Phase 3: Remove Duplicates from GDPR Files
1. **gdpr_components.go** - Remove ConsentRecord duplicate
2. **gdpr_infrastructure.go** - Remove ConsentHistory duplicate

### Phase 4: Remove Duplicates from Chaos Files
1. **chaos_engineering.go** - Update to use ChaosEngineeringConfig from types.go
2. **chaos.go** - Keep simple ChaosConfig for basic chaos testing

### Phase 5: Clean Infrastructure Files
1. **infrastructure.go** - Remove all duplicate types, use aliases to types.go

## Type Naming Conventions

### Chaos Engineering
- `ChaosConfig` - Simple configuration for basic chaos testing (chaos.go)
- `ChaosEngineeringConfig` - Comprehensive configuration for enterprise chaos engineering (types.go)

### Contract Testing
- `ContractReportExporter` - Interface for exporting (types.go)
- `ContractReportExporterImpl` - Concrete implementation (types.go)

### GDPR/Privacy
- `ConsentRecord` - Core consent record structure (types.go)
- `ConsentHistory` - History of consent changes (types.go)

## Benefits
1. **Single Source of Truth**: All types defined in one place
2. **Reduced Maintenance**: Changes only need to be made in one location
3. **Better Organization**: Clear separation between types and implementation
4. **Improved Readability**: Easier to understand the type hierarchy
5. **Faster Compilation**: No more duplicate declaration errors

## Implementation Status
- ✅ Enhanced types.go with core missing types
- 🔄 Removing duplicates from contract files
- ⏳ Removing duplicates from GDPR files
- ⏳ Removing duplicates from chaos files
- ⏳ Cleaning infrastructure files

## Next Steps
1. Remove InteractionRequest, InteractionResponse duplicates from contracts.go
2. Remove ContractReportExporter struct from contract_testing.go
3. Remove ConsentRecord duplicate from gdpr_components.go
4. Update chaos_engineering.go to use ChaosEngineeringConfig
5. Test compilation after each major change 