# Lint Resolution Agents

This document defines specialized agents for resolving lint errors without causing breaking changes. Each agent focuses on specific error categories and follows conservative fixing principles.

## Agent: typecheck-resolver

**Purpose**: Resolves Go typecheck errors and golangci-lint configuration issues that block linting from running.

**Trigger**: Use when `make lint` fails with configuration or compilation errors before showing actual lint issues.

**Capabilities**:
- Fix golangci-lint configuration errors (invalid linters, formatters in linter list)
- Resolve Go compilation errors that prevent static analysis
- Handle missing imports and dependency issues
- Fix basic type errors while preserving behavior

**Key Principles**:
- Configuration fixes first (highest priority - blocks all linting)
- Non-breaking changes only - never alter public APIs
- Conservative approach - minimal changes to resolve errors
- Preserve existing test behavior

**Common Patterns Fixed**:
1. **Config Issues**: Remove formatters from linter lists (`gofumpt`, `goimports`)
2. **Invalid Linters**: Replace deprecated linter names (`gosimple` → available alternatives)
3. **Missing Imports**: Add required import statements
4. **Type Mismatches**: Add safe type conversions
5. **Undefined Variables**: Add proper declarations

**Usage**:
```markdown
I need help resolving typecheck errors that are blocking our lint process. Please run make lint and fix any configuration or compilation issues that prevent the linter from showing actual code quality issues.
```

**Success Criteria**:
- `make lint` runs without configuration errors
- Shows actual lint issues instead of blocking on compilation
- All existing tests still pass
- No changes to public APIs or behavior

---

## Agent: unused-code-cleaner

**Purpose**: Safely removes unused code detected by the `unused` linter.

**Trigger**: Use when lint shows unused variables, functions, types, or imports.

**Capabilities**:
- Remove unused imports safely
- Remove unused private functions and variables
- Clean up unused type definitions
- Handle unused struct fields in internal types
- Remove unused constants and variables

**Key Principles**:
- Only remove private/internal unused code
- Never remove exported symbols (could break external consumers)
- Verify removal doesn't break tests
- Use build tags awareness

**Common Patterns Fixed**:
1. **Unused Imports**: Remove imports that aren't referenced
2. **Unused Variables**: Remove or replace with blank identifier `_`
3. **Unused Functions**: Remove private functions with no callers
4. **Unused Types**: Remove internal type definitions
5. **Unused Fields**: Remove fields from internal structs

**Limitations**:
- Cannot remove exported symbols
- Cannot remove code that might be used via reflection
- Cannot remove code in test files (might be test helpers)

---

## Agent: error-handling-fixer

**Purpose**: Fixes `errcheck` violations by adding proper error handling.

**Trigger**: Use for errcheck linter violations.

**Capabilities**:
- Add error checks to function calls
- Add proper error handling patterns
- Convert ignored errors to logged errors where appropriate
- Add error returns to functions that need them

**Key Principles**:
- Always check errors in production code
- Use logging for errors that can't be returned
- Follow existing error handling patterns in codebase
- Don't change function signatures unless necessary

**Common Patterns Fixed**:
1. **Unchecked Errors**: Add `if err != nil` checks
2. **Deferred Errors**: Add error checks to defer statements
3. **Ignored Returns**: Handle or explicitly ignore error returns
4. **Resource Cleanup**: Ensure proper cleanup on errors

**Safe Patterns**:
```go
// Before
file.Close()

// After  
if err := file.Close(); err != nil {
    log.Printf("Warning: failed to close file: %v", err)
}
```

---

## Agent: duplicate-code-resolver

**Purpose**: Resolves `dupl` linter issues by extracting common code into shared functions.

**Trigger**: Use when dupl linter detects duplicate code blocks.

**Capabilities**:
- Extract duplicate code into shared functions
- Create helper functions for common patterns
- Refactor similar code blocks to use common utilities
- Maintain existing behavior while reducing duplication

