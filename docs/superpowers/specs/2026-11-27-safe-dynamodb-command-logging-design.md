# Safe DynamoDB command logging

## Intent

Prevent OneTable from writing DynamoDB request contents to INFO logs, which can contain keys, personal data, and secrets. Keep operational visibility into the operation and model without exposing request values.

## Approaches

- **Remove request commands from INFO logs (chosen):** retain the operation/model summary, omit the command map. This is safe by default and avoids fragile field allowlists.
- **Redact selected fields:** keep command details but maintain an allowlist/redactor. Rejected because future fields could bypass redaction.
- **Make command logs opt-in:** reduces exposure only when every deployment correctly configures the flag; rejected as unsafe default.

## Design and constraints

- In `table.go`, log operation and model only; never include command/properties/keys/values in INFO output.
- Add regression tests for sensitive values in a normal read and a mixed write transaction; verify safe operation metadata remains visible.
- Add a `v0.13.1` release entry to `RELEASE_NOTES.md`.
- Run library tests, formatter and linter; request review, create and merge a GitHub PR, then create and push the `v0.13.1` tag on the merge commit.
- Do not log request or response data, or alter database operation behavior.
