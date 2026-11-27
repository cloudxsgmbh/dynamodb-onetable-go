# Typed mixed-table write transactions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a typed helper that composes schema-prepared model writes and checks from separate tables into one atomic DynamoDB write.

**Architecture:** Keep `Params.Transaction map[string]any` and `Table.Transact` as the compatibility/wire boundary. Add a typed transaction wrapper that supplies shared operation params, delegates standalone checks to a new schema-aware `Model.Check`, and executes through a chosen table; each model-generated item retains its own table name.

**Tech Stack:** Go, AWS SDK for Go v2 DynamoDB types, existing OneTable model/expression pipeline.

**Spec:** `docs/superpowers/specs/2026-11-27-typed-transactions-design.md`

## Global Constraints

- Preserve existing `Params.Transaction map[string]any` and `Table.Transact` behavior/source compatibility.
- Reuse model schema/expression preparation and current transaction wire shape.
- Do not add a push-repo-specific API or require changes to the auth-session table schema.
- The executor table supplies the DynamoDB client for the atomic request; each operation’s command keeps its own table name.

## Review Focus

- No shared accumulator: constructors/helpers must return argument errors rather than panic.
- Nil `Params`: typed helper must provide safe defaults while preserving custom modifiers when supplied.
- Missing check condition (`Where` and `Exists` both absent) or transaction context: `Model.Check` must return clear argument errors, never issue a standalone request.
- Model/table mismatch: operations from a second schema must keep that model's `TableName` in the same request.
- Empty or invalid write transaction: avoid accidental DynamoDB calls and preserve the legacy map execution behavior.

---

### Task 1: Schema-aware standalone model checks

**Files:**
- Modify: `model.go` high-level CRUD API and operation preparation
- Test: `tests/typed_transact_test.go`

**Interfaces:**
- Produces: `func (m *Model) Check(ctx context.Context, properties Item, params *Params) (Item, error)`.
- Consumes: existing `checkArgs`, `prepareProperties`, `newExpression("check")`, `run`, and `accumulateTransaction` paths.

- [ ] **Step 1: Write failing tests**

Add tests that a check with key + `Params.Where` accumulates one `ConditionCheck` without executing; the generated command uses the model's table/key and schema-resolved condition attributes. Test `Exists` true/false, missing transaction, missing key rollback, and missing condition return argument errors.

- [ ] **Step 2: Run focused test to verify failure**

Run: `go test ./tests -run '^TestModelCheck' -count=1`
Expected: FAIL because `Model.Check` is not defined.

- [ ] **Step 3: Implement `Model.Check`**

Prepare properties and params using existing model validation; require a non-nil transaction accumulator and a non-empty `Where`; create the `check` expression and call `m.run(ctx, "check", expr)`.

- [ ] **Step 4: Run focused tests**

Run: `go test ./tests -run '^TestModelCheck' -count=1`
Expected: PASS, including validation errors.

### Task 2: Typed transaction composition and execution helpers

**Files:**
- Create: `transaction.go`
- Modify: `docs/api/transact.md`
- Test: `tests/typed_transact_test.go`

**Interfaces:**
- Produces: `type Transaction`, `func NewTransaction() *Transaction`, `func (tx *Transaction) Params(params *Params) (*Params, error)`, `func (tx *Transaction) Check(ctx context.Context, model *Model, properties Item, params *Params) (Item, error)`, and `func (tx *Transaction) Write(ctx context.Context, table *Table, params *Params) (any, error)`.
- Consumes: Task 1 `Model.Check`, existing `Params.Transaction`, and `Table.Transact(ctx, "write", ...)`.

- [ ] **Step 1: Write failing tests**

Test a single typed transaction with Create/Update/Remove across two distinct tables, plus `tx.Check` against a table, then verify the single `TransactItems` request contains the expected operation order and each table's own `TableName`. Test unique create/update operations retain the second table's name, replay retains the accumulator, cancellation errors preserve causes, and failed checks don't apply writes. Test nil transaction receiver and nil execution table return argument errors. Confirm modifiers on supplied `Params` survive without mutating caller params, nil receivers fail safely, and the zero value initializes its accumulator.

- [ ] **Step 2: Run focused test to verify failure**

Run: `go test ./tests -run '^TestTypedTransaction' -count=1`
Expected: FAIL because the typed wrapper and helper methods are not defined.

- [ ] **Step 3: Implement the typed wrapper**

Store one private map-compatible accumulator; `Params` attaches it to supplied or default params and returns an argument error for a nil receiver, `Check` delegates through those params, and `Write` delegates to existing `Table.Transact`. Initialize the accumulator for a zero-value transaction. Validate nil receiver/executor and keep the map wire shape unchanged.

- [ ] **Step 4: Document mixed-table usage and limits**

Add an example to `docs/api/transact.md` showing model calls with `tx.Params(...)`, `tx.Check(...)`, a second table's model, and one `tx.Write(...)`; note that one DynamoDB client/account/region executes the request and DynamoDB transaction limits still apply.

- [ ] **Step 5: Run focused tests**

Run: `go test ./tests -run '^Test(TypedTransaction|ModelCheck)' -count=1`
Expected: PASS with all operations represented in one write request.

### Task 3: Compatibility and full verification

**Files:**
- Test: `tests/typed_transact_test.go`
- Verify: repository test/build commands

**Interfaces:**
- Consumes: Tasks 1–2.
- Produces: regression coverage for legacy map transactions and final verification evidence.

- [ ] **Step 1: Add a legacy map compatibility test**

Accumulate an existing model operation through `Params.Transaction` and execute with `Table.Transact`; assert the previous map shape and behavior remain supported.

- [ ] **Step 2: Run focused compatibility tests**

Run: `go test ./tests -run '^Test(Transact|TypedTransaction|ModelCheck)' -count=1`
Expected: PASS.

- [ ] **Step 3: Run full test suite and static checks**

Run: `go test ./...`
Expected: all packages pass. Run `go vet ./...` if supported by repository CI.

- [ ] **Step 4: Inspect final diff and report**

Confirm only library code, focused tests, API docs, and design/plan docs changed; report API, test outputs, limitations, and potential integration conflicts. Do not commit or merge.
