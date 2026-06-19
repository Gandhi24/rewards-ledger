# Rewards Points Ledger

A small HTTP API for a fintech rewards points program. Members earn points
through activities (purchases, referrals, cashback) and redeem them. Each
activity is recorded as a signed ledger entry, and a member's available balance
is the sum of those entries.

Built with **Go 1.22 and the standard library only** — no external
dependencies — which keeps it simple, fast to build, and easy to audit.

---

## Architecture

The code is organized in clean layers so each concern can be tested and changed
independently:

```
cmd/server          # entrypoint: config, HTTP server, graceful shutdown
internal/domain     # entities + business rules (no HTTP, no storage)
internal/store      # in-memory, concurrency-safe persistence
internal/api        # HTTP handlers, routing, error mapping
```

- **`domain`** owns the rules: which point types are credits vs debits, how a
  caller's positive magnitude is converted to a signed value, and how a balance
  is summed. It depends on nothing else, so the rules are trivial to unit-test.
- **`store`** is an in-memory implementation behind a mutex. The
  check-balance-then-write step for redemptions happens under a single write
  lock, so concurrent redemptions can't overdraw an account.
- **`api`** translates HTTP to/from the domain and maps domain errors to status
  codes. Routing uses Go 1.22's method+pattern `ServeMux`, so no router library
  is needed.

Swapping the in-memory store for a SQL-backed one later means implementing the
same methods the `api` layer calls — the domain and transport layers don't
change.

---

## Requirements

- Go 1.22+ **or** Docker.

---

## Running

### Option 1 — `run.sh`

```bash
./run.sh          # run locally (go run)
./run.sh docker   # build & run with docker compose
./run.sh test     # run the test suite
```

### Option 2 — Make

```bash
make run          # run locally on :8080
make test         # run tests with the race detector
make docker-up    # build & run with docker compose
```

### Option 3 — Docker Compose

```bash
docker compose up --build
```

The server listens on `:8080` by default. Override with the `PORT` env var:

```bash
PORT=9090 make run
```

A health check is available at `GET /health`.

---

## API

All requests and responses are JSON. Timestamps are RFC 3339 / ISO 8601 (UTC).

### `POST /members` — create a member

Request:

```json
{ "name": "Alice Johnson", "email": "alice@example.com" }
```

Response `201`:

```json
{
  "member_id": 1,
  "name": "Alice Johnson",
  "email": "alice@example.com",
  "created_at": "2024-01-15T09:00:00Z"
}
```

Errors: `400` (missing name/email or bad JSON), `409` (email already exists,
case-insensitive).

### `GET /members/{memberID}` — member info + balance

Response `200`:

```json
{
  "member_id": 1,
  "name": "Alice Johnson",
  "email": "alice@example.com",
  "points_balance": 450,
  "created_at": "2024-01-15T09:00:00Z"
}
```

Errors: `404` (member not found).

### `POST /rewards` — create a reward entry

The client always sends a **positive** `points` value. The system applies the
sign based on `point_type_id` (types 1–3 are credits, type 4 is a debit).

Request:

```json
{
  "member_id": 1,
  "point_type_id": 1,
  "points": 500,
  "description": "Purchase at Store A"
}
```

Response `201`:

```json
{
  "reward_id": 1,
  "member_id": 1,
  "point_type_id": 1,
  "points": 500,
  "description": "Purchase at Store A",
  "event_date": "2024-02-01T14:22:10Z"
}
```

Errors:
- `400` — invalid `point_type_id` (not 1–4) or non-positive `points`.
- `404` — `member_id` does not reference an existing member.
- `422` — redemption (`point_type_id = 4`) exceeds the available balance.

### `GET /members/{memberID}/rewards` — list a member's rewards

Returns reward entries in insertion order.

Response `200`:

```json
[
  {
    "reward_id": 1,
    "member_id": 1,
    "point_type_id": 1,
    "points": 500,
    "description": "Purchase at Store A",
    "event_date": "2024-02-01T14:22:10Z"
  }
]
```

Errors: `404` (member not found).

---

## Point types

| PointType_ID | Description       | Direction          |
|--------------|-------------------|--------------------|
| 1            | Purchase Earning  | Credit (positive)  |
| 2            | Referral Bonus    | Credit (positive)  |
| 3            | Cashback          | Credit (positive)  |
| 4            | Redemption        | Debit (negative)   |

---

## Validation rules

- A member cannot be created with a duplicate email (case-insensitive) → `409`.
- A redemption must not exceed the member's available balance → `422`.
  Redeeming exactly the available balance (down to `0`) is allowed.
- `member_id` must reference an existing member → `404`.
- `point_type_id` must be a valid type (1–4) → `400`.
- `points` must be a positive number; the system applies the sign → `400`.

---

## Testing

```bash
make test          # go test -race ./...
make test-cover    # with coverage summary
```

Tests cover three levels:
- **domain** — sign application and balance math for every point type.
- **store** — duplicate email, overdraft rejection, exact-balance redemption,
  insertion ordering, and a concurrency test that fires 50 simultaneous
  redemptions against a small balance and asserts the account never goes
  negative.
- **api** — full request/response flow per endpoint and every error status
  code, using `net/http/httptest`.

---

## Example session

```bash
# Create a member
curl -s -X POST localhost:8080/members \
  -H 'Content-Type: application/json' \
  -d '{"name":"Alice Johnson","email":"alice@example.com"}'

# Add rewards (system applies the sign)
curl -s -X POST localhost:8080/rewards -d '{"member_id":1,"point_type_id":1,"points":500,"description":"Purchase at Store A"}'
curl -s -X POST localhost:8080/rewards -d '{"member_id":1,"point_type_id":2,"points":200,"description":"Referred user Bob"}'
curl -s -X POST localhost:8080/rewards -d '{"member_id":1,"point_type_id":3,"points":50,"description":"Cashback on bill payment"}'
curl -s -X POST localhost:8080/rewards -d '{"member_id":1,"point_type_id":4,"points":300,"description":"Redeemed for voucher"}'

# Check balance -> 450
curl -s localhost:8080/members/1

# List rewards
curl -s localhost:8080/members/1/rewards
```

## Notes & trade-offs

- **Storage is in-memory** and resets on restart — appropriate for the scope of
  this exercise. The layering makes a durable store a drop-in replacement.
- **`event_date` is set server-side** at creation time, treating each entry as
  occurring "now". A real system might accept a caller-supplied event time.
- Balances are integer points; no fractional points are supported.