**Key Principles**:
- Extract to private helper functions first
- Maintain exact same behavior
- Use meaningful function names
- Consider adding to existing utility modules

**Common Patterns Fixed**:
1. **Duplicate Handlers**: Extract common validation/response patterns
2. **Repeated Checks**: Create shared validation functions  
3. **Similar Logic**: Extract algorithmic similarities
4. **Common Patterns**: Create reusable helper functions

---

## Agent: godot-comment-fixer

**Purpose**: Fixes `godot` linter issues by adding proper punctuation to comments.

**Trigger**: Use for godot linter violations about comment punctuation.

**Capabilities**:
- Add periods to comments that need them
- Fix comment capitalization
- Standardize comment formatting
- Handle multi-line comment blocks

**Key Principles**:
- Only modify comment text, never code
- Follow Go comment conventions
- Preserve meaning and intent
- Handle special cases (URLs, code snippets)

**Common Patterns Fixed**:
1. **Missing Periods**: Add periods to sentence-like comments
2. **Capitalization**: Ensure proper sentence capitalization  
3. **Function Comments**: Standardize function documentation
4. **Package Comments**: Fix package-level documentation

---

## Agent: security-issue-resolver

**Purpose**: Fixes `gosec` security linter issues safely.

**Trigger**: Use for gosec security violations.

**Capabilities**:
- Fix unsafe random number generation
- Add input validation
- Fix path traversal issues
- Handle SQL injection prevention
- Add proper TLS configuration

**Key Principles**:
- Security fixes must not break functionality
- Use secure alternatives for unsafe patterns
- Add validation without changing interfaces
- Document security-related changes

**Common Patterns Fixed**:
1. **Weak Random**: Replace `math/rand` with `crypto/rand` where needed
2. **Path Traversal**: Add path validation
3. **Input Validation**: Add sanitization for user inputs
4. **TLS Config**: Use secure TLS settings

---

## Agent: performance-optimizer

**Purpose**: Fixes performance-related linter issues from `prealloc`, `unconvert`, etc.

**Trigger**: Use for performance linter violations.

**Capabilities**:
- Pre-allocate slices with known capacity
- Remove unnecessary type conversions
- Optimize string operations
- Fix inefficient assignments

**Key Principles**:
- Performance improvements must not change behavior
- Focus on clear performance wins
- Maintain readability
- Don't micro-optimize unclear cases

**Common Patterns Fixed**:
1. **Slice Preallocation**: Add capacity hints to `make([]T, 0, cap)`
2. **Unnecessary Conversions**: Remove redundant type conversions
3. **String Building**: Use `strings.Builder` for concatenation
4. **Inefficient Assignments**: Fix assignment patterns

---

## Agent: revive-internal-fixer

**Purpose**: Fixes **safe** `revive` style violations (668 issues after config filtering) that don't cause breaking changes.

**Trigger**: Use specifically for revive linter violations in internal/private code only.

**Capabilities**:
- Fix internal variable declarations
- Handle context parameter ordering  
- Fix error string formatting (internal errors only)
- Clean up control flow patterns
- Fix unused parameters in private functions

**Key Principles**:
- **NEVER touch exported APIs** (filtered out by config)
- Only fix internal/private code patterns
- Maintain existing behavior exactly
- Focus on safe, non-breaking improvements

**Common Patterns Fixed (INTERNAL ONLY)**:
1. **Context Usage**: Ensure context parameters come first in private functions
2. **Error Strings**: Fix internal error message formatting  
3. **Control Flow**: Simplify if-return patterns
4. **Variable Declarations**: Use consistent declaration styles internally
5. **Unused Parameters**: Clean up private function parameters

**What's EXCLUDED** (via config):
- Exported function/type naming
- Receiver naming changes  
- Package documentation requirements
- Public API comment requirements

---

## Agent: govet-analyzer-fixer

**Purpose**: Resolves `govet` issues (813 issues - 20% of total) including printf, struct tags, and common mistakes.

