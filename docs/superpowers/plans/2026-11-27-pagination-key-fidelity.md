# Pagination Key Fidelity Implementation Plan

> **For agentic workers:** Execute inline with TDD; do not commit or merge.

**Goal:** Keep query/scan pagination cursors exact across internal requests and caller round-trips, and surface cursor conversion failures.

**Architecture:** Keep the SDK `LastEvaluatedKey` map intact for subsequent requests. Decode a separate public `Item` cursor that retains `float64` only when SDK re-marshalling yields the same numeric value, otherwise using `json.Number`. Propagate decode and encode errors rather than dropping cursors.

**Tech Stack:** Go 1.26; AWS SDK for Go v2 DynamoDB AttributeValue and attributevalue packages; standard `encoding/json` and `math/big`.

**Spec:** `docs/superpowers/specs/2026-11-27-pagination-key-fidelity-design.md`

## Global Constraints

- Do not change `Params.Next` or `Result.Next` field types or public method signatures.
- Preserve JSON number shape; only precision-sensitive cursor numbers use `json.Number`.
- Preserve full primary and index key sets, including GSI continuation keys.
- Do not change scan limit behavior or pagination boundaries.
- Do not add live AWS integration infrastructure.
- Do not commit or merge.

## Review Focus

- Numeric key above 2⁵³ survives `Result.Next` and a caller-supplied follow-up cursor; test exact `json.Number` and actual SDK request value.
- High-precision decimal cursor survives unmarshal/remarshal; test a decimal whose value changes through `float64`.
- Exactly representable numeric cursor retains prior `float64` Go value; test a small integer and verify its type/value.
- GSI LastEvaluatedKey retains index and primary keys; test all four keys on the next QueryInput.
- Invalid caller cursor and malformed later-page cursor return errors; test Find and Scan and assert no silent restart/truncation.

---

### Task 1: Preserve cursor fidelity and propagate conversion errors

**Files:**
- Modify: `table.go` — preserve raw SDK LastEvaluatedKey and propagate cursor decode errors.
- Modify: `model.go` — use raw keys for internal continuation and produce a precision-aware `Result.Next`.
- Modify: `expression.go` — return caller-cursor marshal errors from command construction.
- Test: `tests/find_test.go` — query/scan pagination, numeric precision, GSI, and error regressions.

**Interfaces:**
- `unmarshallCursorFromDynamo(map[string]types.AttributeValue) (Item, error)` returns native Go values where exact and `json.Number` where float64 would alter the number.
- Query/scan execution returns both the decoded `LastEvaluatedKey` Item and the original AttributeValue map for internal continuation.

- [x] **Step 1: Add failing regressions** for two distinct pages carrying `9007199254740993` and a high-precision decimal; verify public cursor round-trip and exact subsequent QueryInput/ScanInput keys. Add a safe-integer `float64` assertion and a GSI query asserting all index and primary key attributes. Add Find/Scan cases for unsupported caller cursor values and malformed LastEvaluatedKey on a later page; assert errors and no silent cursor loss.
- [x] **Step 2: Run the focused tests and verify RED.** Run `go test ./tests -run 'TestPagination_' -count=1 -v`; expected failures must show precision loss, missing error propagation, or omitted cursor values rather than compilation/setup errors.
- [x] **Step 3: Preserve raw query/scan keys in `table.go`.** Keep the original `map[string]types.AttributeValue` for internal continuation, decode a separate public Item, and return contextual errors when cursor decoding fails.
- [x] **Step 4: Decode numeric cursors compatibly in `model.go`.** Use `attributevalue.UnmarshalMapWithOptions` with `UseNumber`; validate each number, then convert to `float64` only if `marshallForDynamo` produces an equivalent DynamoDB numeric value. Otherwise retain `json.Number`. `runMulti` must pass the raw SDK key unchanged and clear public cursor state on a terminal page.
- [x] **Step 5: Propagate caller-cursor marshal failures in `expression.go`.** Return the marshal error from command construction instead of proceeding without `ExclusiveStartKey`.
- [x] **Step 6: Run focused and full verification.** Run `go test ./tests -run 'TestPagination_' -count=1 -v`, then `go test ./... -count=1`, and `git diff --check`; all must pass.
