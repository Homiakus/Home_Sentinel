# Soak and fault-injection qualification

A release candidate must run **72 hours** on production-like hardware. This document defines the test, but a normal CI job does not pretend to have completed it.

Inject at least: camera disconnect/reconnect; Mosquitto restart; Frigate restart; Ollama timeout/hang; Telegram 429/5xx; backup destination loss; recording-disk pressure; server reboot; Internet loss while LAN remains healthy.

Record every 5 minutes: RSS, goroutines, file descriptors, queue depths, DB size, event/outbox backlog, camera health and recovery latency. The run fails on monotonic resource growth, missed critical recordings, unlock without authorization, unrecovered durable outbox, or recovery outside the degradation matrix.

Use `go test ./tests/fault/...` for the deterministic short fault suite before starting the hardware soak. Store the 72h report under `reports/soak-<version>.md`.
