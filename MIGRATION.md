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
bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/migrate-to-b-ui.sh) v1.4.1
```

## What the migration script does

1. Detects an existing compatible `s-ui` installation.
2. Stops the `s-ui` service.
3. Creates a rollback backup under `/var/backups/s-ui/<timestamp>/`.
4. Downloads the release artifact from `BeanYa/b-ui`.
5. Replaces the installed binaries and shell script in place.
6. Runs `sui migrate`.
7. Restarts and enables the `s-ui` service.

## Notes

- Existing panel settings and admin credentials are preserved during
  `--auto-migrate`.
- If the new build fails to start, the installer restores the previous
  installation from the rollback backup automatically.
- After migration, `s-ui update` and related shell actions point to this fork
  instead of the upstream repository.
