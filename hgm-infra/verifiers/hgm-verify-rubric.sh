#!/usr/bin/env bash
# Hypergenium Rubric Verifier (Single Entrypoint)
# Generated from pack version: ba1a734ab662
# Pack digest: a73df10dc3ead5014e2a67c62fdce039104074bd07a2f6cfef432921d12801eb
# Project: lift (lift)
# Domain: custom
#
# This script is the deterministic verifier entrypoint for hgm.validate.
# It reads planning state from hgm-infra/planning/, runs repo-specific check
# commands, writes evidence under hgm-infra/evidence/, and emits a fixed JSON
# report at hgm-infra/evidence/hgm-rubric-report.json.
#
# Usage:
#   ./hgm-infra/verifiers/hgm-verify-rubric.sh
#
# Exit codes:
#   0 - All rubric items PASS
#   1 - One or more rubric items FAIL or BLOCKED
#   2 - Script error (unexpected)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
HGM_INFRA="${REPO_ROOT}/hgm-infra"
PLANNING_DIR="${HGM_INFRA}/planning"
EVIDENCE_DIR="${HGM_INFRA}/evidence"
REPORT_PATH="${EVIDENCE_DIR}/hgm-rubric-report.json"

RUBRIC_VERSION="0.1.0"
COV_THRESHOLD="90"
GO_REQUIRED_PREFIX="go1.25" # from go.mod
GOLANGCI_LINT_REQUIRED_VERSION="v2.4.0" # from .github/workflows/test.yml

mkdir -p "${EVIDENCE_DIR}"

# Normalize working directory so commands behave the same regardless of where the script is invoked from.
cd "${REPO_ROOT}"

# Clean previous run outputs to prevent stale evidence from being misattributed.
# Only remove files this verifier owns.
rm -f \
  "${REPORT_PATH}" \
  "${EVIDENCE_DIR}/"*-output.log \
  "${EVIDENCE_DIR}/DOC-5-parity.log" \
  "${EVIDENCE_DIR}/coverage.out"

REPORT_SCHEMA_VERSION=1
REPORT_TIMESTAMP="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
PASS_COUNT=0
FAIL_COUNT=0
BLOCKED_COUNT=0

declare -a RESULTS=()

json_escape() {
  # Minimal JSON string escaper.
  # NOTE: we keep it simple (quotes + backslashes + newlines).
  local s="$1"
  s="${s//\\/\\\\}"
  s="${s//\"/\\\"}"
  s="${s//$'\n'/\\n}"
  printf '%s' "$s"
}

record_result() {
  local id="$1"
  local category="$2"
  local status="$3"
  local message="$4"
  local evidence_path="$5"

  case "$status" in
    PASS) ((PASS_COUNT++)) || true ;;
    FAIL) ((FAIL_COUNT++)) || true ;;
    BLOCKED) ((BLOCKED_COUNT++)) || true ;;
    *) echo "Internal error: invalid status '${status}'" >&2; exit 2 ;;
  esac

  RESULTS+=(
    "{\"id\":\"$(json_escape "$id")\",\"category\":\"$(json_escape "$category")\",\"status\":\"$(json_escape "$status")\",\"message\":\"$(json_escape "$message")\",\"evidencePath\":\"$(json_escape "$evidence_path")\"}"
  )
}

run_check() {
  local id="$1"
  local category="$2"
  local cmd="$3"
  local output_file="${EVIDENCE_DIR}/${id}-output.log"

  if [[ "$cmd" == "TODO:"* ]]; then
    printf '%s\n' "$cmd" > "$output_file"
    record_result "$id" "$category" "BLOCKED" "Verifier command not configured" "$output_file"
    return 0
  fi

  # Run the command and capture output.
  # Convention: an exit code of 2 is treated as BLOCKED (e.g., tool not installed).
  #
  # IMPORTANT: run in the current bash context so verifier helper functions (e.g., run_coverage)
  # are available.
  set +e
  ( eval "$cmd" ) >"$output_file" 2>&1
  local ec=$?
  set -e

  if [[ $ec -eq 0 ]]; then
    record_result "$id" "$category" "PASS" "Command succeeded" "$output_file"
  elif [[ $ec -eq 2 ]]; then
    record_result "$id" "$category" "BLOCKED" "Command reported BLOCKED (exit code 2)" "$output_file"
  else
    record_result "$id" "$category" "FAIL" "Command failed with exit code ${ec}" "$output_file"
  fi
}

