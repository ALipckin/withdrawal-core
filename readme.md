# Withdrawal Core

REST service for safe withdrawal creation with idempotency and PostgreSQL transaction guarantees.

## Stack

- Go 1.24
- PostgreSQL 17
- Docker Compose

## Run

```bash
make docker-up
```

API is available at `http://localhost:8080`.

Bearer token for local run: `dev-token`.

## API

### Create withdrawal

`POST /v1/withdrawals`

Headers:

- `Authorization: Bearer dev-token`
- `Content-Type: application/json`

Body:

```json
{
  "user_id": 1,
  "amount": 100,
  "currency": "USDT",
  "destination": "wallet-address",
  "idempotency_key": "req-123"
}
```

Responses:

- `201` - withdrawal created (or idempotent replay with the same payload)
- `400` - validation error
- `404` - user not found
- `409` - insufficient balance
- `422` - same `idempotency_key` with different payload
- `401` - invalid bearer token

### Get withdrawal

`GET /v1/withdrawals/{id}`

Headers:

- `Authorization: Bearer dev-token`

Responses:

- `200` - withdrawal found
- `404` - withdrawal not found
- `401` - invalid bearer token

### Confirm withdrawal (optional)

`POST /v1/withdrawals/{id}/confirm`

Headers:

- `Authorization: Bearer dev-token`

Behavior:

- pending -> confirmed;
- already confirmed -> idempotent replay (`200` without duplicate ledger record).

Responses:

- `200` - confirmed or replay
- `404` - withdrawal not found
- `409` - invalid status
- `401` - invalid bearer token

## Consistency design

Double spending is prevented by:

1. Transaction per create request.
2. `SELECT ... FOR UPDATE` lock on `users` row before balance check/update.
3. Unique constraint on `(user_id, idempotency_key)` in `withdrawals`.
4. Canonical payload hash (`user_id`, `amount`, `currency`, `destination`) stored for key conflict validation.

This guarantees serial balance updates for one user and deterministic idempotent behavior under concurrent requests.

## Optional implementation

- Added `ledger_entries` table for audit trail.
- On create: ledger entry `withdrawal_create` with `amount_delta = -amount`.
- On confirm: ledger entry `withdrawal_confirm` with `amount_delta = 0`.
- Added structured JSON logs for events:
  - `withdrawal_created`
  - `withdrawal_create_failed`
  - `withdrawal_confirmed`
  - `withdrawal_confirm_failed`

## Tests

Run tests inside Docker:

```bash
make test
```

Implemented tests:

- create success;
- create insufficient balance;
- idempotency replay + payload conflict;
- concurrent create race on same user balance;
- confirm success;
- confirm idempotency;
- concurrent create with same idempotency key.

## Useful commands

```bash
make lint
make build
make docker-logs
make docker-down
make docker-clean
```
