# Home Sentinel v1 acceptance matrix

Every mandatory item requires concrete evidence (CI run, HIL report, restore report, screenshot/log reference or signed operator record). `PENDING` and `SKIP` never count as production PASS.

## Core / recovery

- [ ] Go 1.26.6 build, vet and race tests pass.
- [ ] Fresh database migrates to the release schema and a second migration run is idempotent.
- [ ] Desired/applied/observed state remain distinct after restart.
- [ ] Critical checkpoint export and restore reproduce resources, revisions and managed secrets.
- [ ] Real restic backup, repository check and sandbox restore-test pass.
- [ ] Host reboot recovers the stack automatically.

## Cameras / NVR

- [ ] Real ONVIF camera is discovered and its device/media profiles are read.
- [ ] Real generic RTSP camera is manually onboarded.
- [ ] Camera credentials do not appear in API/UI/log/audit output.
- [ ] Snapshot/media decode succeeds before provisioning is accepted.
- [ ] go2rtc restream is reachable through Sentinel but internal API port 1984 is not exposed to clients.
- [ ] Frigate recording continues while HA, Ollama and Telegram are stopped.
- [ ] Detection event reaches the normalized Sentinel EventBus and incident store.
- [ ] Frigate invalid config apply returns to last-known-good configuration.

## Home Assistant

- [ ] HA REST and WebSocket connectivity recover after HA restart.
- [ ] MQTT Discovery creates stable Sentinel entities without duplicates.
- [ ] No `.storage` mutation is used.
- [ ] HA outage degrades integration health but does not stop NVR/control-plane startup.

## Intercom

- [ ] Physical button produces exactly one logical press under contact bounce/QoS redelivery.
- [ ] Door and lock observed states are independent of command ACK.
- [ ] Unlock requires authorized confirmation and expires.
- [ ] Replayed/expired unlock command or callback is rejected.
- [ ] Doorbell + recent person + unlock + door transition correlate into one entrance incident.

## AI / Telegram

- [ ] Ollama failure does not stop recording, Frigate ingestion or unlock authorization.
- [ ] VLM receives bounded representative frames, not the full stream.
- [ ] Invalid structured AI output cannot drive machine actions.
- [ ] Telegram pairing is bound to numeric user ID and Sentinel RBAC.
- [ ] Telegram unlock uses two-step, expiring, single-use opaque confirmation tokens.
- [ ] Telegram outage is isolated and retries are bounded.

## Security / network

- [ ] Cameras are isolated by VLAN/router policy from WAN unless explicitly required.
- [ ] Sentinel-generated nftables policy validates with `nft -c` before apply.
- [ ] Internal MQTT/go2rtc/Ollama/Frigate API ports are not host-published.
- [ ] SSRF, CSRF, auth bypass, command injection, secret leakage and Telegram replay regression tests pass.
- [ ] No Docker socket is mounted into Sentinel.
- [ ] Production component images are exact refs and release base images are immutable `@sha256` refs.
- [ ] SBOM and vulnerability reports are attached to the release qualification.

## Operations / UX

- [ ] Installation wizard shows actual readiness and at least one provisioned camera before completion.
- [ ] Dashboard exposes overview, camera wall/detail, entrance, events/incidents, search, diagnostics and settings.
- [ ] Last enabled administrator cannot be disabled/demoted.
- [ ] Dangerous actions require RBAC + CSRF + fresh step-up authentication.
- [ ] `sentinel-updater plan` reports component/schema changes before apply.
- [ ] Failed update restores the pre-update checkpoint and old Compose release.
- [ ] 72-hour production-like soak passes `docs/testing/SOAK.md`.

## Final gate

The release is accepted only when `sentinel-qualify` reports **READY** and every mandatory external HIL/restore/soak evidence entry is present.
