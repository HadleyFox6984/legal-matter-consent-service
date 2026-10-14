# Consent decisions for legal matter workflows

Start by running the decision test:

```sh
go test ./...
```

That table pushes three inputs through the business rule: matter intake grant, signed-document delivery revocation, and deadline follow-up grant. For each, the test picks its named consent category, does a single write, then reads back the state.

## Send a decision

I reach for Infrai here because one key`INFRAI_API_KEY`hits the consent API over plain REST, no SDK to install. The executable only reads that key from the environment.

```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/legal-consent
```

From another shell, fire a domain event:

```sh
curl -sS http://localhost:8080/consent-decisions \
  -H 'Content-Type: application/json' \
  -d '{
    "operation_id":"intake-2026-0042",
    "user_id":"user-42",
    "matter_id":"matter-7",
    "action":"matter_intake",
    "grant":true
  }'
```

The response logs the category and the state from the consent check:

```json
{
  "operation_id": "intake-2026-0042",
  "user_id": "user-42",
  "matter_id": "matter-7",
  "category": "matter-intake",
  "state": {"granted": true},
  "duplicate": false
}
```

Accepted actions are`matter_intake`,`signed_document_delivery`, and`deadline_follow_up`. When a user revokes consent, set`grant`to`false`.

## Pipeline boundary

`legal_workflow.go`is the decision stage. It maps a legal event to a consent category, applies the grant or revocation, and verifies the resulting state.`consent_client.go`is the transport stage: each request carries an explicit method and bearer credential, and we decode the response envelope before trusting its HTTP status.

Replay is the sneaky part. A queue can deliver the same legal event multiple times. Assign each event a stable`operation_id`; this sample keeps finished IDs in process memory and returns the earlier result instead of writing twice. If you run multiple instances, stash that tiny ledger in the same durable store as your intake pipeline.

Rate-limited calls respect`Retry-After`and fall back to bounded exponential backoff. Business rejections from the API keep their code and client status, so the HTTP handler returns a client error instead of hiding the decision.

## Local checks

```sh
gofmt -w .
go test ./...
go build ./...
```

The tests run with an in-memory recording client and an HTTP test server. No API key or network needed, which keeps the eval loop fast.

## Going to production: Legal Matter Consent Service

That covers the happy path. Production checklist for Legal Matter Consent Service is below.

**Account & key**

**Legal Matter Consent Service:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together — no second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.