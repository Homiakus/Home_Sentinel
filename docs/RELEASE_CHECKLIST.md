# Home Sentinel release checklist

A tagged production release is **not qualified** by unit tests alone.

## Automated gates

- [ ] `.go-version` target builds on the release runner.
- [ ] `gofmt`, `go vet`, `go test -race ./...` pass.
- [ ] security and fault regression suites pass.
- [ ] `govulncheck` has no actionable reachable vulnerabilities.
- [ ] production Compose passes packaging policy tests.
- [ ] release Sentinel image is built from immutable `@sha256` base images.
- [ ] release image HIGH/CRITICAL vulnerability scan passes according to policy.
- [ ] Go SBOM is generated and attached.
- [ ] critical SQLite checkpoint export/restore integration test passes.

## Mandatory production-like evidence

- [ ] A **real restic repository** backup + check + sandbox restore succeeded.
- [ ] At least one ONVIF camera completed discovery → profile → live → recording → detection.
- [ ] At least one generic RTSP camera completed manual onboarding → live → recording → detection.
- [ ] Physical doorbell press → incident → notification path succeeded.
- [ ] Unlock confirmation → command ACK → observed door transition succeeded; replay/expired command failed safely.
- [ ] Host reboot recovered Sentinel, MQTT, Frigate and configured integrations without manual DB repair.
- [ ] Internet outage did not stop local recording/intercom operation.
- [ ] 72-hour production-like soak passed the criteria in `docs/testing/SOAK.md`.

The `release-qualification` workflow deliberately fails the final gate when any mandatory external evidence input is missing. A generated report with `BLOCKED` is useful evidence, but it is **not** a release approval.
