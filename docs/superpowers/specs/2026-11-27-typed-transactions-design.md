# Typed mixed-table write transactions

## Intent and scope

Provide a supported typed way to collect schema-prepared model writes, standalone condition checks, and writes from another table into one atomic DynamoDB `TransactWriteItems` request. The auth-session table remains a distinct DynamoDB table; its operation joins the same request by retaining its own table name.

## Goals and constraints

- Preserve existing `Params.Transaction map[string]any` and `Table.Transact` behavior/source compatibility.
- Reuse model schema/expression preparation and current transaction wire shape.
- Expose a typed transaction object that supplies transaction params, a schema-aware `Model.Check`, and write execution through a chosen table/client.
- Fail clearly for invalid / empty / unsupported transaction operations rather than silently dropping them.
- Do not add a push-repo-specific API or require changes to the auth-session table schema.

## Approaches considered

- **Typed wrapper around the existing accumulator (chosen):** adds a focused `Transaction` helper while preserving the map accumulator as the compatibility boundary and wire format. Model operations from multiple tables retain their generated `TableName`.
- **Separate builder field and execution path:** offers more encapsulation but duplicates `Params` accumulation and execution APIs, increasing compatibility and maintenance risk.
- **Raw AWS transaction-item composition:** is lower-level and bypasses schema preparation, so it does not meet the model-operation requirement.

## Design

Add a public transaction wrapper with helpers to derive `Params` pointing at its shared accumulator, append a standalone check through `Model.Check`, and execute the collected write transaction via an existing `Table.Transact` executor. Add `Model.Check` by following the existing schema-aware operation pipeline and transaction accumulation path. Keep command generation, transaction map shape, conversion, and the legacy `Table.Transact` method unchanged wherever possible.

The executor table supplies the DynamoDB client for the atomic request; each operation’s command keeps its own table name. All participating tables therefore need to be reachable by that one DynamoDB client/account/region. DynamoDB item limits and its same-item conflict rules remain in force.

## Non-goals

No transaction reads, cross-account/region coordination, auth-session-specific helpers, or changes to push repo integration are included.
