# Private API Key integration (MVP)

Status: IMPLEMENTED ON FEATURE BRANCH; PRODUCTION NOT VERIFIED
Scope: GoWA backend and /integrations UI; does not modify legacy REST, Claude/Hermes MCP or Hermes CS Bridge.

## Create and revoke keys

1. The existing GoWA Basic Auth account must be explicitly listed in `APP_INTEGRATIONS_ADMIN_USERS`, and must not be listed in `APP_BASIC_AUTH_DEVICES`.
2. Visit `https://wa.ruangsanad.com/integrations?tab=api`.
3. Enter a descriptive integration name, **one valid GoWA device ID**, and (only if truly required) select **automatic group sending**.
4. Generate the key and store the one-time Bearer token in the external platform's **server-side secret vault**. No plaintext token is stored in SQLite or listed later in the UI.
5. Use **Revoke** on the UI to invalidate a key immediately. Generate a new key to rotate credentials; update the external platform before revoking its old key.

Keys expire after 90 days. Each key is bound to one device and grants status read + personal text messaging. The separate group-send checkbox adds explicit automatic group messaging to that same device. Default group access is denied.

GoWA uses `storages/private-api-keys.db` (under the existing persistent Docker volume) for hashed keys and idempotency records. If storage is unavailable, the new feature fails closed and existing integrations keep running. Do **not** delete this DB during upgrades.

## Machine API

The private endpoints are **separate from legacy routes**. Authentication requires `Authorization: Bearer <your key>`; Basic Auth does not grant access to the private v1 endpoints. Conversely, these keys cannot access `/send/*`, `/mcp`, `/cs-ai`, UI or legacy routes.

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

## Security and operational limits

- HTTPS only outside the trusted internal network. Do not place keys in URLs, browser localStorage, client-side code or logs.
- The UI enforces the existing login, explicit admin allowlist, device-restricted account exclusion and same-origin form mutations. Raw key is shown once with `Cache-Control: no-store`.
- Automatic group send via this REST endpoint does **not** change the approval requirement for MCP agent-to-group sending.
- This MVP supports **one granted device per key**, **text send**, **device status** and **submission status**. Key rotation uses create-new/revoke-old; no editable key grants or multiple device memberships.
- Group controls currently allow only per-device key permission and a per-key rate cap, not group JID allowlists, consent ledgers, delivery receipts or enterprise-wide quota enforcement. Review recipient authorization and applicable WhatsApp policies before granting auto-group send.
- Webhook subscription management is NOT implemented in API v1; the existing GoWA webhook configuration is independent and retains its existing authentication.
- The send-rate limiter is process-local and will require shared storage if GoWA scales horizontally.
- Creating/revoking keys is production-sensitive. Back up the SQLite DB and verify the selected client/device before applying.

## Verification and deployment

The branch's CI covers persistence, revoke/expiry, REST auth isolation, device scope, group permission, idempotency, status ownership, UI authentication and route ordering. CI passing is not deployment proof.

For production, explicitly approve merge and deployment separately. Verify persisted volume, route reachability, HTTPS, no token in logs, a safe send to an authorized test recipient, and revoke response. Do not change the Hermes CS Bridge configuration.