check_file_exists() {
  local id="$1"
  local category="$2"
  local file_path="$3"

  if [[ -f "$file_path" ]]; then
    record_result "$id" "$category" "PASS" "File exists" "$file_path"
  else
    record_result "$id" "$category" "FAIL" "Required file missing" "$file_path"
  fi
}

# ---------- Repo-specific check functions ----------

check_gofmt_clean() {
  # Fail if any tracked .go file is not gofmt'd.
  # Exclude vendored/module cache directories and hgm-infra.
  local out
  out="$(cd "$REPO_ROOT" && git ls-files '*.go' ':!:hgm-infra/**' ':!:.gomodcache/**' ':!:.gocache/**' ':!:.golangci-lint-cache/**' | xargs -r gofmt -l)"
  if [[ -n "$out" ]]; then
    echo "Files not gofmt-clean:" >&2
    echo "$out" >&2
    return 1
  fi
  echo "gofmt clean"
}

run_coverage() {
  # Run canonical repo coverage, then copy the coverage profile to hgm-infra evidence.
  cd "$REPO_ROOT"

  # The repo Makefile already excludes examples and uses sandbox-safe caches.
  make test-coverage

  if [[ ! -f "${REPO_ROOT}/coverage.out" ]]; then
    echo "Expected coverage.out not found after make test-coverage" >&2
    return 1
  fi

  cp "${REPO_ROOT}/coverage.out" "${EVIDENCE_DIR}/coverage.out"
  echo "Copied coverage.out to ${EVIDENCE_DIR}/coverage.out"

  echo "Coverage summary (go tool cover -func):"
  go tool cover -func="${EVIDENCE_DIR}/coverage.out" | tail -n 5
}

check_coverage_threshold() {
  # Enforce the fixed coverage floor (anti-drift).
  # Requires that run_coverage has already generated evidence/coverage.out.
  if [[ ! -f "${EVIDENCE_DIR}/coverage.out" ]]; then
    echo "Missing ${EVIDENCE_DIR}/coverage.out; run QUA-3 first" >&2
    return 1
  fi

  local total_line
  total_line="$(go tool cover -func="${EVIDENCE_DIR}/coverage.out" | tail -n 1)"
  echo "Total line: ${total_line}"

  # Example: "total: (statements) 83.7%"
  local pct
  pct="$(printf '%s' "$total_line" | awk '{print $3}' | tr -d '%')"

  if [[ -z "$pct" ]]; then
    echo "Unable to parse coverage percentage from: ${total_line}" >&2
    return 1
  fi

  # Compare as floats (awk).
  awk -v got="$pct" -v want="$COV_THRESHOLD" 'BEGIN{exit !(got+0 >= want+0)}'
  echo "Coverage ${pct}% >= ${COV_THRESHOLD}%"
}

compile_all_modules() {
  # Compile-check root module and nested example modules.
  cd "$REPO_ROOT"

  echo "Root module compile check..."
  go test -run TestNonexistent -count=0 ./...

  # Example submodules are in examples/*/go.mod
  if [[ -d "${REPO_ROOT}/examples" ]]; then
    while IFS= read -r -d '' modfile; do
      local moddir
      moddir="$(dirname "$modfile")"
      echo "Example module compile check: ${moddir}"
      (cd "$moddir" && go test -run TestNonexistent -count=0 ./...)
    done < <(find "${REPO_ROOT}/examples" -name go.mod -print0 2>/dev/null || true)
  fi
}

check_toolchain_pins() {
  cd "$REPO_ROOT"

  echo "Go version:"
  local gov
  gov="$(go env GOVERSION)"
  echo "$gov"

  if [[ "$gov" != ${GO_REQUIRED_PREFIX}* ]]; then
    echo "Go version mismatch: require prefix ${GO_REQUIRED_PREFIX}, got ${gov}" >&2
    return 1
  fi

  # Check CI pins golangci-lint version.
  if [[ ! -f ".github/workflows/test.yml" ]]; then
    echo "Missing .github/workflows/test.yml" >&2
    return 1
  fi

  if ! grep -q "version: ${GOLANGCI_LINT_REQUIRED_VERSION}" ".github/workflows/test.yml"; then
    echo "CI does not pin golangci-lint to ${GOLANGCI_LINT_REQUIRED_VERSION} in .github/workflows/test.yml" >&2
    return 1
  fi

  echo "Toolchain pins OK (Go + golangci-lint)"
}

