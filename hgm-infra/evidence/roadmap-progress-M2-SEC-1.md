# Roadmap Progress: M2-SEC-1

Date: 2026-01-13T17:32:52Z

Issue:
- SEC-1 failed due to golangci-lint flag drift ("unknown flag: --disable-all").

Verifier change:
- Old: golangci-lint run --disable-all --enable=gosec --config .golangci.yml ./...
- New: golangci-lint run --enable-only=gosec --config .golangci.yml ./...

Doc updates for parity:
- hgm-infra/planning/lift-10of10-rubric.md (SEC-1 command)
- hgm-infra/planning/lift-evidence-plan.md (SEC-1 refresh command)

Final result:
- SEC-1 PASS after rerunning verifier.
- Evidence: hgm-infra/evidence/SEC-1-output.log
- Report: hgm-infra/evidence/hgm-rubric-report.json
