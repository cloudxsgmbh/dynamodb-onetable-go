# Release notes

# v0.13.1

November 27, 2026

- Stop logging DynamoDB request contents at INFO level to prevent sensitive keys and values from appearing in logs.

# v0.13.0

November 27, 2026

- Add typed transactions for composing model writes, standalone condition checks, and cross-table operations in one atomic request; retain the map-based API.
- Fix Query and Scan pagination, including continuation cursor propagation, terminal-page cursors, and numeric cursor precision.
- Make `Limit` cap matching results across filtered pages by budgeting each subsequent request. This may read more evaluated items than a single-page limit; `MaxPages` still bounds pagination.
- Improve transaction condition-check coverage and document the new transaction and pagination behavior.

# v0.11.0

November 2, 2026

- Upgrade dependencies

# v0.10.0

October 1, 2026

- Upgrade dependencies

# v0.9.0

September 7, 2026

- Upgrade dependencies

# v0.8.0

August 5, 2026

- Upgrade dependencies

# v0.7.0

July 27, 2026

- Replace legacy logging interface with JSON logger

# v0.6.2

July 23, 2026

- Update docs
- Add llms.txt

# v0.6.1

July 22, 2026

- Update docs

# v0.6.0

July 1, 2026

- Upgrade dependencies

# v0.5.0

June 1, 2026

- Upgrade dependencies

# v0.4.4

May 1, 2026

- Upgrade dependencies

# v0.4.3

April 14, 2026

- Fix logging to avoid multiple entries on JSON logs

# v0.4.2

April 14, 2026

- Fix linting issues
- Add unit tests for table, uid, ulid
- Add vulnerability check
