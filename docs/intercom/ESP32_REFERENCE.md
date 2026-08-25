# ESP32 intercom reference behavior

1. Connect to Mosquitto with a device-specific username/ACL and LWT `state/availability=offline` retained.
2. On connect publish `online` retained and fresh door/lock states.
3. Debounce the physical button locally; publish one non-retained `event/button` with monotonic `sequence`.
4. Subscribe only to `sentinel/intercom/<device_id>/command/#`.
5. Before executing `unlock`, validate schema, device-local policy, `expires_at`, and `request_id` replay cache.
6. Publish `event/ack` before actuation and `event/result` afterwards.
7. Drive the relay for a configured bounded pulse. Never use a permanent MQTT `unlocked` command state.
8. Publish door/lock sensors independently of command success.
9. After reboot clear transient relay state, reconnect, and publish observed physical state.
