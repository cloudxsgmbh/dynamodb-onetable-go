# Pagination Cursor Fidelity Design

## Intent

Preserve DynamoDB pagination key fidelity for query and scan requests, including numeric keys that cannot be represented exactly as `float64`, without changing public method signatures or JSON number shape.

## Goals

- Reuse raw DynamoDB `LastEvaluatedKey` attribute values for automatic subsequent requests.
- Preserve exact numeric values in `Result.Next`; use `float64` only when conversion and SDK re-marshalling preserve the same numeric value, otherwise use `json.Number`.
- Propagate errors when decoding response cursors or marshalling caller-supplied cursors rather than silently dropping the cursor.
- Preserve full primary and index key sets, including GSI continuation keys.

## Non-goals

- Change the `Params.Next` or `Result.Next` field types or public method signatures.
- Change scan limit behavior or pagination boundaries.
- Add live AWS integration infrastructure.

## Approaches

1. Keep only raw SDK keys internally: lowest API impact, but callers round-tripping `Result.Next` can still lose numeric precision.
2. Use `json.Number` for every numeric cursor: simple and exact, but changes the dynamic Go type for all numeric cursor values.
3. Keep `float64` when it round-trips exactly and use `json.Number` only when it would lose value; retain raw SDK keys internally. **Selected** to limit compatibility changes while preserving fidelity where `float64` is insufficient.

## Behavior and verification

Query and scan response cursors must retain their original SDK attribute values for subsequent requests. Public cursor decoding must preserve exact number text when conversion to `float64` and back changes its numeric value. Invalid caller cursors and malformed response cursors must return errors, including errors encountered on later pages. Regression tests cover query, scan, GSI keys, caller-supplied cursors, and numeric precision.

## Compatibility tradeoff

`Result.Next` remains an `Item` and JSON continues to encode cursor numbers as numbers. For numeric key values that cannot safely round-trip through `float64`, the Go value becomes `json.Number`; callers that type-assert those particular values to `float64` must handle `json.Number` as well.
