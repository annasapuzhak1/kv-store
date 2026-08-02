# kv-store

A key-value store microservice in Go, with a Go client for other services
to use, and per-entry AES-256-GCM encryption.

## Running it

Requires Go 1.22+.

```bash
go run ./cmd/server
```

The server listens on `:8080` by default (override with the `PORT` env var).

## Running the tests

```bash
go test ./...
```

To also check for data races in the concurrent storage layer:

```bash
go test -race ./...
```

## API

| Operation | Method & Path | Body | Response |
|---|---|---|---|
| STORE | `POST /store` | `{"id": "...", "data": "..."}` | `201` `{"key": "<hex>"}` (or `409` if id already exists) |
| RETRIEVE | `GET /retrieve/{id}?key=<hex>` | - | `200` `{"data": "..."}` |
| UPDATE | `PUT /update/{id}?key=<hex>` | `{"data": "..."}` | `204 No Content` |
| DELETE | `DELETE /delete/{id}?key=<hex>` | - | `204 No Content` |

STORE is insert-only, not an upsert: calling it twice with the same `id`
returns `409 Conflict` on the second call rather than silently overwriting
the existing entry.

### Example (curl)

```bash
# Store
curl -X POST localhost:8080/store \
  -d '{"id":"user-123","data":"name: John Doe"}'
# => {"key":"a1b2c3..."}

# Retrieve (use the key returned above)
curl "localhost:8080/retrieve/user-123?key=a1b2c3..."
# => {"data":"name: John Doe"}

# Update
curl -X PUT "localhost:8080/update/user-123?key=a1b2c3..." \
  -d '{"data":"name: Jane Doe"}'
# => 204 No Content

# Delete
curl -X DELETE "localhost:8080/delete/user-123?key=a1b2c3..."
# => 204 No Content
```

### Using the Go client

```go
import "github.com/annasapuzhak1/kv-store"

c := client.New("http://localhost:8080")

key, err := c.Store("user-123", []byte("name: John Doe"))
// caller is responsible for storing `key` securely - the server does not
// keep a copy.

data, err := c.Retrieve("user-123", key)

err = c.Update("user-123", key, []byte("name: Jane Doe"))

err = c.Delete("user-123", key)
```

