#!/usr/bin/env bash
# Hypergenium Rubric Verifier (Single Entrypoint)
# Pack version: 4ae6c743d023
# Project: lift
# Domain: custom
#
# This script is the deterministic verifier entrypoint for hgm.validate.
# It reads planning state from hgm-infra/planning/, runs repo-specific check
# commands, writes evidence under hgm-infra/evidence/, and emits a fixed JSON
# report at hgm-infra/evidence/hgm-rubric-report.json.
#
# Usage (from repo root; scripts may be non-executable by default):
#   bash hgm-infra/verifiers/hgm-verify-rubric.sh
#
# Exit codes:
#   0 - All rubric items PASS
#   1 - One or more rubric items FAIL or BLOCKED
#   2 - Script error (missing dependencies, invalid config, etc.)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
HGM_INFRA="${REPO_ROOT}/hgm-infra"
PLANNING_DIR="${HGM_INFRA}/planning"
EVIDENCE_DIR="${HGM_INFRA}/evidence"
REPORT_PATH="${EVIDENCE_DIR}/hgm-rubric-report.json"

# Always run checks from repo root so relative commands are stable.
cd "${REPO_ROOT}"

# Optional repo-local tools directory (to enforce pinned tool versions deterministically).
# Tools are installed here (never system-wide) and put first on PATH.
HGM_TOOLS_DIR="${HGM_INFRA}/.tools"
HGM_TOOLS_BIN="${HGM_TOOLS_DIR}/bin"
mkdir -p "${HGM_TOOLS_BIN}"
export PATH="${HGM_TOOLS_BIN}:${PATH}"

# Tool pins.
# If these are unset, checks that depend on them must be marked BLOCKED (never “use whatever is installed”).
PIN_GOLANGCI_LINT_VERSION="v2.4.0"  # pinned in .github/workflows/test.yml
PIN_GOVULNCHECK_VERSION="TODO: pin govulncheck (e.g., v1.1.4)"

mkdir -p "${EVIDENCE_DIR}"

# Contract suite cache (so QUA-2 and CON-3 can share one run).
CONTRACT_SUITE_LOG="${EVIDENCE_DIR}/contract-suite.log"
CONTRACT_SUITE_EC="${EVIDENCE_DIR}/contract-suite.exitcode.log"

# Clean previous run outputs to prevent stale evidence from being misattributed.
rm -f \
  "${REPORT_PATH}" \
  "${EVIDENCE_DIR}/"*-output.log \
  "${EVIDENCE_DIR}/DOC-5-parity.log" \
  "${CONTRACT_SUITE_LOG}" \
  "${CONTRACT_SUITE_EC}"

REPORT_SCHEMA_VERSION=1
REPORT_TIMESTAMP="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
PASS_COUNT=0
FAIL_COUNT=0
BLOCKED_COUNT=0

declare -a RESULTS=()

