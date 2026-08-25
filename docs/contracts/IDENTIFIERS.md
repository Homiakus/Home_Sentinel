# Identifier contract

Logical entities use stable opaque IDs. IP addresses, hostnames and display names are mutable attributes and must not be used as primary identity.

- `camera_id`: `cam_<26-char-crockford-base32>`
- `device_id`: `dev_<...>`
- `event_id`: `evt_<...>`
- `incident_id`: `inc_<...>`
- `request_id`: `req_<...>`
- `correlation_id`: `cor_<...>`

IDs are generated from cryptographic randomness and are safe for URLs/log correlation. They do not encode secrets or topology.