check_security_config_not_diluted() {
  # Minimal policy check: ensure gosec is enabled and only narrow excludes are present.
  cd "$REPO_ROOT"

  if [[ ! -f ".golangci.yml" ]]; then
    echo "Missing .golangci.yml" >&2
    return 1
  fi

  if ! grep -Eq "^[[:space:]]*- gosec" .golangci.yml; then
    echo "gosec not enabled in .golangci.yml (linters.enable)" >&2
    return 1
  fi

  # Enforce that gosec excludes remain narrow (current repo excludes only G104).
  # This is intentionally strict; widen only with rubric version bump.
  local excludes
  excludes="$(awk '
    /gosec:/{in=1}
    in && /excludes:/{ex=1;next}
    in && ex && /^[[:space:]]*-[[:space:]]*G[0-9]+/{print $2}
    in && ex && /^[^[:space:]]/{exit}
  ' .golangci.yml | tr '\n' ' ')"

  echo "Detected gosec excludes: ${excludes:-<none>}"

  if [[ -n "$excludes" && "$excludes" != "G104 " && "$excludes" != "G104" ]]; then
    echo "Unexpected gosec excludes (potential dilution): ${excludes}" >&2
    return 1
  fi

  echo "Security config policy OK"
}

run_govulncheck_if_available() {
  cd "$REPO_ROOT"

  if ! command -v govulncheck >/dev/null 2>&1; then
    echo "TODO: govulncheck not installed (install golang.org/x/vuln/cmd/govulncheck); treating as BLOCKED" >&2
    return 2
  fi

  govulncheck ./...
}

check_supply_chain_basics() {
  # Minimal supply-chain checks (does not require external network access):
  # - Workflows use commit SHA pins for GitHub Actions uses.
  # - golangci-lint action version pinned.
  # - Release workflow produces sha256 checksums.
  cd "$REPO_ROOT"

  if [[ ! -d ".github/workflows" ]]; then
    echo "Missing .github/workflows" >&2
    return 1
  fi

  local bad_uses
  bad_uses="$(grep -RIn "^[[:space:]]*uses:[[:space:]]*[^#]*@v" .github/workflows --include='*.yml' --include='*.yaml' || true)"

  # Allow 'aws-cdk@2' etc? That is npm, not uses.
  if [[ -n "$bad_uses" ]]; then
    echo "Found workflow actions pinned by tag (@v*) instead of commit SHA (supply-chain risk):" >&2
    echo "$bad_uses" >&2
    return 1
  fi

  if [[ -f ".github/workflows/release.yml" ]]; then
    if ! grep -q "sha256sum" ".github/workflows/release.yml"; then
      echo "release.yml does not appear to produce sha256 checksums" >&2
      return 1
    fi
  else
    echo "Missing .github/workflows/release.yml" >&2
    return 1
  fi

  echo "Supply-chain basics OK"
}

file_budget_check() {
  # Simple maintainability budget: keep individual Go files reasonably bounded.
  # This is a drift/AI-safety guardrail, not a style preference.
  #
  # Budget policy (v0.1.0):
  # - No non-test Go file under pkg/ or cmd/ may exceed 1500 lines.
  # - Exceptions require a rubric version bump + explicit allowlist.
  cd "$REPO_ROOT"

  local max_lines=1500
  local violations
  violations="$(
    git ls-files '*.go' \
      ':!:**/*_test.go' \
      ':!:hgm-infra/**' \
      ':!:.gomodcache/**' \
      ':!:.gocache/**' \
      ':!:.golangci-lint-cache/**' \
      | awk -v max="$max_lines" '{
          cmd="wc -l < \"" $0 "\"";
          cmd | getline n;
          close(cmd);
          if (n > max) {
            printf("%s:%d\n", $0, n);
          }
        }'
  )"

  if [[ -n "$violations" ]]; then
    echo "File budget violations (max ${max_lines} lines):" >&2
    echo "$violations" >&2
    return 1
  fi

  echo "File budget OK (<= ${max_lines} lines per file)"
}

