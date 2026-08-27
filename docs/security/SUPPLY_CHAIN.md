# Supply-chain policy

## Rules

1. `go.mod` and `go.sum` are reviewed together; `go mod verify` is mandatory.
2. Production component images must be exact version/digest values. `latest`, empty fallbacks and mutable developer tags are forbidden by the packaging validator.
3. Security tooling is version-pinned in CI, not installed with `@latest`.
4. Every release produces a CycloneDX SBOM for Go modules and an image/filesystem SBOM where available.
5. `govulncheck` gates reachable Go vulnerabilities. Trivy scans the repository/container configuration and release image.
6. Release artifacts include checksums, build metadata, SBOMs and a qualification report.
7. The runtime container never mounts `/var/run/docker.sock`; updates are executed by an explicit host-side updater command.
8. Dependency updates are proposed by Dependabot and pass normal CI/security qualification before merge.
9. Every external GitHub Action is referenced by a reviewed 40-character commit SHA. Version comments are documentary only and never the execution ref.
10. Supply-chain policy is executable: `go run ./cmd/sentinel-supplychain --root .` must pass before security evidence is trusted.

## Pinned security tools

- `golang.org/x/vuln/cmd/govulncheck@v1.7.0`
- `github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@v1.10.0`
- Trivy `v0.74.0`
- Gremlins `v0.6.0`

Versions are intentionally explicit. Updating a scanner is a normal reviewed dependency change, not an implicit CI side effect.

## Current workflow contract

- `ci.yml` performs module hygiene, immutable-reference self-check, `govulncheck`, formatting, vet, unit, race, reconciliation and benchmark smoke.
- `security.yml` produces the Go module SBOM and performs Trivy filesystem vulnerability/misconfiguration/secret scanning.
- `mutation.yml` performs changed-critical-path mutation testing and stores mutation evidence.

The workflow files themselves are part of the trusted computing base and are checked by the supply-chain verifier before their outputs are treated as release evidence.
