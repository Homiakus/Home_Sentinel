# Implementation progress

## Current milestone

**Milestone:** P12 unified dashboard + managed live-view baseline
**Target toolchain:** Go 1.26.6
**Local verification fallback:** system Go 1.23.x + `sqlite_cgo` build tag because this execution environment cannot download the target toolchain/modules.

## Completed / baseline-complete

### P0–P5 — foundation and cameras

- **HS-001…HS-027** — architecture contracts, repository/CI, typed config, SecretRef, SQLite/WAL, migrations, repositories, revisions, transactional apply/rollback, resource locks, state bundle and application lifecycle.
- **HS-029…HS-032, HS-034** — API problem contract, users/sessions, RBAC, CSRF/security headers and bounded SSE.
- **HS-035…HS-042** — hardware inventory/recommendation.
- **HS-043…HS-053** — camera domain, credential rotation, WS-Discovery, ONVIF/media/PTZ, generic RTSP, safe ffprobe/snapshot, UVC and vendor fallback profiles.

### P6 — Frigate/go2rtc

- **HS-054…HS-063** — typed Frigate API client, version/capability gate, deterministic managed config, stream mapping, go2rtc stream generation, record/detect policies, preflight, atomic apply/rollback, event/media references and desired/actual drift detection.
- Camera credentials are materialized as managed `FRIGATE_SENTINEL_*` credential files for a read-only Frigate `/run/secrets` mount; generated config uses placeholders rather than plaintext passwords.

### P7 — MQTT/events

- **HS-064…HS-071** — Mosquitto baseline/ACLs, MQTT boundary, topic contract, external event adapters, bounded EventBus, durable transactional outbox, correlation engine and replay harness.
- Production runtime uses Eclipse Paho/autopaho behind a build-tagged integration boundary; the local old compiler tests the same public boundary through the compatibility implementation.

### P8 — Home Assistant

- **HS-072…HS-080** baseline — official REST/WebSocket clients, setup/probe service, MQTT integration verification, device-style MQTT Discovery, stable entity contract, allowlisted action bridge, generated file-managed HA package/dashboard and Frigate-in-HA verification.
- Sentinel does not write Home Assistant `.storage`.
- HA being unavailable at Sentinel startup is now **degraded**, not fatal to the control plane.

### P9 — DIY intercom

- **HS-081…HS-087** — versioned intercom protocol, ESP32 reference contract, button dedupe, independent per-state sequences, physical door/lock observed state, expiring single-use unlock command/ACK/result, media capability model and runtime incident correlation.
- HA receives observed intercom sensors only; there is intentionally no HA lock `command_topic` that could bypass Sentinel authorization/audit.
- **HS-088 Entrance UI** — baseline complete: same-origin managed live view, observed door/lock state, two-way-talk action for capable cameras and step-up protected unlock.

### P10 — local AI/VLM

- **HS-089…HS-098** baseline — `AIProvider`, Ollama native client, model inventory, hardware recommender, representative-frame selection, strict structured output schema, validation/confidence policy, bounded job queue, per-camera privacy and `sentinel-ai-eval` dataset harness.
- AI is outside the recording/unlock critical path.

### P11 — Telegram

- **HS-099…HS-102, HS-104…HS-106** baseline — thin Bot API client, numeric-ID pairing, RBAC binding, incident/doorbell notifications, opaque two-step unlock tokens, commands/status and retry/backoff/rate protection.
- Telegram transport errors are deliberately sanitized because the bot token is part of the Bot API request path.
- **HS-103 safe live/deep-link UX** — Sentinel now has a stable authenticated camera/entrance live URL suitable for Telegram deep links; richer snapshot/media message enrichment remains incomplete.

### P12 — unified Dashboard baseline complete

- **HS-107…HS-116** now have functional baselines: Overview, Camera Wall, Camera Detail, Event Feed, persistent Incident domain/timeline, unified local metadata search, dependency-aware System Status, Settings and Diagnostics UX.
- Camera Wall intentionally uses latest-frame previews; full live is opened only on camera/entrance pages.
- Browser live traffic goes through an authenticated Sentinel allowlisted go2rtc proxy; internal port `1984` is not published. Sentinel credentials are stripped before proxying.
- WebRTC candidates are managed in the Sentinel-owned Frigate config section; production Compose maps only media port `8555` TCP/UDP to an explicitly selected trusted LAN/VPN host IP.
- Settings can manage HA setup, Frigate plan/apply, Telegram pairing, local AI model inventory, backup operations and users/RBAC. The last enabled administrator cannot be demoted or disabled.
- Search currently exposes the supported local metadata index. Frigate semantic search remains optional/pending a selected stable public API adapter rather than relying on an internal endpoint.

### P13 — storage/backup/recovery

- **HS-117…HS-126** implementation baseline:
  - StoragePolicy + retention estimator + hysteretic Disk Guard;
  - restic command adapter without shell interpolation;
  - repository password supplied through a temporary `RESTIC_PASSWORD_FILE`, never argv;
  - critical/important/disposable backup-set separation;
  - scheduled critical backup;
  - retention preview/apply (`forget → prune → check`);
  - repository check;
  - sandbox restore-test;
  - consistent WAL-safe SQLite snapshot using `VACUUM INTO`;
  - SHA-256 manifest validation;
  - disaster-recovery documentation.
- Sentinel's own restic recovery password is excluded from the critical bundle when managed by the file-secret provider.
- The current execution environment does not contain a real `restic` binary, so command orchestration is contract-tested with a fake Runner; SQLite bundle/restore integrity paths execute against real SQLite locally.