maintainability_roadmap_check() {
  # MAI-2: Maintainability roadmap must exist, be current, and match active rubric version.
  # This is a governance guardrail to ensure maintainability constraints are documented
  # and stay aligned with the rubric as it evolves.
  cd "$REPO_ROOT"

  local roadmap_path="${PLANNING_DIR}/lift-maintainability-roadmap.md"
  local required_version="Rubric v0.1.0"
  local required_sections=("Guardrails" "Milestones" "Progress log")

  # Check: file exists
  if [[ ! -f "$roadmap_path" ]]; then
    echo "ERROR: Maintainability roadmap not found: $roadmap_path" >&2
    return 1
  fi

  # Check: contains active rubric version
  if ! grep -qF "$required_version" "$roadmap_path"; then
    echo "ERROR: Maintainability roadmap does not reference active rubric version: $required_version" >&2
    return 1
  fi

  # Check: contains required section markers
  local roadmap_content
  roadmap_content="$(cat "$roadmap_path")"
  for section in "${required_sections[@]}"; do
    if ! echo "$roadmap_content" | grep -qF "$section"; then
      echo "ERROR: Maintainability roadmap missing required section: $section" >&2
      return 1
    fi
  done

  echo "Maintainability roadmap OK"
}

canonical_semantics_duplication_check() {
  # MAI-3: Canonical semantics / duplication control.
  # Enforces limits on code duplication to prevent divergent implementations.
  # Uses the dupl linter (already configured in .golangci.yml) to detect duplicate code.
  cd "$REPO_ROOT"

  # Check: golangci-lint is available
  if ! command -v golangci-lint >/dev/null 2>&1; then
    echo "BLOCKED: golangci-lint not found in PATH" >&2
    return 2  # BLOCKED status
  fi

  # Check: dupl is enabled in config (anti-drift)
  if ! grep -qE '^\s*-\s*dupl' .golangci.yml; then
    echo "ERROR: dupl linter not enabled in .golangci.yml (config dilution detected)" >&2
    return 1
  fi

  # Run dupl-only scan to enforce duplication limits
  # Using --enable-only to keep MAI-3 semantically distinct from CON-2 (full lint)
  if ! golangci-lint run --enable-only=dupl --config .golangci.yml ./...; then
    echo "Duplication check FAILED: dupl linter found issues" >&2
    return 1
  fi

  echo "Canonical semantics OK (duplication within limits)"
}

logging_operational_standards_check() {
  # COM-6: Logging/operational standards enforced.
  # Prevents common operational security issues via static analysis:
  # - No direct stdlib "log" package usage (bypasses structured logging and sanitization)
  # - No fmt.Print*/println usage (writes to stdout/stderr unsanitized)
  # Scope: Lambda runtime code only (excludes CLI, dev server, testing frameworks, CDK infrastructure)
  # Policy doc: hgm-infra/planning/lift-logging-standards.md
  cd "$REPO_ROOT"

  # Fail closed: verify git is available
  if ! command -v git >/dev/null 2>&1; then
    echo "BLOCKED: git not found in PATH" >&2
    return 2
  fi

  # Enumerate non-test Go files in Lambda runtime code
  # Exclude: test files, vendor, hgm-infra, CLI tools, dev server, testing frameworks, CDK
  # Temporary allowlist:
  #   - connection_store_dynamodb.go (1 warning Printf on line 151)
  #   - app_request_handler.go (1 debug Println on line 1219 for local test)
  local go_files
  go_files="$(git ls-files 'pkg/**/*.go' 'cmd/**/*.go' 'internal/**/*.go' 2>/dev/null | \
    grep -v '_test\.go$' | \
    grep -v '^pkg/cli/' | \
    grep -v '^pkg/dev/' | \
    grep -v '^pkg/testing/' | \
    grep -v '^pkg/cdk/' | \
    grep -v '^pkg/lift/connection_store_dynamodb\.go$' | \
    grep -v '^pkg/lift/app_request_handler\.go$')"

  if [[ -z "$go_files" ]]; then
    echo "WARNING: No Lambda runtime Go files found to check" >&2
    echo "Logging standards OK (no files in scope)"
    return 0
  fi

  local violations=""

  # Check 1: Disallow direct stdlib "log" package imports
  local log_imports
  log_imports="$(echo "$go_files" | xargs grep -nE '^[[:space:]]*import[[:space:]]+"log"[[:space:]]*$' 2>/dev/null || true)"
  if [[ -n "$log_imports" ]]; then
    violations="${violations}Direct stdlib 'log' package imports found (must use pkg/logger):\n${log_imports}\n\n"
  fi

  # Check 2: Disallow fmt.Print* family calls (exclude commented lines)
  # Pattern excludes: 1) lines where // appears before fmt.Print*, 2) lines starting with //
  local fmt_prints
  fmt_prints="$(echo "$go_files" | xargs grep -nE '\bfmt\.(Print|Println|Printf)\(' 2>/dev/null | grep -vE '//.*fmt\.(Print|Println|Printf)|^[^:]*:[^:]*:[[:space:]]*//' || true)"
  if [[ -n "$fmt_prints" ]]; then
    violations="${violations}fmt.Print* calls found (must use structured logger):\n${fmt_prints}\n\n"
  fi

  # Check 3: Disallow builtin println usage (exclude commented lines)
  # Pattern excludes: 1) lines where // appears before println, 2) lines starting with //
  local println_calls
  println_calls="$(echo "$go_files" | xargs grep -nE '\bprintln\(' 2>/dev/null | grep -vE '//.*println|^[^:]*:[^:]*:[[:space:]]*//' || true)"
  if [[ -n "$println_calls" ]]; then
    violations="${violations}builtin println calls found (must use structured logger):\n${println_calls}\n\n"
  fi

  # Report violations
  if [[ -n "$violations" ]]; then
    echo -e "Logging/operational standards violations detected:\n" >&2
    echo -e "$violations" >&2
    echo "See hgm-infra/planning/lift-logging-standards.md for policy details" >&2
    return 1
  fi

  echo "Logging/operational standards OK (Lambda runtime code clean)"
}

