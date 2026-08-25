# Runtime degradation policy

Home Sentinel distinguishes **fatal core dependencies** from **optional external integrations**.

Fatal during startup:

- invalid Sentinel configuration;
- inability to open/migrate the Sentinel database;
- invalid local trust-zone/network policy;
- corruption of local invariants required to authorize commands.

Non-fatal and represented through the health model:

- Home Assistant unreachable;
- Frigate configured but temporarily unreachable;
- MQTT broker disconnected after startup;
- Ollama/model unavailable;
- Telegram API unavailable;
- backup repository unavailable.

The purpose is to keep independent safety functions independent. In particular, an unavailable AI or notification provider must not turn a video-recording problem into a whole-system outage.
