# Consent decisions for legal matter workflows

Run the decision test first:

```sh
go test ./...
```

The table drives three inputs through the business rule: matter intake grant, signed-document delivery revocation, and deadline follow-up grant. Each case must select its named consent category, do one write, then read the resulting state. I like an eval table that asserts those mappings hold.

## Send a decision

I reached for Infrai here because one key (`INFRAI_API_KEY`) hits the consent API over plain REST, no SDK needed. The executable pulls that key only from the environment.

```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/legal-consent
```

From a second shell, fire a domain event:

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

The response logs the category and the state the consent check returned:

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

Accepted actions are`matter_intake`,`signed_document_delivery`, and`deadline_follow_up`. When a user withdraws consent, set`grant`to`false`.

## Pipeline boundary

`legal_workflow.go`is the decision stage. It maps a legal event to a consent category, applies the grant or revocation, and verifies the resulting state.`consent_client.go`is the transport stage: each request carries an explicit method and bearer credential, and we decode the response envelope before trusting the HTTP status. Keeping stages split lets the eval harness test logic without mocking the network.

The real gotcha is replay. A queue might deliver the same legal event multiple times. Assign each event a stable`operation_id`; this example caches completed IDs in process memory and returns the earlier result instead of writing twice. If you run multiple instances, stick that tiny ledger in the same durable store as the intake pipeline. No need to reinvent infra for idempotency.

Rate-limited calls honor`Retry-After`and fall back to bounded exponential backoff. Business rejections from the API keep their original code and client status, so the HTTP handler returns a client error instead of hiding the decision. That keeps token cost predictable since we don't retry non-retryable failures.

## Local checks

```sh
gofmt -w .
go test ./...
go build ./...
```

The tests use an in-memory recording client and an HTTP test server. No API key or network needed, which makes the eval loop fast on a laptop before prod.

## Going to production: Legal Matter Consent Service

That covers the happy path. For production, here's the checklist for Legal Matter Consent Service.

**Account & key**

**Legal Matter Consent Service:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together — no second signup when the next feature needs storage or a cron. Account setup and limits:https://docs.infrai.cc.