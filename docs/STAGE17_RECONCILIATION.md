# Stage 17 — Evidence Reconciliation

Date: 2026-08-28
Observed commit: `5ccff8bf89508da86f82e77703f11586eb7bc8a0`

This document reconciles Stage 17 of `docs/AXIOM_IMPLEMENTATION_PLAN.md` against executable code/tests and the current GitHub Actions baseline. It is evidence, not a replacement for `MASTER_PLAN.md`.

Status semantics:

- **VERIFIED** — current production code has a concrete contract and executable evidence.
- **PARTIAL** — meaningful implementation exists, but the Stage 17 contract is broader than the evidence.
- **OPEN** — the required production contract is absent or not wired into the running application.

## Executive result

Stage 17 is not a monolithic unfinished stage. Callback security and durable callback semantics are substantially implemented, while several transport/runtime and principal/API contracts remain open. The stale status sentence that said callback binding, authorization-decision audit and exactly-once resume were not proven is no longer accurate.

## Clause matrix

| Stage 17 clause | Status | Executable evidence | Residual gap / task |
|---|---|---|---|
| `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout` | VERIFIED | `internal/httpserver/server.go` constructs `http.Server` with all four bounded values. | Keep regression coverage; no new implementation task. |
| max request/body size before decode | VERIFIED for JSON command bodies | shared `decodeBody` uses `http.MaxBytesReader(..., 64<<10)`; command handlers reuse it. | Future non-JSON upload endpoints need endpoint-specific budgets. |
| strict JSON command decode | VERIFIED | `decodeBody` uses `json.Decoder.DisallowUnknownFields`; no competing HTTP JSON decoder was found. | Preserve as central command decoder. |
| request ID | VERIFIED | request middleware generates and propagates `X-Request-ID`. | None. |
| actor identity | PARTIAL | cookie-authenticated requests resolve `auth.Principal`; callback ingress derives actor from signed human subject. | No explicit service/system principal type; T-014. |
| per-IP/per-principal rate limits | PARTIAL | login/setup/unlock/pairing use in-memory IP-scoped limiter. | No per-principal keying; no callback HTTP transport to rate-limit; T-015/T-013. |
| production remote plaintext forbidden | OPEN in runtime | `HardenedServerConfig.Validate` rejects non-loopback without TLS, but runtime `Config.Validate` only checks non-empty listen/timeouts and production server uses `ListenAndServe`. | F-006 / T-012: fail closed for remote runtime bind until TLS is actually wired. |
| pluggable local/OIDC/mTLS authenticator | OPEN | current browser auth is concrete local session/password implementation. | F-010 / T-017. |
| callback keyring verification | VERIFIED | callback runtime is loaded from secret references; `CallbackSecurity` exposes narrow `Accept`/`Sign`; callback work packets and tests cover key selection/rotation/failure. | External transport wiring still open. |
| replay guard before workflow mutation | VERIFIED at callback acceptance/orchestration boundary | callback `Acceptor` performs exact-binding/replay admission; callback work packet proves concurrent duplicate rejection. | HTTP transport must call this boundary, T-013. |
| viewer/operator/admin capability matrix | VERIFIED baseline | `internal/authz/rbac.go` grants explicit capabilities; high-risk callback resolution requires admin; unlock route requires `door:unlock` plus fresh auth. | Taxonomy does not model service/system/security-admin explicitly. |
| system/inference cannot obtain human authority | PARTIAL/structurally indirect | browser/callback human decisions require persisted `usr_*` users and RBAC; callback rejects non-human subjects. | No first-class principal kind makes the invariant explicit across future transports; F-008/T-014. |
| authorization decision audit trail | VERIFIED for callback decisions, PARTIAL globally | callback ingress records allow/deny and fails closed if allow-audit cannot be written. | generic HTTP `requireCapability` does not persist every allow/deny decision; F-009/T-015. |
| callback execution/node/event/action binding | VERIFIED | `CallbackIngress` builds fixed `NodeAwaitAck`/`NodeHumanDecision` bindings; signed subject cannot be replaced by body actor; negative binding tests exist. | External HTTP adapter absent; T-013. |
| actual waiting-node / stale callback safety | VERIFIED at orchestration boundary | stale-finish, retry-idempotency, durable integration and exact-once callback tests are present; high-risk callback API targets the fixed human node. | Expose only through authenticated transport. |
| medium-risk exactly-once redelivery/restart resume | VERIFIED | durable callback tests and work packet use ADGO event identity/SeenEvents semantic dedupe. | HTTP adapter must preserve event ID and not invent a second dedupe mechanism. |
| high-risk stale/already-resolved callback cannot transition twice | VERIFIED | callback runtime/integration tests include stale resolution safety. | Same transport requirement. |
| expected execution version/ETag stale-command protection | OPEN | repository search found no `If-Match`/ETag/expected-version command contract. | F-011/T-016. |
| `409 Conflict` on stale operator command | OPEN as general HTTP contract | orchestration can reject stale semantic actions, but there is no general HTTP version precondition contract. | T-016. |
| idempotency key on POST command endpoints | OPEN as general contract | durable workflows have internal idempotency identities, but HTTP POST commands do not expose a common `Idempotency-Key` contract. | T-016. |
| SameSite cookie + CSRF | VERIFIED | session cookie is HttpOnly + SameSiteStrict + configurable Secure; mutating browser routes use CSRF middleware. | Secure cookie remains necessary but cannot substitute for remote TLS. |
| CSP/security headers | VERIFIED | middleware sets CSP, frame denial, nosniff, referrer and permissions policy. | Maintain regression tests. |
| callback bearer independent of cookie/CSRF | VERIFIED at application boundary | callback work packet and `CallbackIngress` use signed token claims rather than browser session principal. | HTTP bearer adapter still absent, T-013. |
| callback secrets never forwarded/logged | VERIFIED baseline | application exposes a narrow callback-security interface; callback audit stores key ID/binding metadata but not token/key bytes; go2rtc proxy strips browser credentials. | Maintain secret-redaction tests. |

