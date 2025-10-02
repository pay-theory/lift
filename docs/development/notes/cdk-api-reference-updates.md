# CDK API Reference Documentation Updates

## Overview

This document outlines the discrepancies found between the current CDK API reference documentation (`docs/cdk-api-reference.md`) and the actual implementation in the codebase. The review was conducted on the `godoc-enhancement` branch.

## Executive Summary

The documentation is generally well-structured and covers the core concepts correctly, but requires significant updates to align with the current implementation. Major issues include incorrect construct names, mismatched property names, missing constructs, and outdated file references.

## Critical Issues Requiring Immediate Attention

### 1. Construct Name Corrections

#### DynamORMTable → LiftTable
- **Current Documentation**: References `DynamORMTable` construct
- **Actual Implementation**: Named `LiftTable` in `pkg/cdk/constructs/dynamodb.go`
- **Action Required**: Update all references throughout the documentation

#### File Path Corrections
- **Current Documentation**: References `pkg/cdk/constructs/dynamorm_table.go`
- **Actual Implementation**: Located in `pkg/cdk/constructs/dynamodb.go`
- **Action Required**: Update file path references

### 2. Property Name Misalignments

#### RateLimitedFunction Properties
| Documentation | Implementation | Status |
|---------------|----------------|---------|
| `LimitType` | `RateLimitType` | ❌ Needs Update |
| `RequestLimit` | `Limit` | ❌ Needs Update |
| `WindowSeconds` | `WindowSeconds` | ✅ Correct |
| `TableName` | `TableName` | ✅ Correct |

#### IdempotentFunction Properties
| Documentation | Implementation | Status |
|---------------|----------------|---------|
| `KeySource` | `IdempotentKeyExtractor` | ❌ Needs Update |
| `KeyPath` | `KeyField` | ❌ Needs Update |
| `TTLSeconds` | `TTLSeconds` | ✅ Correct |
| `TableName` | `TableName` | ✅ Correct |

#### LiftFunction Properties
| Documentation | Implementation | Status |
|---------------|----------------|---------|
| `EnableDeadLetterQueue` | Not implemented | ❌ Remove from docs |
| `LogRetentionDays` | Not implemented | ❌ Remove from docs |
| `EnableDynamORM` | `EnableDynamORM` | ✅ Correct |
| `DynamORMTableName` | `DynamORMTableName` | ✅ Correct |

### 3. Missing Constructs

#### EnhancedMonitoring
- **Status**: Implemented but not documented
- **Location**: `pkg/cdk/constructs/monitoring_enhanced.go`
- **Action Required**: Add complete documentation section

#### EnhancedSecurity
- **Status**: Implemented but not documented
- **Location**: `pkg/cdk/constructs/security_enhanced.go`
- **Action Required**: Add complete documentation section

#### MicroserviceComplete Pattern
- **Status**: Implemented but not documented
- **Location**: `pkg/cdk/patterns/microservice_complete.go`
- **Action Required**: Add complete documentation section

## Detailed Recommendations by Section

### Core Constructs Section

#### LiftFunction
```markdown
# Current Issues
- Remove non-existent properties: `EnableDeadLetterQueue`, `LogRetentionDays`
- Update method references to match actual implementation
- Fix example code to use correct property names

# Recommended Updates
- Update constructor signature to match actual implementation
- Remove references to methods that don't exist
- Update environment variable documentation
```

#### LiftAPI
```markdown
# Current Status: ✅ Mostly Accurate
# Minor Updates Needed
- Verify all property names match `APICommonProps` interface
- Update method signatures to match implementation
- Ensure example code compiles
```

#### LiftTable (Currently Documented as DynamORMTable)
```markdown
# Major Updates Required
- Rename construct from `DynamORMTable` to `LiftTable`
- Update file reference from `dynamorm_table.go` to `dynamodb.go`
- Align all property names with actual implementation
- Update method signatures
- Fix example code
```

### Middleware Constructs Section

#### RateLimitedFunction
```markdown
# Property Updates Required
- `LimitType` → `RateLimitType`
- `RequestLimit` → `Limit`
- Update enum values to match implementation
- Fix example code property names
```

#### IdempotentFunction
```markdown
# Property Updates Required
- `KeySource` → `IdempotentKeyExtractor`
- `KeyPath` → `KeyField`
- Update enum values to match implementation
- Fix example code property names
```

#### SecureFunction
```markdown
# Current Status: ✅ Accurate
# Minor Updates
- Verify all property names match implementation
- Update example code if needed
```

