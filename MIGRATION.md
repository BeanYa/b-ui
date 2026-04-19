# Migration From S-UI

This fork keeps the current Linux install layout compatible with upstream `s-ui`:

- service name: `s-ui`
- install directory: `/usr/local/s-ui`
- database path: `/usr/local/s-ui/db/s-ui.db`
- management command: `s-ui`

That means migration does not require exporting and re-importing data. The
recommended path is an in-place replacement of the installed files.

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

1. Detects an existing compatible `s-ui` installation.
2. Stops the `s-ui` service.
3. Creates a rollback backup under `/var/backups/s-ui/<timestamp>/`.
4. Downloads the release artifact from `BeanYa/b-ui`.
   The current Linux asset name is `b-ui-linux-<arch>.tar.gz`.
5. Replaces the installed binaries and shell script in place.
6. Runs `sui migrate`.
7. Restarts and enables the `s-ui` service.

## Notes

- Existing panel settings and admin credentials are preserved during
  `--migrate`.
- If the new build fails to start, the installer restores the previous
  installation from the rollback backup automatically.
- After migration, `s-ui update` and related shell actions point to this fork
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