json_escape() {
  local s="$1"
  s="${s//\\/\\\\}"
  s="${s//\"/\\\"}"
  s="${s//$'\n'/\\n}"
  s="${s//$'\r'/\\r}"
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

is_unset_token() {
  local v="$1"
  [[ -z "${v//[[:space:]]/}" ]] && return 0
  [[ "$v" == "TODO:"* ]] && return 0
  return 1
}

detect_golangci_lint_version_from_ci() {
  # Best-effort detection of the pinned golangci-lint version from GitHub Actions workflow config.
  # Fail closed if multiple versions are found.
  local wf_dir="${REPO_ROOT}/.github/workflows"
  [[ -d "$wf_dir" ]] || return 1

  local versions=""
  local file
  for file in "$wf_dir"/*.yml "$wf_dir"/*.yaml; do
    [[ -f "$file" ]] || continue

    local snippet
    snippet="$(grep -E '^[[:space:]]*version:[[:space:]]*v[0-9]+\.[0-9]+\.[0-9]+' "$file" | awk '{print $2}' | sort -u || true)"
    if [[ -n "$snippet" ]]; then
      versions+=$'\n'"$snippet"
    fi
  done

  versions="$(printf '%s' "$versions" | sed '/^$/d' | sort -u)"
  [[ -n "$versions" ]] || return 1
  if [[ "$(printf '%s\n' "$versions" | wc -l | tr -d ' ')" != "1" ]]; then
    return 1
  fi
  printf '%s' "$versions"
}

ensure_golangci_lint_pinned() {
  local v="$PIN_GOLANGCI_LINT_VERSION"
  if is_unset_token "$v"; then
    if v="$(detect_golangci_lint_version_from_ci)"; then
      :
    else
      echo "BLOCKED: golangci-lint version pin missing" >&2
      return 2
    fi
  fi
  if [[ "$v" != v* ]]; then
    v="v${v}"
  fi

  if ! command -v go >/dev/null 2>&1; then
    echo "BLOCKED: go toolchain not available to install golangci-lint ${v}" >&2
    return 2
  fi

  local want="${v#v}"
  if command -v golangci-lint >/dev/null 2>&1; then
    if golangci-lint --version 2>/dev/null | grep -q "$want"; then
      return 0
    fi
  fi

  echo "Installing golangci-lint ${v} into ${HGM_TOOLS_BIN}..." >&2
  if ! GOBIN="${HGM_TOOLS_BIN}" go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${v}"; then
    echo "BLOCKED: failed to install pinned golangci-lint ${v} (check network/toolchain)" >&2
    return 2
  fi

  if ! golangci-lint --version 2>/dev/null | grep -q "$want"; then
    echo "FAIL: installed golangci-lint does not report expected version ${v}" >&2
    golangci-lint --version 2>/dev/null || true
    return 1
  fi

  return 0
}

ensure_govulncheck_pinned() {
  local v="$PIN_GOVULNCHECK_VERSION"
  if is_unset_token "$v"; then
    echo "BLOCKED: govulncheck version pin missing (set PIN_GOVULNCHECK_VERSION)" >&2
    return 2
  fi
  if [[ "$v" != v* ]]; then
    v="v${v}"
  fi

  if ! command -v go >/dev/null 2>&1; then
    echo "BLOCKED: go toolchain not available to install govulncheck ${v}" >&2
    return 2
  fi

  if command -v govulncheck >/dev/null 2>&1; then
    if govulncheck -version 2>/dev/null | grep -q "govulncheck@${v}"; then
      return 0
    fi
  fi

  echo "Installing govulncheck ${v} into ${HGM_TOOLS_BIN}..." >&2
  if ! GOBIN="${HGM_TOOLS_BIN}" go install "golang.org/x/vuln/cmd/govulncheck@${v}"; then
    echo "BLOCKED: failed to install pinned govulncheck ${v} (check network/toolchain)" >&2
    return 2
  fi

  if ! govulncheck -version 2>/dev/null | grep -q "govulncheck@${v}"; then
    echo "FAIL: installed govulncheck does not report expected version ${v}" >&2
    govulncheck -version 2>/dev/null || true
    return 1
  fi

  return 0
}

prepare_check_env() {
  local id="$1"
  local cmd="$2"

  [[ -f "${REPO_ROOT}/go.mod" ]] || return 0

  case "$id" in
    CON-2|COM-3|SEC-1)
      if [[ -f "${REPO_ROOT}/.golangci.yml" ]] || [[ "$cmd" == *"golangci-lint"* ]]; then
        ensure_golangci_lint_pinned
      fi
      ;;
    SEC-2)
      if [[ "$cmd" == *"govulncheck"* ]]; then
        ensure_govulncheck_pinned
      fi
      ;;
    *) return 0 ;;
  esac
}

hgm_check_go_coverage() {
  if [[ ! -f "go.mod" ]]; then
    echo "BLOCKED: go.mod not found"
    return 2
  fi
  if ! command -v go >/dev/null 2>&1; then
    echo "BLOCKED: go toolchain not available"
    return 2
  fi

  local exclude_prefix="^github.com/pay-theory/lift/examples"
  local pkgs
  pkgs="$(go list ./... | grep -v "${exclude_prefix}" || true)"
  pkgs="$(printf '%s\n' "${pkgs}" | sed '/^$/d' || true)"
  if [[ -z "${pkgs}" ]]; then
    echo "BLOCKED: no packages selected for coverage"
    return 2
  fi

  go test -covermode=atomic -coverprofile="${EVIDENCE_DIR}/coverage.out" ${pkgs}

  if [[ ! -f "${REPO_ROOT}/scripts/coverage-core-report.sh" ]]; then
    echo "BLOCKED: scripts/coverage-core-report.sh not found"
    return 2
  fi

  bash "${REPO_ROOT}/scripts/coverage-core-report.sh" "${EVIDENCE_DIR}/coverage.out" "${EVIDENCE_DIR}/coverage.core.out"
}

hgm_check_core_coverage_threshold() {
  local threshold="90"

  hgm_check_go_coverage >/dev/null

  if [[ ! -f "${EVIDENCE_DIR}/coverage.core.out" ]]; then
    echo "BLOCKED: coverage core report missing at ${EVIDENCE_DIR}/coverage.core.out"
    return 2
  fi

  local total
  total="$(go tool cover -func="${EVIDENCE_DIR}/coverage.core.out" | tail -n 1 | awk '{print $3}' | tr -d '%')"
  if [[ -z "${total}" ]]; then
    echo "BLOCKED: failed to parse core coverage percent"
    return 2
  fi

  echo "core_coverage=${total}% threshold=${threshold}%"
  awk -v c="${total}" -v t="${threshold}" 'BEGIN{exit (c+0 < t)}'
}

hgm_check_gofmt_clean() {
  if ! command -v gofmt >/dev/null 2>&1; then
    echo "BLOCKED: gofmt not available"
    return 2
  fi

  local files
  files="$(find . -type f -name '*.go' \
    -not -path './.git/*' \
    -not -path './.gocache/*' \
    -not -path './.gomodcache/*' \
    -not -path './.golangci-lint-cache/*' \
    -not -path './hgm-infra/*' \
    -print)"

  if [[ -z "${files}" ]]; then
    return 0
  fi

  local out
  out="$(gofmt -l ${files} || true)"
  if [[ -n "${out}" ]]; then
    echo "gofmt changes required:"
    printf '%s\n' "${out}"
    return 1
  fi
}

hgm_check_contract_suite() {
  if [[ ! -f "go.mod" ]]; then
    echo "BLOCKED: go.mod not found"
    return 2
  fi
  if ! command -v go >/dev/null 2>&1; then
    echo "BLOCKED: go toolchain not available"
    return 2
  fi

  if [[ -f "${CONTRACT_SUITE_EC}" ]]; then
    if [[ -f "${CONTRACT_SUITE_LOG}" ]]; then
      cat "${CONTRACT_SUITE_LOG}"
    else
      echo "FAIL: contract suite cache missing log file at ${CONTRACT_SUITE_LOG}"
      return 1
    fi
    local ec
    ec="$(cat "${CONTRACT_SUITE_EC}" 2>/dev/null || echo 1)"
    return "${ec}"
  fi

  set +e
  go test -tags=contract -count=1 ./pkg/contract/... >"${CONTRACT_SUITE_LOG}" 2>&1
  local ec=$?
  set -e

  printf '%s' "${ec}" >"${CONTRACT_SUITE_EC}"
  cat "${CONTRACT_SUITE_LOG}"
  return "${ec}"
}

hgm_check_doc_integrity() {
  local files=(
    "hgm-infra/README.md"
    "hgm-infra/AGENTS.md"
    "hgm-infra/planning/lift-controls-matrix.md"
    "hgm-infra/planning/lift-threat-model.md"
    "hgm-infra/planning/lift-10of10-rubric.md"
    "hgm-infra/planning/lift-10of10-roadmap.md"
    "hgm-infra/planning/lift-evidence-plan.md"
    "hgm-infra/planning/lift-ai-drift-recovery.md"
  )

  local f
  for f in "${files[@]}"; do
    test -f "${f}"
    if grep -q "{{" "${f}"; then
      echo "Unrendered template token found in ${f}"
      return 1
    fi
  done
}

run_check() {
  local id="$1"
  local category="$2"
  local cmd="$3"

  local output_file="${EVIDENCE_DIR}/${id}-output.log"

  if [[ -z "${cmd//[[:space:]]/}" ]] || [[ "${cmd}" == "TODO:"* ]]; then
    printf '%s\n' "Verifier command not configured: ${cmd}" > "${output_file}"
    record_result "$id" "$category" "BLOCKED" "Verifier command not configured" "$output_file"
    return 0
  fi

  set +e
  (
    set -euo pipefail
    prepare_check_env "$id" "$cmd"
    eval "${cmd}"
  ) >"${output_file}" 2>&1
  local ec=$?
  set -e

  if [[ $ec -eq 0 ]]; then
    record_result "$id" "$category" "PASS" "Command succeeded" "$output_file"
  elif [[ $ec -eq 2 || $ec -eq 126 || $ec -eq 127 ]]; then
    record_result "$id" "$category" "BLOCKED" "Command reported BLOCKED (exit code ${ec})" "$output_file"
  else
    record_result "$id" "$category" "FAIL" "Command failed with exit code ${ec}" "$output_file"
  fi
}

check_file_exists() {
  local id="$1"
  local category="$2"
  local file_path="$3"

  if [[ -f "${file_path}" ]]; then
    record_result "$id" "$category" "PASS" "File exists" "$file_path"
  else
    record_result "$id" "$category" "FAIL" "Required file missing" "$file_path"
  fi
}

check_parity() {
  local threat_model="${PLANNING_DIR}/lift-threat-model.md"
  local controls_matrix="${PLANNING_DIR}/lift-controls-matrix.md"
  local evidence_path="${EVIDENCE_DIR}/DOC-5-parity.log"

  if [[ ! -f "${threat_model}" ]] || [[ ! -f "${controls_matrix}" ]]; then
    printf '%s\n' "Threat model or controls matrix missing" > "${evidence_path}"
    record_result "DOC-5" "Docs" "BLOCKED" "Threat model or controls matrix missing" "${evidence_path}"
    return 0
  fi

  local threat_ids
  threat_ids="$(grep -oE 'THR-[0-9]+' "${threat_model}" | sort -u || true)"

  local missing=""
  local thr_id
  for thr_id in ${threat_ids}; do
    if ! grep -q "${thr_id}" "${controls_matrix}"; then
      missing="${missing} ${thr_id}"
    fi
  done

  {
    echo "Threat IDs found: ${threat_ids:-none}"
    echo "Missing from controls:${missing:-none}"
  } > "${evidence_path}"

  if [[ -z "${missing}" ]]; then
    record_result "DOC-5" "Docs" "PASS" "All threat IDs mapped in controls matrix" "${evidence_path}"
  else
    record_result "DOC-5" "Docs" "FAIL" "Unmapped threats:${missing}" "${evidence_path}"
  fi
}

echo "=== Hypergenium Rubric Verifier ==="
echo "Project: lift"
echo "Timestamp: ${REPORT_TIMESTAMP}"
echo ""

# Commands are intentionally centralized here so the rubric docs and verifier stay aligned.
CMD_UNIT="./scripts/ci-check.sh"
CMD_INTEGRATION="hgm_check_contract_suite"

# Avoid embedding shell variables in command strings (easy to break under `set -u` + `eval`).
# For complex checks, prefer calling a function.
CMD_COVERAGE="hgm_check_go_coverage"
CMD_FMT="hgm_check_gofmt_clean"

CMD_LINT="make lint"
CMD_CONTRACT="hgm_check_contract_suite"

CMD_MODULES="go build ./..."

CMD_TOOLCHAIN="grep -q '^go 1.25$' go.mod; grep -q '1.25.x' .github/workflows/test.yml; grep -q 'version: v2.4.0' .github/workflows/test.yml"

CMD_LINT_CONFIG="test -f .golangci.yml; grep -q '^version: \"2\"$' .golangci.yml; grep -q 'modules-download-mode: readonly' .golangci.yml; grep -q -- '- gosec' .golangci.yml"

CMD_COV_THRESHOLD="hgm_check_core_coverage_threshold"

CMD_SEC_CONFIG="test -f .golangci.yml; grep -q -- '- gosec' .golangci.yml; grep -q 'gosec:' .golangci.yml; grep -q -- '- G104' .golangci.yml; if grep -q -- '- G101' .golangci.yml; then echo 'gosec excludes G101 (too high-signal)'; exit 1; fi"

# SAST is run as gosec only (separate from general lint), using pinned golangci-lint.
CMD_SAST="golangci-lint run --config .golangci.yml --enable-only=gosec ./..."

CMD_VULN="TODO: pin and run govulncheck (e.g., govulncheck ./...)"

# Supply chain: require actions pinned by commit SHA (no @v2/@v5) and ensure go.sum exists.
CMD_SUPPLY="test -f go.sum; if grep -R -- '^[[:space:]]*uses:[[:space:]].*@v[0-9]' .github/workflows/*.yml .github/workflows/*.yaml 2>/dev/null; then echo 'Unpinned GitHub Action detected (uses @vN)'; exit 1; fi; echo 'Actions appear SHA-pinned'"

CMD_P0="TODO: add domain P0 regression tests (secrets/logging/auth invariants)"

CMD_CONTROLS="test -f hgm-infra/planning/lift-controls-matrix.md"
CMD_EVIDENCE="test -f hgm-infra/planning/lift-evidence-plan.md"
CMD_THREAT_MODEL="test -f hgm-infra/planning/lift-threat-model.md"

CMD_DOCS="test -f hgm-infra/planning/lift-10of10-rubric.md && test -f hgm-infra/planning/lift-10of10-roadmap.md"

CMD_DOC_INTEGRITY="hgm_check_doc_integrity"

CMD_FILE_BUDGET="TODO: implement file-size/complexity budgets"
CMD_MAINTAINABILITY="TODO: maintainability verifier"
CMD_SINGLETON="TODO: singleton/canonical implementation verifier"

# === Quality (QUA) ===
run_check "QUA-1" "Quality" "$CMD_UNIT"
run_check "QUA-2" "Quality" "$CMD_INTEGRATION"
run_check "QUA-3" "Quality" "$CMD_COVERAGE"

# === Consistency (CON) ===
run_check "CON-1" "Consistency" "$CMD_FMT"
run_check "CON-2" "Consistency" "$CMD_LINT"
run_check "CON-3" "Consistency" "$CMD_CONTRACT"

# === Completeness (COM) ===
run_check "COM-1" "Completeness" "$CMD_MODULES"
run_check "COM-2" "Completeness" "$CMD_TOOLCHAIN"
run_check "COM-3" "Completeness" "$CMD_LINT_CONFIG"
run_check "COM-4" "Completeness" "$CMD_COV_THRESHOLD"
run_check "COM-5" "Completeness" "$CMD_SEC_CONFIG"
run_check "COM-6" "Completeness" "TODO: define logging/operational standards verifier"

# === Security (SEC) ===
run_check "SEC-1" "Security" "$CMD_SAST"
run_check "SEC-2" "Security" "$CMD_VULN"
run_check "SEC-3" "Security" "$CMD_SUPPLY"
run_check "SEC-4" "Security" "$CMD_P0"

# === Compliance Readiness (CMP) ===
check_file_exists "CMP-1" "Compliance" "${PLANNING_DIR}/lift-controls-matrix.md"
check_file_exists "CMP-2" "Compliance" "${PLANNING_DIR}/lift-evidence-plan.md"
check_file_exists "CMP-3" "Compliance" "${PLANNING_DIR}/lift-threat-model.md"

# === Maintainability (MAI) ===
run_check "MAI-1" "Maintainability" "$CMD_FILE_BUDGET"
run_check "MAI-2" "Maintainability" "$CMD_MAINTAINABILITY"
run_check "MAI-3" "Maintainability" "$CMD_SINGLETON"

# === Docs (DOC) ===
check_file_exists "DOC-1" "Docs" "${PLANNING_DIR}/lift-threat-model.md"
check_file_exists "DOC-2" "Docs" "${PLANNING_DIR}/lift-evidence-plan.md"
check_file_exists "DOC-3" "Docs" "${PLANNING_DIR}/lift-10of10-rubric.md"
run_check "DOC-4" "Docs" "$CMD_DOC_INTEGRITY"
check_parity

# === Generate Report ===
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
    "version": "4ae6c743d023",
    "digest": "83338d5db82b9af9247475a62d08623a3e930b6b6e2466d68b254d6842c1f2c2"
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

echo "Report written to: ${REPORT_PATH}"
echo "Status: ${OVERALL_STATUS} (pass=${PASS_COUNT} fail=${FAIL_COUNT} blocked=${BLOCKED_COUNT})"

if [[ "${OVERALL_STATUS}" == "PASS" ]]; then
  exit 0
fi
exit 1
