# Transaction Operations

DynamoDB transactions provide atomic, all-or-nothing operations across multiple items. OneTable provides `Transact` on the `Table` type to execute prepared transactions.

---

## How transactions work

Transactions are built incrementally by passing a shared `map[string]any` accumulator via `Params.Transaction` to individual Model API calls. The calls are **not executed** — they are recorded into the transaction map. You then execute the entire transaction with a single `Table.Transact` call.

There are two transaction types:
- **`"write"`** — atomically create, update, or delete multiple items
- **`"get"`** — atomically read multiple items (consistent reads across items)

---

## TransactWrite

Atomically write multiple items. All writes succeed or all fail.

### Building a write transaction

```go
tx := map[string]any{}

// Accumulate operations — none of these hit DynamoDB yet
Account.Create(ctx, onetable.Item{"id": "a1", "name": "Acme Corp"}, &onetable.Params{Transaction: tx})
User.Create(ctx, onetable.Item{
    "id":        "u1",
    "name":      "Alice",
    "accountId": "a1",
}, &onetable.Params{Transaction: tx})

// Execute the transaction
_, err := table.Transact(ctx, "write", tx, nil)
if err != nil {
    log.Fatal(err)
}
```

### Mixing operation types

A single transaction can mix creates, updates, and deletes across different models:

```go
tx := map[string]any{}

// Create an order
Order.Create(ctx, onetable.Item{
    "id":     "order1",
    "userId": "user1",
    "total":  99.99,
}, &onetable.Params{Transaction: tx})

// Update the user's order count
User.Update(ctx, onetable.Item{
    "id":         "user1",
    "orderCount": 5,
}, &onetable.Params{Transaction: tx})

// Delete a pending item
Pending.Remove(ctx, onetable.Item{"id": "pending1"}, &onetable.Params{Transaction: tx})

_, err := table.Transact(ctx, "write", tx, nil)
```

### Upserts in transactions

Use `Update` with `Exists: nil` to create-or-update within a transaction:

```go
tx := map[string]any{}

User.Update(ctx, onetable.Item{
    "id":    "user1",
    "name":  "Alice",
    "email": "alice@example.com",
}, &onetable.Params{
    Transaction: tx,
    Exists:      nil, // upsert
})

_, err := table.Transact(ctx, "write", tx, nil)
```

### Conditional transactions

Use `Params.Where` to add conditions that must be satisfied for the transaction to succeed:

```go
tx := map[string]any{}

Order.Update(ctx, onetable.Item{
    "id":     "order1",
    "status": "shipped",
}, &onetable.Params{
    Transaction: tx,
    Where:       `${status} = {pending}`,
})

_, err := table.Transact(ctx, "write", tx, nil)
// If order status is not "pending", the transaction fails
```

### Execute=false

Set `Params.Execute` to `false` to inspect the prepared transaction without executing it:

```go
tx := map[string]any{}
Account.Create(ctx, onetable.Item{"id": "a1"}, &onetable.Params{
    Transaction: tx,
    Execute:     boolPtr(false),
})

prepared, err := table.Transact(ctx, "write", tx, nil)
// prepared contains the DynamoDB TransactWriteItems input
fmt.Printf("prepared transaction: %v\n", prepared)
```

---

## TransactGet

Atomically read multiple items with consistent reads across all items.

### Building a get transaction

```go
tx := map[string]any{}

table.Get(ctx, "Account", onetable.Item{"id": "a1"}, &onetable.Params{Transaction: tx})
table.Get(ctx, "User",    onetable.Item{"id": "u1"}, &onetable.Params{Transaction: tx})
table.Get(ctx, "Order",   onetable.Item{"id": "o1"}, &onetable.Params{Transaction: tx})

result, err := table.Transact(ctx, "get", tx, &onetable.Params{Parse: true})
if err != nil {
    log.Fatal(err)
}

// result is []Item when Parse=true
items := result.([]onetable.Item)
```

### Grouping results by type

Use `GroupByType` to organize the returned items by model:

```go
tx := map[string]any{}

table.Get(ctx, "Account", onetable.Item{"id": "a1"}, &onetable.Params{Transaction: tx})
table.Get(ctx, "User",    onetable.Item{"id": "u1"}, &onetable.Params{Transaction: tx})

result, err := table.Transact(ctx, "get", tx, &onetable.Params{Parse: true})
if err != nil {
    log.Fatal(err)
}

items := result.([]onetable.Item)
grouped := table.GroupByType(items, nil)

accounts := grouped["Account"]
users    := grouped["User"]

fmt.Printf("got %d accounts, %d users\n", len(accounts), len(users))
```

### Without parsing

When `Params.Parse` is `false` (the default), the raw DynamoDB response is returned:

```go
result, err := table.Transact(ctx, "get", tx, nil)
// result is map[string]any with raw DynamoDB TransactGetItems response
```

---

## Complete example

```go
func transferFunds(ctx context.Context, table *onetable.Table, fromID, toID string, amount float64) error {
    Account, err := table.GetModel("Account")
    if err != nil {
        return err
    }

    tx := map[string]any{}

    // Debit from account
    Account.Update(ctx, onetable.Item{"id": fromID}, &onetable.Params{
        Transaction: tx,
        Set: map[string]string{
            "balance": "${balance} - {" + fmt.Sprintf("%.2f", amount) + "}",
        },
        Where: `${balance} >= {` + fmt.Sprintf("%.2f`, amount) + `}`,
    })

    // Credit to account
    Account.Update(ctx, onetable.Item{"id": toID}, &onetable.Params{
        Transaction: tx,
        Set: map[string]string{
            "balance": "${balance} + {" + fmt.Sprintf("%.2f", amount) + "}",
        },
    })

    _, err = table.Transact(ctx, "write", tx, nil)
    if err != nil {
        return fmt.Errorf("transfer failed: %w", err)
    }

    return nil
}
```

---

## Limitations

- Maximum **100 items** per transaction (DynamoDB limit).
- Maximum **4 MB** total request size (DynamoDB limit).
- Cannot use `Scan` or `Query` inside a transaction — only individual item operations.
- Transaction items must not overlap (same item appearing in multiple operations).
- DynamoDB transactions are charged at 2x the cost of normal reads/writes.

---

## Batch vs Transaction

| Feature | Batch | Transaction |
|---------|-------|-------------|
| Atomicity | No (individual items may fail) | Yes (all-or-nothing) |
| Max items | 100 (get) / 25 (write) | 100 |
| Max size | 16 MB | 4 MB |
| Cost | 1x | 2x |
| Operations | Read-only (get) or write-only | Mixed reads and writes |
| Conditional | No | Yes (via `Params.Where`) |

Use **batches** for bulk operations where atomicity is not required. Use **transactions** when you need all-or-nothing guarantees.
