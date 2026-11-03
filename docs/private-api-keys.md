# Private API Key integration (MVP)

Status: FEATURE BRANCH; PRODUCTION NOT VERIFIED
Scope: GoWA backend, /integrations UI, private-key REST tool access and a separate Bearer MCP gateway. Legacy Basic Auth and OAuth routes remain unchanged; Hermes CS Bridge is not exposed.

## Create and revoke keys

1. The existing GoWA Basic Auth account must be explicitly listed in `APP_INTEGRATIONS_ADMIN_USERS`, and must not be listed in `APP_BASIC_AUTH_DEVICES`.
2. Visit `https://wa.ruangsanad.com/integrations?tab=api`.
3. Enter a descriptive integration name and **one valid GoWA device ID**. Optionally grant **REST Tools**, **MCP Tools**, and **automatic group REST operations**. All three are OFF by default and must be explicitly selected.
4. Generate the key and store the one-time Bearer token in the external platform's **server-side secret vault**. No plaintext token is stored in SQLite or listed later in the UI.
5. Use **Revoke** on the UI to invalidate a key immediately. Generate a new key to rotate credentials; update the external platform before revoking its old key.

Keys expire after 90 days. Each key is bound to one device and grants status read + personal text messaging. Optional REST and MCP tool scopes grant additional capabilities; group automation is a separate REST-only opt-in. Existing keys retain their original restricted grants after the additive database migration. Recreate keys if broader grants are needed.

GoWA uses `storages/private-api-keys.db` (under the existing persistent Docker volume) for hashed keys and idempotency records. If storage is unavailable, the new feature fails closed and existing integrations keep running. Do **not** delete this DB during upgrades.

## Machine API

Every private endpoint requires `Authorization: Bearer <your key>`; Basic Auth does not grant access to `/api/v1`. An API Key never authenticates to the legacy `/mcp` OAuth path, `/integrations` admin, `/cs-ai`, browser UI, device creation/deletion, or device webhook-secret operations. If and only if **REST Tools** is granted, the key also authenticates to explicitly allowlisted legacy **device-scoped** REST messaging endpoints.

Base URL (example): `https://wa.ruangsanad.com/api/v1` (prefix with APP_BASE_PATH if configured).

### Device status

`GET /api/v1/devices/{device_id}/status` with Bearer authentication. Only the device bound to the key can be queried.

### Send one text message

```http
POST /api/v1/messages
Authorization: Bearer <API_KEY>
X-Device-Id: <GRANTED_DEVICE_ID>
Content-Type: application/json
Idempotency-Key: req-unique-001

{"phone":"628123456789@s.whatsapp.net","message":"Pesan pengujian"}
```

Use your own authorized test recipient, not the illustrative number. Accepted personal destination formats: international phone digits or `<digits>@s.whatsapp.net`. Group JIDs ending `@g.us` are accepted **only** when the key was explicitly created with group permission.

A successful response contains `request_id`, `state: SENT`, and the GoWA `message_id` if available. **SENT means the send usecase returned successfully, not that the recipient has read/delivered the message.**

Every send must include an `Idempotency-Key` of 8-128 ASCII letters/digits/`:`/`_`/`-`. Sending the same key + same payload again returns the original request (without sending a second WhatsApp message). Reusing the same idempotency key for different content returns 409. On an ambiguous upstream send error, state is `UNCERTAIN` and callers must reconcile, not auto-retry under a new key. A request left `PENDING` after a crash is also **not** automatically resent.

### Get send result

`GET /api/v1/messages/{request_id}` returns only requests created by the same API key. It reports `PENDING`, `SENT` or `UNCERTAIN`; it is not a WhatsApp delivery/read-receipt endpoint.

### HTTP errors

- 400: malformed recipient, message, JSON or idempotency key
- 401: missing, invalid, expired or revoked API Key
- 403: another device or ungranted group send
- 404: send request not owned by this API Key
- 409: idempotency key conflicts with a different payload
- 429: per-key send rate cap (30 sends/minute, per GoWA process)
- 503: integration storage or backend unavailable; no fallback to legacy Basic Auth

## Expanded access to REST tools and MCP

### REST Tools (explicit opt-in)

An API Key with **REST Tools** can call the existing device-scoped GoWA REST surface, for example:

