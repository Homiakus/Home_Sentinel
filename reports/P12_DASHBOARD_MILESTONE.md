# P12 Dashboard milestone report

## Implemented

- Overview aggregate API and page.
- Responsive Camera Wall using low-cost latest-frame previews.
- Camera detail live descriptor and authenticated go2rtc reverse proxy.
- Separate two-way-talk URL for capable cameras.
- Event feed and persistent incident timeline.
- Local unified metadata search.
- Dependency-aware System Status page.
- Camera/system diagnostics UI.
- Settings control surface for HA, Frigate, Telegram, AI, backup and users.
- User management with last-admin invariant.
- CSRF rotation after UI session restoration.
- Standard generic SSE message contract.
- Production WebRTC 8555 bind and generated candidates without exposing go2rtc HTTP port 1984.

## Deliberately not claimed complete

- `templ + HTMX` migration (HS-033) remains pending; the embedded UI is currently dependency-free HTML/CSS/ES modules.
- Frigate semantic search is not proxied because this milestone does not rely on an undocumented/internal search endpoint; local metadata search is the supported fallback.
- 72-hour real-camera soak and browser matrix are not available in the current execution environment.
- HTTPS termination/installer automation remains P16; two-way microphone access must be tested behind the production HTTPS endpoint.

## Verification executed in the sandbox

- `internal/config`, `internal/auth`, `internal/authz`, `internal/search` — PASS under the SQLite compatibility build.
- Frigate config/client/service and go2rtc mapping — PASS.
- `internal/httpserver`, `internal/app`, incidents/events/realtime — PASS.
- Home Assistant, intercom, Telegram, AI, backup, health/watchdog — PASS.
- database/repositories, all camera adapters and current security packages — PASS.
- `cmd/sentinel` local compatibility build — PASS.
- embedded `app.js` syntax check with Node — PASS.
- production Compose YAML structural validation — PASS; go2rtc HTTP port 1984 is not published.
- HTTP `-race` was attempted separately, but race-instrumented compilation exceeded this sandbox's execution limit; it is therefore **not** reported as passed and remains a target CI check.
