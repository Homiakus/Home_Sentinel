# Release migration and rollback

## Invariants

1. An update plan is built before any mutation.
2. A verified rollback checkpoint is mandatory before staging.
3. Database migrations are forward-only in place. A database downgrade is performed by restoring the pre-update checkpoint, not by ad-hoc `DOWN` SQL.
4. Irreversible migrations are allowed only when the release manifest marks them and the operator has a checkpoint that can be restored by the previous release.
5. Runtime activation is verified through health/readiness before the update is committed.
6. A failed activation/verification triggers previous-runtime rollback plus checkpoint restore.

The Home Sentinel runtime container does not mount Docker's control socket. Host/container orchestration belongs to an explicit privileged updater process or operator command.

## Host configuration boundary

The critical checkpoint restores Sentinel-owned durable state (`sentinel.db`) and managed secret files. It does **not** overwrite the host release env file or bind-mounted `/opt/home-sentinel` deployment files. The host-side updater owns those release descriptors and switches back to `current-env`/old Compose when rollback is required. This keeps the runtime container unable to rewrite its own deployment authority.
