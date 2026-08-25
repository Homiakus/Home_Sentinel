# Configuration ownership

Every managed configuration artifact has exactly one owner.

- Sentinel DB/config: Sentinel-managed.
- Generated Frigate sections: Sentinel-managed; manual zones are allowed only when explicitly marked external/manual.
- go2rtc stream aliases created by Sentinel: Sentinel-managed.
- HA `.storage`: never modified by Sentinel.
- HA MQTT Discovery entities: Sentinel-managed through retained discovery topics.
- Mosquitto base server config: deployment-managed; generated ACL fragment may be Sentinel-managed later.
- Ollama models: observed by Sentinel; installation is explicit user action.
- restic repository: restic-owned; scheduling/policy metadata is Sentinel-owned.
