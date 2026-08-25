# P14/P16 security, installation and release milestone

Date: 2026-08-17

## Implemented

### P14 hardening completion

- Sentinel-owned `nftables` policy generator and connection matrix.
- Explicit `nft -c` preflight; no silent replacement of the host's global firewall policy.
- Supply-chain workflow with module verification, govulncheck, Trivy and CycloneDX SBOM generation.
- Release-only Dockerfile with mandatory immutable base-image inputs.
- Dedicated security regression package for auth bypass, CSRF/body limits, SSRF, shell injection, secret redaction, MQTT ACL, Telegram action replay and managed go2rtc proxy isolation.

### P15 qualification harness

- Fault regression package for root-cause suppression and bounded watchdog behavior.
- `sentinel-soak` health/readiness/metrics sampling tool suitable for the mandatory 72-hour HIL run.
- A 72-hour run itself was not fabricated in the sandbox and remains external release evidence.

### P16 installation/update/release

- Dashboard setup wizard backed by actual readiness state.
- Explicit setup verification re-probes stored camera streams and Frigate and checks enabled integration health.
- `sentinel-updater inventory` derives Sentinel version/schema from the running service and component image refs from a host release env file.
- Compatibility planning includes semantic version, schema window and exact component reference checks.
- Update apply performs critical checkpoint → image stage → activate → readiness verify.
- Failed activation/readiness executes old-release checkpoint restore and old Compose restart.
- Database rollback policy is restore-based for downgrade/irreversible migrations; reverse migrations are not invented.
- Production Compose uses exact image variables and does not mount the Docker socket.
- Release image build has a dedicated `Dockerfile.release` whose two base images must be supplied as immutable `@sha256` refs.
- `release-qualification` workflow builds/tests/scans/SBOMs the candidate and produces a machine-readable qualification report.
- `sentinel-qualify` refuses READY when any mandatory check is PENDING/FAIL/SKIP.
- Acceptance matrix now explicitly requires real restic restore, real ONVIF + RTSP cameras, physical intercom/lock, host reboot recovery and 72-hour soak evidence.

## Verification executed in this environment

PASS groups using the environment-compatible `sqlite_cgo` path:

- config/database/repositories/backup;
- update/release/setup/firewall/soak;
- all operational CLI packages;
- HTTP/auth/RBAC/security/fault/watchdog;
- camera/ONVIF/UVC/RTSP;
- Frigate/go2rtc;
- HA/MQTT;
- intercom/incidents;
- AI/Ollama;
- Telegram.

Also verified:

- YAML syntax for security and release qualification workflows;
- current production Compose passes the repository packaging policy test;
- checkpoint export/restore works against real local SQLite;
- JSON state bundle accepts SQLite TEXT or BLOB storage for JSON payloads;
- release manifest example passes strict parsing and exact-reference validation.

## Not falsely claimed as executed

The current sandbox does not have Docker, nftables or a real restic binary/remote repository and does not provide the target Go 1.26.6 module/toolchain graph. Therefore the following remain release-runner/HIL evidence, despite tooling being implemented:

- actual `docker compose` production deployment;
- actual `nft -c` against the target host ruleset;
- release-only Docker image build from real pinned base digests;
- target `go test -race ./...` on Go 1.26.6;
- real restic repository restore;
- physical cameras/intercom/door lock;
- reboot/internet-loss HIL;
- mandatory 72-hour soak.

The qualification gate intentionally remains BLOCKED until those mandatory evidence records exist.
