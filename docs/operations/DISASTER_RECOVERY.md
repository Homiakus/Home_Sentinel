# Home Sentinel disaster recovery

## Recovery invariant

A backup is **not** considered verified because `restic backup` succeeded. A usable recovery point requires both:

1. an encrypted restic snapshot of the Sentinel critical bundle; and
2. a successful Sentinel `restore-test` that verifies the bundle manifest and opens the restored SQLite database with `PRAGMA integrity_check`.

The restic repository password/recovery credential must be stored outside the repository it unlocks. Do not make the only copy of the password another file inside the same Sentinel backup.

## What the critical bundle contains

- a consistent SQLite snapshot produced with SQLite `VACUUM INTO` while the live database may be in WAL mode;
- a logical, schema-versioned state export;
- explicitly configured Sentinel/component configuration files;
- managed file secrets required to reconstruct integrations;
- `manifest.json` with SHA-256 and size for every payload file.

The secret that unlocks the backup repository itself is explicitly excluded when it is managed by Sentinel's file-secret provider.

Continuous Frigate recordings are **not** part of the default critical backup set. Important clips/snapshots require a separate policy so multi-terabyte video does not silently enter configuration backup jobs.

## Routine verification

1. Run a critical backup.
2. Record the returned snapshot ID.
3. Run repository `check` on schedule.
4. Run `restore-test` against a recent snapshot.
5. Treat `last successful backup` and `last successful restore-test/check` as separate health signals.

## Clean-host recovery procedure

1. Prepare a clean supported Debian/Ubuntu host.
2. Restore the off-host recovery credential from the administrator's protected recovery location.
3. Configure the restic repository and password `SecretRef`.
4. Restore the selected restic snapshot into an empty recovery directory.
5. Verify `manifest.json` before moving any file into a live location.
6. Open the restored `state/sentinel.db` read-only first and run `PRAGMA integrity_check`.
7. Restore managed secrets with owner-only permissions.
8. Restore explicitly backed-up configuration files.
9. Start Sentinel with external integrations disabled if necessary and verify `/readyz` plus `/api/v1/health`.
10. Re-enable MQTT/Frigate/Home Assistant/AI/Telegram one dependency at a time and verify observed state.
11. Perform a new backup after recovery and retain the recovery report.

## Do not

- overwrite the only live database before validating the restored copy;
- expose the restic password on a command line;
- copy a live WAL database with a plain filesystem copy and call it consistent;
- automatically restore camera credentials into an untrusted machine;
- prune the repository immediately before or during a recovery exercise.
