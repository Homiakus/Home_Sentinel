# Event Envelope v1

Every external event is normalized before application logic consumes it.

Required fields: schema version, event ID, type, source, occurred time, received time, correlation ID, optional causation ID, severity and JSON payload.

Adapters own external payload parsing. Domain/application code must not depend on raw MQTT/Frigate/HA/Telegram payload formats.