A runnable end-to-end example using the client (store → retrieve → update →
retrieve → delete → confirm it's gone, with assertions at each step) is in
`cmd/example/main.go`. With the server running in one terminal:

```bash
go run ./cmd/example
```

## Design

### The server never persists an encryption key

This is the central design decision of the implementation, and it comes
directly from reading the spec's inputs/outputs table closely: `STORE`
*outputs* an encryption key, and `RETRIEVE`/`UPDATE`/`DELETE` all *require*
the key as an input. If the server kept its own copy of the key, it
wouldn't need the caller to supply it back.

So the flow is:

1. `STORE` generates a fresh random AES-256 key, encrypts the data with
   AES-256-GCM, and persists the ciphertext together with the nonce used to
   encrypt it (see "Ciphertext/nonce representation" below). The key is
   returned to the caller once and then discarded - it is never written to
   disk or kept in memory beyond the request.
2. `RETRIEVE`/`UPDATE`/`DELETE` all require the caller to supply that key
   again. The server uses it to decrypt (and, for `UPDATE`, re-encrypt with
   the *same* key, per the spec's "Update outputs: none" - the key does not
   change).

**Security property this gives us:** if an attacker obtains a dump of the
underlying storage, they get nothing but ciphertext - the keys needed to
decrypt it were never stored anywhere on the server side.

**Tradeoff:** the caller is now the sole custodian of the key. If they lose
it, the data is permanently unrecoverable - there is no server-side
recovery path by design. This is a deliberate consequence of the spec, not
an oversight, and is worth being explicit about in review.

**Wrong key and missing ID are intentionally indistinguishable to the
caller.** `Retrieve`/`Update`/`Delete` all require successfully decrypting
the entry with the supplied key. Rather than returning different responses
for "id doesn't exist" (404) vs. "key is wrong" (403), both cases return
the same `404 Not Found` with a generic message. This closes an
information-leak oracle: someone probing the API without a valid key can't
learn which IDs exist by observing whether they get a different error for
"not found" vs. "wrong key." Internally, the service layer still
distinguishes the two cases (`service.ErrNotFound` vs.
`service.ErrDecryptFailed`) and both are logged server-side with that
distinction intact - only the external HTTP response is deliberately
collapsed.

### STORE is insert-only

`storage.Create` (called by `service.Store`) rejects an `id` that already
exists rather than overwriting it, returning `409 Conflict` at the API
layer. This matches STORE's spec description ("Insert data into the
store") as a distinct operation from UPDATE, rather than blurring the two
into one upsert behavior. The tradeoff: a caller retrying a STORE call
after a network timeout (unsure whether the first attempt landed) will get
a `409` on the retry rather than it silently succeeding again - that's
correct for "insert" semantics, but means retry logic on the caller's side
needs to treat `409` as "already done," not as a failure to retry further.

### Logging

The server logs each request that results in an error, server-side only:

```
2026/08/02 16:56:44 GET /retrieve/id-1 -> error: service: decryption failed - wrong key or corrupted data
```

This is deliberately more detailed than what the HTTP response tells the
caller (see the wrong-key/not-found note above) - it gives an operator
visibility to notice patterns like repeated decryption failures against
the same ID (a possible brute-force attempt), without exposing that same
detail externally. The encryption key itself is never logged, on failure
or otherwise.

### Ciphertext/nonce representation

`encryption.Encrypt` returns the ciphertext and the nonce used to produce
it as two separate `[]byte` values, rather than packing them into one blob.
`storage.Entry{Ciphertext, Nonce}` is the named record that actually gets
persisted. The `encryption` package has no dependency on `storage` (or vice
versa) - `service` is the only package that imports both, and is
responsible for building a `storage.Entry` from encryption's output, and
unpacking one back into `Ciphertext`/`Nonce` before decrypting. This keeps
each package independently testable and readable without needing to know
an implicit "first N bytes are the nonce" convention.

### Storage: in-memory

The spec explicitly says "there are no particular requirements for how the
data is stored," so I used a simple `map[string][]byte` guarded by a
`sync.RWMutex` rather than an embedded database (e.g. bbolt) or an external
one (e.g. Postgres). This keeps the project trivially easy to run/verify
(no external dependencies, no files to clean up) and let me spend the
available time on the part of the task that's actually being evaluated -
the encryption design and correctness of Store/Retrieve/Update/Delete.

Storage is defined as an interface (`internal/storage.Store`), so a
persistent implementation (bbolt, SQL, etc.) could be substituted without
touching the service, API, or client layers.

### Architecture

```
client/                     <- Go client other services import (HTTP under the hood)
   |
   v  HTTP
internal/api/                <- handlers: HTTP <-> service translation only
   |
   v
internal/service/            <- business logic: the only package that knows
   |         |                  about both encryption and storage
   v         v
internal/    internal/
encryption/  storage/
```

Each layer only knows about the one below it, which keeps concerns
separated and each package independently testable (see the accompanying
`_test.go` file per package).

### Transport: HTTP/JSON

Chosen over gRPC for this exercise since it needs no code generation step
and keeps the whole project runnable with just `go run`/`go test` - nothing
else to install. The `internal/api` and `client` packages are the only
places that know the transport is HTTP, so a different transport could be
added alongside them without touching `service`, `storage`, or
`encryption`.

## Possible extensions

- **Persistent storage.** Swap `MemoryStore` for a `bbolt`-backed
  implementation of the same `storage.Store` interface so data survives a
  restart.
- **Hide the ID → storage-key mapping.** Currently IDs are used directly as
  storage map keys. To harden against a raw database dump revealing which
  IDs exist, the storage key could instead be `HMAC(serverSecret, id)`
  rather than the plaintext ID.
- **Split storage into its own microservice**, so storage and encryption
  can scale independently, as suggested in the brief.
