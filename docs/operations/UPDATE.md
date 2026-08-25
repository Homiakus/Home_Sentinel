# Updating Home Sentinel

The updater is a **host-side** executable. The Sentinel runtime never mounts `/var/run/docker.sock`.

## 1. Inventory the current deployment

```bash
sentinel-updater inventory \
  --env /opt/home-sentinel/releases/current.env \
  --system-url http://127.0.0.1:8080 \
  --out current-state.json
```

The env file is treated as data, not sourced by a shell. Every component image must be an exact non-`latest` tag or digest.

## 2. Plan

```bash
sentinel-updater plan --current current-state.json --manifest release-manifest.json
```

The plan checks Sentinel semver, database schema migration window and every component ref before making changes.

## 3. Apply

```bash
sentinel-updater apply \
  --current current-state.json \
  --manifest release-manifest.json \
  --current-env /opt/home-sentinel/releases/current.env \
  --target-env /opt/home-sentinel/releases/1.1.0.env \
  --compose /opt/home-sentinel/compose.prod.yml
```

Apply sequence:

1. create a consistent critical checkpoint from the currently running Sentinel;
2. pull target images;
3. activate the target Compose release;
4. wait for `/readyz`;
5. on activation/readiness failure, stop the target, restore the checkpoint using the **old image**, and start the old Compose release.

Database migrations are forward-only in place. For an irreversible schema change, rollback is defined as checkpoint restore, not a reverse migration. See `ROLLBACK.md`.