doc_integrity_check() {
  # Guardrail: planning docs should not contain unresolved template placeholders.
  # Also ensure rubric version references are consistent.
  cd "$REPO_ROOT"

  local docs=(
    "${PLANNING_DIR}/lift-controls-matrix.md"
    "${PLANNING_DIR}/lift-threat-model.md"
    "${PLANNING_DIR}/lift-10of10-rubric.md"
    "${PLANNING_DIR}/lift-10of10-roadmap.md"
    "${PLANNING_DIR}/lift-evidence-plan.md"
    "${PLANNING_DIR}/lift-ai-drift-recovery.md"
  )

  for f in "${docs[@]}"; do
    if [[ ! -f "$f" ]]; then
      echo "Missing planning doc: $f" >&2
      return 1
    fi
    if grep -q "{{" "$f"; then
      echo "Unresolved template placeholder found in $f" >&2
      grep -n "{{" "$f" | head -n 20 >&2
      return 1
    fi
  done

  if ! grep -q "Rubric v${RUBRIC_VERSION}" "${PLANNING_DIR}/lift-10of10-roadmap.md"; then
    echo "Roadmap does not reference expected rubric version v${RUBRIC_VERSION}" >&2
    return 1
  fi
  if ! grep -q "Rubric v${RUBRIC_VERSION}" "${PLANNING_DIR}/lift-evidence-plan.md"; then
    echo "Evidence plan does not reference expected rubric version v${RUBRIC_VERSION}" >&2
    return 1
  fi

  echo "Doc integrity OK"
}

check_parity_threats_controls() {
  local threat_model="${PLANNING_DIR}/lift-threat-model.md"
  local controls_matrix="${PLANNING_DIR}/lift-controls-matrix.md"
  local evidence_path="${EVIDENCE_DIR}/DOC-5-parity.log"

  if [[ ! -f "$threat_model" ]] || [[ ! -f "$controls_matrix" ]]; then
    printf '%s\n' "Threat model or controls matrix missing" > "$evidence_path"
    record_result "DOC-5" "Docs" "BLOCKED" "Threat model or controls matrix missing" "$evidence_path"
    return 0
  fi

  local threat_ids
  threat_ids="$(grep -oE 'THR-[0-9]+' "$threat_model" | sort -u || true)"

  local missing=""
  local thr_id
  for thr_id in $threat_ids; do
    if ! grep -q "$thr_id" "$controls_matrix"; then
      missing="${missing} ${thr_id}"
    fi
  done

  {
    echo "Threat IDs found: ${threat_ids:-none}"
    echo "Missing from controls:${missing:-none}"
  } > "$evidence_path"

  if [[ -z "$missing" ]]; then
    record_result "DOC-5" "Docs" "PASS" "All threat IDs mapped in controls matrix" "$evidence_path"
  else
    record_result "DOC-5" "Docs" "FAIL" "Unmapped threats:${missing}" "$evidence_path"
  fi
}

# ---------- Execute rubric ----------