**Trigger**: Use for govet linter violations.

**Capabilities**:
- Fix printf format string mismatches
- Correct struct tag formatting  
- Fix unreachable code
- Handle shadow variable issues
- Fix composite literal issues
- Resolve method signature problems

**Key Principles**:
- Fix correctness issues that could cause runtime errors
- Maintain exact same behavior
- Focus on high-severity govet warnings first
- Don't change public interfaces

**Common Patterns Fixed**:
1. **Printf Issues**: Match format strings with arguments
2. **Struct Tags**: Fix malformed JSON/DB tags
3. **Shadowing**: Resolve variable shadowing in scopes  
4. **Unreachable Code**: Remove or fix unreachable statements
5. **Composite Literals**: Fix struct/slice literal formatting

---

## Agent: cognitive-complexity-reducer

**Purpose**: Addresses `gocognit` (46 issues) and `gocyclo` (32 issues) complexity violations.

**Trigger**: Use for cognitive and cyclomatic complexity violations.

**Capabilities**:
- Extract complex logic into helper functions
- Simplify nested conditionals  
- Break down large functions
- Use early returns to reduce nesting
- Extract common validation patterns

**Key Principles**:
- Reduce complexity without changing behavior
- Extract to private helper functions
- Maintain exact same logic flow
- Focus on readability improvements

**Common Patterns Fixed**:
1. **Extract Helpers**: Move complex logic to private functions
2. **Early Returns**: Reduce nesting with guard clauses  
3. **Switch Statements**: Replace complex if-else chains
4. **Validation Extraction**: Create shared validation helpers

---

## Agent: misspelling-corrector

**Purpose**: Fixes `misspell` violations (26 issues) in comments and strings.

**Trigger**: Use for misspell linter violations.

**Capabilities**:
- Correct spelling in comments
- Fix typos in string literals (carefully)
- Fix variable/function name typos
- Handle documentation spelling

**Key Principles**:
- Only fix obvious spelling errors
- Be very careful with string literals (could break APIs)
- Focus on comments and documentation first
- Verify corrections don't break external contracts

**Common Patterns Fixed**:
1. **Comment Typos**: Fix spelling in code comments
2. **Documentation**: Correct documentation spelling
3. **Variable Names**: Fix typos in internal variable names
4. **Error Messages**: Carefully fix user-facing error messages

---

## Agent: constant-extractor

**Purpose**: Handles `goconst` violations (45 issues) by extracting repeated string/numeric literals.

**Trigger**: Use for goconst linter violations.

**Capabilities**:
- Extract repeated string literals to constants
- Extract magic numbers to named constants  
- Create constant groups for related values
- Handle API endpoint paths, error messages, etc.

**Key Principles**:
- Only extract clearly repeated values
- Use meaningful constant names
- Group related constants together
- Don't extract single-use values

**Common Patterns Fixed**:
1. **String Literals**: Extract repeated strings like "application/json"
2. **Magic Numbers**: Extract repeated numeric values  
3. **Error Messages**: Extract common error text
4. **API Paths**: Extract repeated URL paths

---

## Agent: modern-go-updater

**Purpose**: Handles `copyloopvar` (1 issue) and other Go version-specific improvements.

**Trigger**: Use for Go version-specific linter violations.

**Capabilities**:
- Remove unnecessary loop variable captures (Go 1.22+)
- Update to modern Go patterns
- Remove obsolete workarounds
- Use newer standard library features

**Key Principles**:
- Only apply if using supported Go version
- Maintain backward compatibility if needed
- Update to cleaner, modern patterns
- Test thoroughly after changes

**Common Patterns Fixed**:
1. **Loop Variables**: Remove `x := x` captures in Go 1.22+
2. **Context**: Use modern context patterns
3. **Errors**: Use modern error handling patterns
4. **Standard Library**: Use newer stdlib features

---

## Agent: security-vulnerability-patcher

**Purpose**: Enhanced security agent for the 43 `gosec` violations found.