## Evidence clusters

### HTTP safety/browser auth

- `internal/httpserver/server.go`
- `internal/httpserver/auth_handlers.go`
- `internal/httpserver/middleware.go`
- `internal/httpserver/ratelimit.go`
- `internal/auth/session.go`
- `internal/auth/users.go`
- `internal/authz/rbac.go`

### Callback acceptance/runtime

- `internal/security/callback/*`
- `internal/app/callback_security.go`
- `internal/orchestration/incident/callback_ingress.go`
- `internal/orchestration/incident/callback_ingress*_test.go`
- `internal/orchestration/incident/callback_exactly_once_test.go`
- `internal/orchestration/incident/callback_durable_integration_test.go`
- `internal/orchestration/incident/callback_retry_idempotency_test.go`
- `internal/orchestration/incident/callback_stale_finish_test.go`
- `docs/engineering/work-packets/stage17-callback-acceptance.json`
- `docs/engineering/work-packets/stage17-callback-runtime.json`

### Durable notifier

- `internal/telegram/notifier.go`
- `internal/telegram/notifier_store.go`
- `internal/database/migrations/0008_notification_delivery.sql`
- `docs/engineering/work-packets/stage17-durable-notifier.json`

The notifier is a Stage 17 adjacent external-effect safety slice: semantic idempotency, frozen recipients, per-recipient receipts, crash-window ambiguity and no blind resend are already separately proven.

## New findings from reconciliation

### F-006 — Hardened remote-TLS rule is disconnected from production runtime

**Severity:** High  
**Confidence:** Confirmed

`HardenedServerConfig.Validate` correctly rejects a non-loopback bind without TLS, but the runtime `Config` consumed by `app.Open` has no TLS fields and `Config.Validate` accepts any non-empty listen address. `httpserver.Server.ListenAndServe` is plaintext. The security intent exists but is not enforced on the production path.

Decision: implement T-012 first as a fail-closed loopback-only runtime guard. Full TLS runtime support can then be introduced as a compatible extension; remote plaintext must not remain possible in the interim.

### F-007 — Authenticated callback semantics are not wired to an external HTTP transport

**Severity:** High  
**Confidence:** Strong

The repository has `CallbackSecurity` and a well-tested orchestration `CallbackIngress`, but production references to `CallbackIngress` are confined to its package/tests; the HTTP route table exposes no callback route. Therefore the secure boundary is real but not externally reachable as the Stage 17 ingress contract requires.

Decision: T-013 will add the narrow bounded/rate-limited bearer HTTP adapter only after T-012 closes remote plaintext exposure.

### F-008 — Principal model does not encode user/service/system authority kinds

**Severity:** High  
**Confidence:** Confirmed

`auth.Principal` is session-user specific and persisted roles are only viewer/operator/admin. Callback subjects are deliberately restricted to `usr_*`, which is safe, but the architecture has no first-class principal-kind type preventing a future service/system transport from being confused with a human authority principal.

Decision: T-014 introduces explicit principal kind/authority semantics before adding new machine transports or Scenario API authority.

### F-009 — Authorization audit is strong on callback path but not a global middleware contract

**Severity:** Medium/High  
**Confidence:** Confirmed

Callback allow/deny decisions are persisted and allowed callbacks fail closed if audit append fails. Generic HTTP `requireCapability` returns 403 but does not persist an authorization decision. The Stage 17 clause is therefore partial, not absent.

Decision: T-015 defines one bounded authorization-decision audit contract with safe failure semantics instead of ad-hoc logging.

### F-010 — Authentication is not pluggable

**Severity:** Medium  
**Confidence:** Confirmed

Browser authentication is tightly coupled to local password/session stores; no local/OIDC/mTLS authenticator interface exists outside the plan.

Decision: T-017, after principal semantics are explicit. Local auth remains the only supported mode until then.

### F-011 — HTTP command concurrency/idempotency contract is absent

**Severity:** High  
**Confidence:** Confirmed

No common `If-Match`/ETag/expected-version or `Idempotency-Key` command contract was found. Internal workflow idempotency is not equivalent to stale browser command protection.

Decision: T-016 defines the common HTTP precondition/idempotency boundary before Scenario API broadens the command surface.

## Dependency-aware residual Stage 17 sequence

```text
T-012 fail-closed remote runtime bind
   |
   +--> T-013 callback HTTP transport
   |
   +--> T-014 explicit principal kinds
           |
           +--> T-015 authorization audit + principal-aware limiting
           +--> T-017 pluggable authenticator boundary

T-014 + T-016 command concurrency contract --> authenticated Scenario API
```

T-004 schema/plan versioning may proceed in parallel after this reconciliation, but release qualification remains blocked on the Stage 17 P0 residuals.