### P14 — hardening baseline complete

- **HS-127** Sentinel-owned `nftables` policy generator + connection matrix + `nft -c` preflight. It intentionally does not replace the host's global INPUT policy; router/VLAN camera isolation remains operator-visible.
- **HS-128** SSRF guard — camera/RTSP/ONVIF destinations restricted to configured camera CIDRs with unsafe local/link-local destinations rejected.
- **HS-129** centralized redaction baseline for URL userinfo/query secrets and common secret key/value forms.
- **HS-130** append-oriented audit records for dangerous/configuration operations.
- **HS-131** 64 KiB JSON body limits + per-IP rate limits for login/bootstrap/pairing/unlock/reauth.
- **HS-132** step-up password reauthentication with a 15-minute sensitive-action window; applied to web unlock, HA credential reconfiguration and destructive backup retention.
- **HS-133** supply-chain automation: module verification, pinned security-tool versions, Go + image SBOM generation, govulncheck/Trivy gates, production image refs required explicitly, and release-only Dockerfile bases supplied as immutable `@sha256` refs.
- **HS-134** security regression baseline: auth bypass, CSRF/body-limit, SSRF, command-injection, secret-redaction, MQTT ACL, Telegram callback replay and unmanaged go2rtc proxy tests.

### P15 — observability/reliability baseline

- **HS-135** Unified Health Model and registry.
- **HS-136** Prometheus text exposition endpoint with aggregate HTTP/realtime/camera/backup/component metrics and no user/high-cardinality IDs.
- **HS-137** structured `slog` baseline.
- **HS-138** bounded watchdog with timeout and exponential backoff.
- **HS-139** dependency/root-cause diagnosis suppresses derivative HA errors when MQTT is the root failure.
- **HS-140 implementation baseline** — automated fault regression suite plus `sentinel-soak`, which produces JSON health/readiness/metrics evidence for configurable runs. The mandatory **real 72-hour HIL run is intentionally not marked complete in this sandbox**.

### P16 — install/update/release implementation baseline

- **HS-141** Update Manager: current inventory, exact-ref manifest, compatibility plan, checkpoint, stage/activate/readiness verification and automatic rollback. Update authority is host-side; Sentinel never mounts Docker socket.
- **HS-142** release migration policy: forward-only in-place migrations, migration-window checks and checkpoint restore for downgrade/irreversible rollback.
- **HS-143** Installation Wizard: readiness state in Dashboard plus explicit system verification that re-probes persisted camera streams, Frigate and enabled integration health. Physical camera/intercom event HIL remains an acceptance gate.
- **HS-144** production packaging: exact image env requirements, isolated control/app/camera networks, Compose secrets, private go2rtc API, explicit WebRTC bind, release-only Dockerfile with immutable base-image arguments.
- **HS-145** release qualification workflow: target Go race/vet/tests, security/fault suites, govulncheck, image vulnerability scan, Go/image SBOMs, release-image build and machine-readable qualification report.
- **HS-146 tooling/matrix**: expanded acceptance matrix and `sentinel-qualify` final gate. Production PASS remains blocked until real restic restore, camera/intercom/reboot HIL and 72-hour soak evidence are supplied.

## Intentionally pending

- **HS-028** final `chi` router dependency.
- **HS-033** final `templ + HTMX` shell. The embedded dependency-free UI is functional but is not falsely counted as this technology-specific task.
- Richer Telegram snapshot/media notification enrichment beyond the now-stable Sentinel live deep-link target.
- Optional Frigate semantic-search adapter once its public API contract is explicitly selected/version-gated.
- Real **72-hour HS-140 HIL soak execution** and its evidence.
- Final **HS-146 production acceptance evidence**: real restic restore, real ONVIF + RTSP cameras, physical intercom/lock, reboot recovery and 72-hour soak.

## Verification performed in this milestone

Using the environment-compatible `sqlite_cgo` target:

- AI/Ollama/evaluation packages — PASS.
- auth/RBAC/step-up reauthentication — PASS.
- SQLite migrations/repositories/state bundle — PASS.
- backup critical-bundle checksum/tamper/WAL snapshot tests — PASS.
- restic adapter argv/password handling contract tests — PASS.
- camera/ONVIF/UVC/RTSP tests — PASS.
- Frigate/go2rtc config/client/rollback tests — PASS.
- MQTT/event/outbox/correlation/replay tests — PASS.
- HA REST/Discovery/setup tests — PASS.
- intercom TTL/replay/sequence tests — PASS.
- Telegram pairing/action replay/API tests — PASS.
- incidents persistence tests — PASS.
- health/root-cause/watchdog tests — PASS.
- HTTP auth/CSRF/rate-limit/step-up handlers — PASS.
- P12 Dashboard aggregate/search/embedded-asset handlers — PASS.
- Managed go2rtc proxy allowlist and credential-stripping integration test — PASS.
- CSRF rotation and last-enabled-admin invariant — PASS.
- Frigate managed WebRTC candidate rendering — PASS.

The target-only Paho/coder-websocket/modernc compilation remains the responsibility of the Go 1.26.6 CI job because the current sandbox cannot fetch that toolchain/module graph. The Paho v0.23 API used by the production adapter was cross-checked against its tagged upstream source.

P12 also attempted a race-instrumented HTTP/auth/realtime run. Normal tests pass, but race-instrumented compilation exceeded the sandbox execution ceiling before producing a result, so `-race` remains an explicit target-CI requirement rather than being marked green here.