**Trigger**: Use for gosec security violations.

**Capabilities**:
- Fix hardcoded credentials detection
- Resolve weak random number usage
- Fix path traversal vulnerabilities  
- Handle SQL injection risks
- Fix insecure HTTP configurations

**Key Principles**:
- Security fixes are high priority
- Don't break existing functionality
- Use secure alternatives
- Document security changes

**Common Patterns Fixed**:
1. **Weak Random**: Replace math/rand with crypto/rand for security contexts
2. **Path Validation**: Add path traversal protection
3. **Input Sanitization**: Add validation for user inputs
4. **TLS Configuration**: Use secure TLS settings

---

## Usage Guidelines

### Priority Order (based on conservative lint analysis of 2,993 issues)

1. **Configuration Issues First**: typecheck-resolver (blocks all other linting) ✅ DONE
2. **Correctness Issues**: govet-analyzer-fixer (940 issues - 31% of total)
3. **Error Handling**: error-handling-fixer (856 issues - 29% of total)  
4. **Safe Style Issues**: revive-internal-fixer (668 issues - 22% of total)
5. **Code Quality**: unused-code-cleaner → duplicate-code-resolver
6. **Security**: security-vulnerability-patcher (43 critical issues)
7. **Performance**: performance-optimizer → constant-extractor
8. **Complexity**: cognitive-complexity-reducer 
9. **Modern Patterns**: modern-go-updater
10. **Polish**: misspelling-corrector → godot-comment-fixer

**Key Change**: Lint config now filters out breaking-change-prone rules, reducing total issues by 25%.

### Best Practices

1. **Batch by impact**: Focus on high-volume, low-risk changes first
2. **Test frequently**: Run tests after each major agent run
3. **Commit incrementally**: Separate commits for each error category
4. **Monitor regressions**: Watch for behavioral changes
5. **Prioritize production code**: Focus on pkg/ before examples/

## Agent Selection by Error Count & Impact

```bash
# CRITICAL: Configuration blocking (fixes 100% of linting)
/use-agent typecheck-resolver

# HIGH SEVERITY: Correctness issues (940 issues - 31% of total)  
/use-agent govet-analyzer-fixer

# HIGH SEVERITY: Error handling (856 issues - 29% of total)
/use-agent error-handling-fixer

# MEDIUM VOLUME: Safe style issues (668 issues - 22% of total)
/use-agent revive-internal-fixer

# MEDIUM: Code cleanliness (205 issues combined)
/use-agent unused-code-cleaner
/use-agent duplicate-code-resolver

# MEDIUM: Security vulnerabilities (43 issues)
/use-agent security-vulnerability-patcher

# MEDIUM: Performance & constants (73 issues combined)
/use-agent performance-optimizer
/use-agent constant-extractor

# LOW: Complexity (78 issues combined)
/use-agent cognitive-complexity-reducer

# LOW: Modern patterns (1 issue)
/use-agent modern-go-updater

# POLISH: Spelling & formatting (26+ issues)
/use-agent misspelling-corrector
/use-agent godot-comment-fixer
```

## Expected Impact

**After conservative config changes (25% reduction already achieved):**
- **govet-analyzer-fixer**: Reduce errors by ~31% (940 → 0)  
- **error-handling-fixer**: Reduce errors by ~29% (856 → 0)
- **revive-internal-fixer**: Reduce errors by ~22% (668 → 0)
- **Combined effect**: Address ~82% of remaining lint issues with top 3 agents
- **Total potential**: From 3,986 → 2,993 → ~500 issues (87% total reduction)

## Focus Areas by File Pattern

- **Production code** (`pkg/`): All agents, prioritize security & correctness
- **Examples** (`examples/`): Focus on style consistency, skip minor issues  
- **Tests** (`*_test.go`): Lighter touch, focus on obvious errors only
- **Enterprise features** (`pkg/testing/enterprise/`): High unused code, prioritize cleanup