# PRD — Private External API & MCP Management for GoWA

Status: APPROVED DIRECTION / IMPLEMENTATION PARTIAL (2026-11-03)
Repository: ruangsanadindonesia-art/go-whatsapp-web-multidevice
Related: [GoWA MCP / Dashboard PRD](prd-gowa-mcp-dashboard-integration.md)

## Locked product decisions

1. External access is a **private integration**, with manually provisioned and revocable client identities, not open self-registration.
2. The first supported external REST v1 use cases are **send message, message/device send status, and webhook subscriptions**.
3. **REST group sending is automatic only for an explicitly granted client** with \`send:group\`; default deny. This decision does not remove human approval for direct Claude/Hermes MCP group sends in the existing PRD.
4. Create a GoWA-owned UI page covering **REST API and MCP management**. Do not expose password/token contents in the UI.
5. Preserve existing REST/Basic Auth, MCP OAuth, Dashboard business outbox and Hermes CS Bridge pathways.

## Architecture

External application -> Scoped REST API v1 -> Authenticated principal/device/action policy -> Shared GoWA usecases -> WhatsApp.

Claude/Hermes -> MCP OAuth -> Scoped principal/device/tool policy -> Human approval for direct group requests -> Shared GoWA usecases.

Dashboard business message -> Dashboard business validation/outbox -> Existing REST/Bridge route. The Dashboard remains business source of truth.

### Permissions (proposed)

- \`device:status\`, \`message:send:personal\`, \`message:send:group\`, \`message:status\`, \`webhook:manage\`.
- \`send:group\` requires a named client, explicit device grant, rate/burst caps, recipient eligibility, consent/allowed-use policy and immediate revoke.
- Default every new client to **no device grants / no actions**; operator adds least privilege.
- Origin/principal is derived from authenticated server-side identity, never caller-supplied metadata.
- An API key cannot be used against existing privileged REST or MCP endpoints to bypass policy. Existing Basic Auth remains distinct from v1 credentials.
- Device \`cs-ai\` stays behind the existing Hermes CS Bridge for lead/CRM workflows. No private key is granted generic raw send to this device.
- MCP direct group requests still require durable human approval per occurrence.

## Initial REST contract (proposal, not implemented)

| Method | Path | Permission | Purpose |
|---|---|---|---|
| GET | /api/v1/devices | device:status | List only granted devices |
| GET | /api/v1/devices/{device_id}/status | device:status | Connection state |
| POST | /api/v1/messages | message:send:personal / message:send:group | One logical send |
| GET | /api/v1/messages/{request_id} | message:status | Result and reconciliation |
| POST | /api/v1/webhooks | webhook:manage | Subscribe to scoped events |
| GET | /api/v1/webhooks | webhook:manage | Show subscriptions |
| DELETE | /api/v1/webhooks/{id} | webhook:manage | Revoke subscription |

**API v1 responses:** request_id, transport state, wa_message_id when available, explicit error_code; SENT is not DELIVERED or READ. Support \`Idempotency-Key\`, deterministic duplicate responses, UNCERTAIN state for ambiguous timeout, and avoid blind retries.

**Webhook:** per-client scoped events, HMAC signature, timestamp, event_id, retry with bounded backoff and replay deduplication. Never include credentials or unredacted sensitive data by default. Validate callback endpoints against SSRF and DNS/redirect attacks.

**Media:** URL hardening and private TTL media staging follow the original GoWA PRD; not automatically included in the first text-send slice.

## Management UI stages

- **Stage 1, implemented in this change:** \`/integrations\` shows current REST Basic identities (usernames and configured device scopes only), MCP enablement/OAuth state, six existing tool categories and documentation. It is authenticated, unavailable when Basic Auth is unset, and denies device-restricted Basic users. It is read-only: no false API Key creation or permission controls.
- **Stage 2, not implemented:** API client create/revoke/rotate, per-client device/action grants, audit history, send quotas and webhook management. Controls are enabled only when backed by tested server-side policy and persistent token hashing.
- **Stage 3, not implemented:** MCP OAuth principal/client listing, granular MCP tool/device grants, revocation, group approval queue and audited management operations. Requires authenticated operator role + CSRF protection.
- **UI ownership:** GoWA backend serves \`/integrations\` independent of third-party \`aldinokemal/gowa-ui\` releases. Adding it to the upstream UI's navigation requires an organization-owned UI build/fork or an upstream contribution; do not claim upstream navigation is changed by this PR.

## Acceptance and verification

- Anonymous or unrestricted deployment without APP_BASIC_AUTH cannot open management inventory.
- Restricted Basic accounts cannot open management page.
- Authorized unrestricted account can see current API/MCP configuration metadata without secrets or HTML injection.
- New v1 client must be unable to access any device/action not granted, including replacing device_id or X-Device-Id.
- A client lacking message:send:group cannot send to groups; a granted client is allowed only for its devices/targets within caps.
- New API keys must not authenticate to legacy REST, MCP, Dashboard or CS Bridge.
- Repeat same logical send cannot produce two WhatsApp sends.
- Existing REST/MCP/Bridge behavior has regression coverage before rollout.
- Production state is NOT VERIFIED until separate deploy approval and runtime smoke.

## Next steps

Implement tested scoped API identity/permissions and the v1 send/status vertical slice, then wire real CRUD management and webhooks into the new page. Do not issue tokens or enable auto-group messaging based solely on this PRD.
