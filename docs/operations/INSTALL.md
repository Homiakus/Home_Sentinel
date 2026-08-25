# Production installation

## Host prerequisites

- Debian/Ubuntu Linux host with Docker Engine + Compose plugin.
- A dedicated recordings disk is recommended.
- Trusted LAN/VPN address chosen for Sentinel UI and WebRTC media.
- Camera VLAN/CIDRs known before enabling discovery.
- MQTT password files created with owner-only permissions.
- Exact image references selected for every service.

## Bootstrap

1. Copy `deploy/compose/compose.prod.yml` and a reviewed `config.json` to `/opt/home-sentinel/`.
2. Create a release env file containing exact `SENTINEL_IMAGE`, `MOSQUITTO_IMAGE`, `HOMEASSISTANT_IMAGE`, `FRIGATE_IMAGE`, and `OLLAMA_IMAGE` refs plus bind/network variables.
3. Create the three MQTT secret files referenced by the env file.
4. Validate the network policy with `sentinel-firewall render` and `nft -c` before applying it.
5. Start the stack with `docker compose --env-file <release.env> -f compose.prod.yml up -d`.
6. Open Sentinel and create the bootstrap administrator.
7. Follow **Мастер настройки**. It checks actual readiness rather than only saved values.
8. Add at least one real camera and verify snapshot, live, recording and detection.
9. Configure HA, intercom, Ollama and Telegram as required.
10. Initialize backup, execute backup check and sandbox restore-test.
11. Complete `docs/ACCEPTANCE.md`; production release remains blocked until hardware, reboot and 72-hour soak evidence is recorded.

The installer deliberately does not edit router/VLAN settings or globally replace the host firewall. Those boundaries require operator-visible changes.
