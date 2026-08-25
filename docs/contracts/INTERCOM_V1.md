# Intercom protocol v1

## Topics

- `sentinel/intercom/<id>/state/availability` — retained observed state (`online|offline`).
- `sentinel/intercom/<id>/state/door` — retained observed state (`open|closed|unknown`).
- `sentinel/intercom/<id>/state/lock` — retained observed state (`locked|unlocked|unknown`).
- `sentinel/intercom/<id>/event/button` — transient button press.
- `sentinel/intercom/<id>/event/ack` — command acknowledgement.
- `sentinel/intercom/<id>/event/result` — command execution result.
- `sentinel/intercom/<id>/command/unlock` — transient unlock command; **never retained**.

Every JSON payload carries `schema_version: 1`. Commands additionally carry a unique `request_id`, `correlation_id`, `issued_at` and `expires_at`. A controller MUST keep a bounded cache of processed request IDs and MUST reject expired or duplicate commands before actuating the relay.

## Safety contract

An MQTT publish or ACK means only that the command was transported/accepted. It does **not** prove that the door opened. Physical door and lock state are separate observed-state topics and stale state is represented as unknown/degraded by Sentinel.
