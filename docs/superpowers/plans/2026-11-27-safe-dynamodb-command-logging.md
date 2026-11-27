# Safe DynamoDB Command Logging Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop OneTable INFO logs from exposing DynamoDB request values while retaining safe operation metadata.

**Architecture:** Keep database operation behavior unchanged, but log only operation and model metadata in both executed and `Execute:false` prepared-command paths. Add public-API regression tests around a read, a typed transaction, and command preparation, update release notes to v0.13.1, review, merge the PR and tag the merge commit.

**Tech Stack:** Go, OneTable, GitHub CLI.

**Spec:** `docs/superpowers/specs/2026-11-27-safe-dynamodb-command-logging-design.md`

## Global Constraints

- Never log request or response data, or alter database operation behavior.
- Test that secret keys and values do not appear in INFO output while safe operation/model metadata remains visible.
- Update `RELEASE_NOTES.md` with `v0.13.1`.
- Create and merge a reviewed PR, then create and push the `v0.13.1` tag on its merge commit.

## Review Focus

1. Prepared commands with `Execute:false` must not leak keys or values while still being returned to callers (Task 1).
2. Read keys and consistency metadata must not leak (Task 1).
3. Typed cross-table transaction values, condition values, and table names must not leak (Task 1).
4. Safe operation/model metadata must remain present on executed and transaction paths (Task 1).
5. Concurrent logger writes must not race with test capture (Task 1).

---

### Task 1: Remove raw request values from INFO logs and release v0.13.1

**Files:**
- Modify: `table.go` (`Table.execute` INFO log), `model.go` (prepared-command INFO log).
- Create: `tests/logging_test.go`.
- Modify: `RELEASE_NOTES.md`.

**Interfaces:** No public API changes. Existing log message retains operation and model names; it omits the `cmd` map entirely.

- [x] Add `TestInfoLogOmitsReadRequestValues` and `TestInfoLogOmitsTransactionRequestValues`; assert request fields are absent and exact operation metadata remains.
- [x] Run `go test ./tests -run 'TestInfoLogOmits(ReadRequestValues|TransactionRequestValues)' -count=1`; confirm both tests fail because raw commands are logged.
- [x] Change `table.go` to log only operation and model metadata; add a `v0.13.1` release entry to `RELEASE_NOTES.md`.
- [ ] Add `TestInfoLogOmitsUnexecutedCommandValues`; assert no prepared command values are logged and `Execute:false` still returns the command.
- [ ] Run `go test ./tests -run TestInfoLogOmitsUnexecutedCommandValues -count=1`; confirm it fails because the prepared command is logged.
- [ ] Remove the prepared command from `model.go` INFO logging while retaining operation/model metadata.
- [ ] Run `gofmt -w table.go model.go tests/logging_test.go`, `go test ./...`, and `golangci-lint run --new-from-rev=origin/main`; all pass. Full `golangci-lint run` has 12 unrelated existing findings.
- [ ] Commit the implementation, request Reviewer-profile review, address blocking findings, then create and merge a PR against `main`.
- [ ] Create and push annotated tag `v0.13.1` on the merged PR commit; verify remote tag target and release module resolution.
