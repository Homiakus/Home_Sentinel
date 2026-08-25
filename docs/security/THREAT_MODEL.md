# Threat model v1

## Protected assets

Camera credentials, live/archive video, door-control authority, HA token, MQTT credentials, Telegram binding, backup credentials and configuration history.

## Primary adversarial scenarios

1. Compromised IP camera attempts lateral movement.
2. Malicious camera URL attempts SSRF into management services.
3. Stolen web/Telegram session attempts door unlock.
4. Leaked token appears in logs, API or audit.
5. Crafted media endpoint attempts command injection.
6. MQTT client publishes an unauthorized lock command.
7. Replay of an old unlock/callback command.
8. Corrupt or hostile external-service payload reaches internal domain logic.
9. Disk exhaustion disables recording/system services.
10. Broken update/configuration makes the NVR unavailable.

## Baseline controls

- Camera VLAN and explicit network allowlists.
- No shell interpolation for external inputs.
- Opaque secret references and centralized redaction.
- RBAC + step-up confirmation for dangerous actions.
- Expiring, single-use command IDs and audit correlation.
- MQTT identities/ACL per component.
- Typed adapters and payload validation at trust boundaries.
- Plan/validate/apply/verify/rollback configuration lifecycle.
- Storage guard and independently tested backups.