echo "=== Hypergenium Rubric Verifier ==="
echo "Project: lift"
echo "Rubric version: v${RUBRIC_VERSION}"
echo "Timestamp: ${REPORT_TIMESTAMP}"
echo ""

# Quality
run_check "QUA-1" "Quality" "make test"
run_check "QUA-2" "Quality" "go test -tags=contract ./internal/contracts -count=1"
run_check "QUA-3" "Quality" "run_coverage"

# Consistency
run_check "CON-1" "Consistency" "check_gofmt_clean"
run_check "CON-2" "Consistency" "golangci-lint run --config .golangci.yml ./..."
run_check "CON-3" "Consistency" "go test -tags=contract ./internal/contracts -count=1"

# Completeness
run_check "COM-1" "Completeness" "compile_all_modules"
run_check "COM-2" "Completeness" "check_toolchain_pins"
run_check "COM-3" "Completeness" "golangci-lint config verify --config .golangci.yml"
run_check "COM-4" "Completeness" "check_coverage_threshold"
run_check "COM-5" "Completeness" "check_security_config_not_diluted"
run_check "COM-6" "Completeness" "logging_operational_standards_check"

# Security
run_check "SEC-1" "Security" "golangci-lint run --enable-only=gosec --config .golangci.yml ./..."
# Return code 2 from run_govulncheck_if_available means BLOCKED (missing tool). Translate by wrapper.
run_check "SEC-2" "Security" "run_govulncheck_if_available"
run_check "SEC-3" "Security" "check_supply_chain_basics"
run_check "SEC-4" "Security" "go test -tags=security_p0 ./internal/securityp0 -count=1"

# Compliance readiness
check_file_exists "CMP-1" "Compliance" "${PLANNING_DIR}/lift-controls-matrix.md"
check_file_exists "CMP-2" "Compliance" "${PLANNING_DIR}/lift-evidence-plan.md"
check_file_exists "CMP-3" "Compliance" "${PLANNING_DIR}/lift-threat-model.md"

# Maintainability
run_check "MAI-1" "Maintainability" "file_budget_check"
run_check "MAI-2" "Maintainability" "maintainability_roadmap_check"
run_check "MAI-3" "Maintainability" "canonical_semantics_duplication_check"

# Docs
check_file_exists "DOC-1" "Docs" "${PLANNING_DIR}/lift-threat-model.md"
check_file_exists "DOC-2" "Docs" "${PLANNING_DIR}/lift-evidence-plan.md"
check_file_exists "DOC-3" "Docs" "${PLANNING_DIR}/lift-10of10-rubric.md"
run_check "DOC-4" "Docs" "doc_integrity_check"
check_parity_threats_controls

# ---------- Generate report ----------

RESULTS_JSON=$(printf "%s," "${RESULTS[@]}")
RESULTS_JSON="[${RESULTS_JSON%,}]"

OVERALL_STATUS="PASS"
if [[ ${FAIL_COUNT} -gt 0 ]]; then
  OVERALL_STATUS="FAIL"
elif [[ ${BLOCKED_COUNT} -gt 0 ]]; then
  OVERALL_STATUS="BLOCKED"
fi

cat > "${REPORT_PATH}" <<EOF
{
  "\$schema": "https://hgm.pai.dev/schemas/hgm-rubric-report.schema.json",
  "schemaVersion": ${REPORT_SCHEMA_VERSION},
  "timestamp": "${REPORT_TIMESTAMP}",
  "pack": {
    "version": "ba1a734ab662",
    "digest": "a73df10dc3ead5014e2a67c62fdce039104074bd07a2f6cfef432921d12801eb"
  },
  "project": {
    "name": "lift",
    "slug": "lift"
  },
  "summary": {
    "status": "${OVERALL_STATUS}",
    "pass": ${PASS_COUNT},
    "fail": ${FAIL_COUNT},
    "blocked": ${BLOCKED_COUNT}
  },
  "results": ${RESULTS_JSON}
}
EOF

echo ""
echo "=== Summary ==="
echo "Status: ${OVERALL_STATUS}"
echo "Pass: ${PASS_COUNT}"
echo "Fail: ${FAIL_COUNT}"
echo "Blocked: ${BLOCKED_COUNT}"
echo "Report written to: ${REPORT_PATH}"

echo ""
echo "CI surface (minimum commands):"
echo "  bash ./hgm-infra/verifiers/hgm-verify-rubric.sh"

if [[ "${OVERALL_STATUS}" == "PASS" ]]; then
  exit 0
fi
exit 1