#### MonitoredFunction
```markdown
# Current Status: ✅ Accurate
# Minor Updates
- Verify all property names match implementation
- Update example code if needed
```

### Pattern Constructs Section

#### BasicAPI
```markdown
# Current Status: ✅ Accurate
# No major changes needed
```

#### SecureAPI
```markdown
# Significant Updates Required
- Update property structure to match implementation
- Fix property names in examples
- Update method signatures
- Verify WAF configuration examples
```

#### LiftApp
```markdown
# Current Status: ✅ Mostly Accurate
# Minor Updates
- Verify property names match implementation
- Update example code if needed
```

### Missing Sections to Add

#### EnhancedMonitoring
```markdown
# New Section Required
## EnhancedMonitoring

**File**: `pkg/cdk/constructs/monitoring_enhanced.go`  
**Type**: Comprehensive Monitoring Construct  
**Lines**: 1-655

### Constructor
```go
func NewEnhancedMonitoring(scope constructs.Construct, id *string, props *EnhancedMonitoringProps) *EnhancedMonitoring
```

### Properties (EnhancedMonitoringProps)
[Include complete property documentation]

### Methods
[Include all available methods]

### Example
[Include usage example]
```

#### EnhancedSecurity
```markdown
# New Section Required
## EnhancedSecurity

**File**: `pkg/cdk/constructs/security_enhanced.go`  
**Type**: Comprehensive Security Construct  
**Lines**: 1-757

### Constructor
```go
func NewEnhancedSecurity(scope constructs.Construct, id *string, props *EnhancedSecurityProps) *EnhancedSecurity
```

### Properties (EnhancedSecurityProps)
[Include complete property documentation]

### Methods
[Include all available methods]

### Example
[Include usage example]
```

#### MicroserviceComplete Pattern
```markdown
# New Section Required
## MicroserviceComplete

**File**: `pkg/cdk/patterns/microservice_complete.go`  
**Type**: Complete ECS-based Microservice Pattern  
**Lines**: 1-903

### Constructor
```go
func NewMicroserviceComplete(scope constructs.Construct, id *string, props *MicroserviceCompleteProps) *MicroserviceComplete
```

### Properties (MicroserviceCompleteProps)
[Include complete property documentation]

### Methods
[Include all available methods]

### Example
[Include usage example]
```

## Implementation Priority

### High Priority (Critical)
1. Fix construct name `DynamORMTable` → `LiftTable`
2. Update file path references
3. Fix property name mismatches in middleware constructs
4. Remove non-existent properties from LiftFunction

### Medium Priority (Important)
1. Add missing EnhancedMonitoring documentation
2. Add missing EnhancedSecurity documentation
3. Update SecureAPI property structure
4. Fix example code throughout

### Low Priority (Nice to Have)
1. Add MicroserviceComplete pattern documentation
2. Improve code examples
3. Add more detailed method descriptions
4. Cross-reference related constructs

## Testing Recommendations

After implementing these updates:

1. **Compile Test**: Ensure all example code compiles
2. **Integration Test**: Verify examples work with actual CDK deployment
3. **Cross-Reference**: Check that all file paths and construct names are correct
4. **Review**: Have team members review updated documentation

## File Structure Updates

The documentation should reflect the actual file structure:

```
pkg/cdk/
├── constructs/
│   ├── lambda.go              # LiftFunction
│   ├── api.go                 # LiftAPI
│   ├── dynamodb.go            # LiftTable (not dynamorm_table.go)
│   ├── ratelimited.go         # RateLimitedFunction
│   ├── idempotent.go          # IdempotentFunction
│   ├── secure.go              # SecureFunction
│   ├── monitored.go           # MonitoredFunction
│   ├── monitoring_enhanced.go # EnhancedMonitoring
│   └── security_enhanced.go   # EnhancedSecurity
├── patterns/
│   ├── basic_api.go           # BasicAPI
│   ├── secure_api.go          # SecureAPI
│   ├── lift_app.go            # LiftApp
│   └── microservice_complete.go # MicroserviceComplete
└── stacks/
    ├── microservice.go        # MicroserviceStack
    ├── multi_tenant_saas.go   # MultiTenantSaaSStack
    └── event_driven.go        # EventDrivenStack
```

## Conclusion

The CDK API reference documentation requires significant updates to align with the current implementation. The core concepts and structure are sound, but the specific API details need correction. Implementing these recommendations will ensure the documentation accurately reflects the codebase and provides reliable guidance for developers using the Lift CDK constructs.

Priority should be given to fixing the critical issues first, particularly the construct name corrections and property name alignments, as these affect the usability of the documentation for developers.
