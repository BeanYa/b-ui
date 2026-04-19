# Migration From Upstream

This fork keeps the current Linux install layout compatible with the upstream release:

- service name after migration: `b-ui`
- install directory: `/usr/local/s-ui`
- database path after migration: `/usr/local/s-ui/db/b-ui.db`
- management command after migration: `b-ui`

When an existing upstream database is present at `/usr/local/s-ui/db/s-ui.db`
and `b-ui.db` does not exist yet, the application migrates the legacy database
content into `b-ui.db` automatically before continuing. The recommended path is
an in-place replacement of the installed files followed by an explicit update
check to the latest published `b-ui` release.

## One-line migration

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/migrate-to-b-ui.sh)
```

To migrate to a specific release:

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/migrate-to-b-ui.sh) v0.0.1
```

The migration helper is a thin wrapper around:

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) --migrate
```

## What the migration script does

1. Detects an existing compatible upstream installation.
2. Stops the legacy `s-ui` service.
3. Creates a rollback backup under `/var/backups/s-ui/<timestamp>/`.
4. Downloads the release artifact from `BeanYa/b-ui`.
   The current Linux asset name is `b-ui-linux-<arch>.tar.gz`.
5. Replaces the installed binaries and shell script in place.
6. Runs `sui migrate`.
   If only the legacy `s-ui.db` exists, it is migrated to `b-ui.db` first.
7. Switches the systemd service name from `s-ui` to `b-ui`.
8. Switches the management command from `s-ui` to `b-ui`.
9. When no version is specified, performs an explicit update check against the latest published `b-ui` release.
10. Restarts and enables the `b-ui` service.

## Notes

- Existing panel settings and admin credentials are preserved during
  `--migrate`.
- Existing inbounds, outbounds, ports, and other persisted panel data are
  carried forward into `b-ui.db` automatically when the legacy database is the
  only database present.
- If the new build fails to start, the installer restores the previous
  installation from the rollback backup automatically.
- Without an explicit version argument, migration targets the latest published
  `b-ui` release.
- After migration, `b-ui update` and related shell actions point to this fork
  instead of the upstream repository.

## Update Modes

After migration, the install script supports explicit update modes:

```sh
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) --update
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) --force-update
```

- `--update`: update only when the installed version differs from the target release
- `--force-update`: reinstall the target release even when the version already matches
- Both modes accept an optional version, for example `--update v0.0.1`
