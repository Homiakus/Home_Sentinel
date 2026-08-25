# Trust zones

1. **Management zone** — Sentinel server and administrative interfaces.
2. **Trusted LAN/VPN clients** — authenticated users only.
3. **Camera VLAN** — untrusted devices; may initiate only required traffic toward the server/DNS/NTP policy.
4. **Container service network** — Frigate, HA, MQTT, Ollama, restic adapters; not exposed to WAN by default.
5. **External notification plane** — Telegram; never receives raw credentials or RTSP URLs.

Default network posture: deny camera Internet access and deny direct WAN access to Frigate internal API, go2rtc API, MQTT, Ollama and RTSP.