- `POST /send/message`, `POST /send/image`, `POST /send/video`, `POST /send/file` and other `/send/*` messaging routes
- `/chat/*`, `/chats`, `/message/*`, `/user/*`, `/call/*`, `/app/status` and relevant device-bound operations
- `/group/*` and `/newsletter/*` only when **Group REST** permission is also granted

Use `Authorization: Bearer <API_KEY>` and `X-Device-Id: <GRANTED_DEVICE_ID>` with these legacy endpoints. All requests are checked against the key's canonical device; aliases and query/path variants cannot switch devices. Existing legacy endpoint request/response shapes are unchanged, and their media formats follow `docs/openapi.yaml`.

The allowance is **not an open wildcard**: global device listing/creation/deletion, device webhook secrets, admin, CS Bridge, UI, OAuth, and unknown future routes remain blocked. Without the Group REST grant, group send requests and schedule-management mutations that could resume a group send are denied. Group REST permissions do not imply webhook or cross-device administration.

REST legacy mutations do **not** have the v1 text endpoint's durable idempotency guarantees. Do not automatically retry uncertain writes.

### MCP Tools (explicit opt-in)

Use the distinct Bearer endpoint:

```
https://wa.ruangsanad.com/api/v1/mcp
```

This endpoint exposes the same six consolidated tools available in GoWA:
`whatsapp_send`, `whatsapp_schedule`, `whatsapp_message`, `whatsapp_chat`,
`whatsapp_group` and `whatsapp_app`.

Compatible clients send `Authorization: Bearer <API_KEY>`, normal MCP Streamable HTTP JSON-RPC requests and, optionally, `X-Device-Id: <GRANTED_DEVICE_ID>`. Server-side checks enforce the canonical granted device even if a tool tries to override `device_id`. Batch JSON-RPC requests, unknown tools, or unsupported methods are denied.

**Human approval limitation:** direct MCP sends to groups, group mutations and schedule mutations remain blocked with `MCP_GROUP_APPROVAL_REQUIRED` until a durable approval service is introduced. Group reads (info, participants, join requests, non-reset invite link) remain available. The separate REST Group checkbox does not bypass the MCP gate.

The original `/mcp` route continues using existing Basic Auth or OAuth, not private API Keys. Claude connectors that require OAuth should continue using that endpoint; a client supporting custom Bearer headers can use `/api/v1/mcp`.

**Operational limits:** 30 API-Key MCP tool calls/minute per running GoWA process. Non-idempotent MCP operations cannot guarantee exactly-once delivery on network retries. Do not put tokens in prompts, URLs, frontend assets or logs.

## Security and operational limits

- HTTPS only outside the trusted internal network. Do not place keys in URLs, browser localStorage, client-side code or logs.
- The UI enforces the existing login, explicit admin allowlist, device-restricted account exclusion and same-origin form mutations. Raw key is shown once with `Cache-Control: no-store`.
- Automatic group send via this REST endpoint does **not** change the approval requirement for MCP agent-to-group sending.
- Keys grant one canonical device each. REST and MCP tool flags are selected on creation. Changes to permissions require creating a new key and revoking the previous one; no multi-device grants or in-place privilege edits.
- Group controls currently allow only per-device key permission and a per-key rate cap, not group JID allowlists, consent ledgers, delivery receipts or enterprise-wide quota enforcement. Review recipient authorization and applicable WhatsApp policies before granting auto-group send.
- Webhook subscription management is NOT implemented in API v1; the existing GoWA webhook configuration is independent and retains its existing authentication.
- The send-rate limiter is process-local and will require shared storage if GoWA scales horizontally.
- Creating/revoking keys is production-sensitive. Back up the SQLite DB and verify the selected client/device before applying.

## Verification and deployment

The branch's CI covers key persistence and privilege defaults, revoke/expiry, REST auth isolation and device scope, group permission, MCP tool policy and batch rejection, idempotency for v1 text, UI authentication and route ordering. CI passing is not deployment proof.

For production, explicitly approve merge and deployment separately. Verify persisted volume, route reachability, HTTPS, no token in logs, a safe send to an authorized test recipient, and revoke response. Do not change the Hermes CS Bridge configuration.
