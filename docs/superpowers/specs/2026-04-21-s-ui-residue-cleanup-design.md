# S-UI Residue Cleanup Design

## Goal

Make the repository and shipped product present as a complete `b-ui` project.
`s-ui` must remain only as a migration source during migration detection, import, and conversion. It must not remain as part of the default runtime, build output, package naming, user-visible branding, or operator-facing documentation after migration completes.

## Non-Goals

- Removing migration support from upstream `s-ui`
- Changing historical facts in changelogs or commit history
- Reworking unrelated repository structure
- Guaranteeing that historical install paths disappear immediately if path retention is still needed for migration compatibility

## Requirements

### Functional Requirements

1. New installs and normal updates must run as `b-ui` without depending on `s-ui` naming.
2. Migration flows must still detect upstream `s-ui` installs, data, and service state.
3. Migration must convert legacy `s-ui` state into `b-ui` runtime state.
4. After migration, the visible system state must be `b-ui`, including executable names, service names, cookie/session names, logs, package names, and operator documentation.
5. Any remaining `s-ui` references in the codebase must have an explicit migration-only reason.

### Compatibility Boundary

`s-ui` is allowed only as migration input. This includes:

- detecting legacy install roots when needed
- detecting and importing legacy databases such as `s-ui.db`
- detecting and replacing legacy service names, executable names, or configuration identifiers
- reading legacy runtime/session identifiers only long enough to transition them to `b-ui`

`s-ui` is not allowed as default steady-state output. This includes:

- default executable names
- default service names
- default cookie or session names
- default logger names
- package and artifact names
- default script output paths
- user-facing product naming in docs and scripts

## Current-State Findings

The repository currently contains both migration-compatible references and non-compatible residue.

### Migration-Compatible References That May Remain

- install root references such as `/usr/local/s-ui` when required to migrate an existing installation in place
- legacy database references such as `s-ui.db`
- legacy service detection such as `s-ui`

### Non-Compatible Residue To Remove Or Convert

- root-level local binary `sui`
- runtime executable naming such as `sui` and `sui.exe`
- service entrypoints targeting `/usr/local/s-ui/sui`
- frontend session cookie detection using `s-ui=`
- backend session store name `s-ui`
- backend logger name `s-ui`
- build outputs and packaging steps producing `sui`
- Windows installer and service scripts built around `sui.exe`
- docs and contributor guidance that still describe non-migration `s-ui` runtime naming

## Design

### Migration Model

The cleanup follows a strict two-stage model.

#### Stage 1: Migration Input Compatibility

Migration code may inspect legacy `s-ui` state and read from legacy sources. This includes old databases, old service units, old executable names, old cookies or sessions if required, and old installation layouts.

#### Stage 2: Post-Migration Standardization

Once migration succeeds, all resulting runtime state must be standardized to `b-ui`. The migrated installation must no longer depend on `s-ui` names to start, authenticate, log, update, package, or operate.

This means the project should behave as if it were originally installed as `b-ui`, even if the migration process briefly consumed upstream `s-ui` artifacts.

## Detailed Changes

### 1. Remove Local Binary Residue

- Delete the root-level ignored binary `sui`
- Keep repository ignore rules aligned with the new output names

This binary is a local build artifact and not part of the intended repository structure.

### 2. Standardize Executable Naming

Change the default built and packaged executable names from `sui` and `sui.exe` to `b-ui` and `b-ui.exe`.

This affects:

- local build scripts
- dev run scripts
- Linux packaging scripts
- Windows packaging scripts
- release workflows
- install and runtime scripts
- container entrypoints

Migration code may still detect old executable names long enough to replace or absorb them.

### 3. Standardize Service And Runtime Entry Points

Change service definitions and runtime scripts so the default launched executable is `b-ui`.

This includes:

- systemd `ExecStart`
- runtime management scripts
- container startup commands
- Windows service/install scripts

If the install root remains `/usr/local/s-ui` for compatibility, only the path may remain legacy. The executable within that location must default to `b-ui`, not `sui`.

### 4. Standardize Authentication And Session Naming

Change the default session and cookie names from `s-ui` to `b-ui`.

Compatibility rule:

- migration and transition logic may read old `s-ui` identifiers if needed
- default writes and steady-state reads must target `b-ui`

If temporary dual-read support is needed, it must be explicitly constrained to transition logic and must not redefine the default identity of the product.

### 5. Standardize Logging And Operator-Facing Labels

Change logger names and operator-facing identifiers from `s-ui` to `b-ui`.

This reduces confusion in logs, service status, operational support, and screenshots/documentation.

### 6. Standardize Packaging, CI, And Artifact Names

Build outputs, release packaging, and workflow steps must produce `b-ui`-named artifacts by default.

Examples:

- `build/out/b-ui` instead of `build/out/sui`
- packaged executable `b-ui`
- packaged Windows executable `b-ui.exe`

Tests and workflow assertions must be updated accordingly.

### 7. Preserve Migration-Only Legacy Detection

Migration paths must continue to support:

- legacy database discovery and conversion
- legacy service detection and replacement
- legacy install root probing where needed
- rollback and backup semantics for migration failures

These code paths should be documented and named as migration-only behavior, not default runtime behavior.

### 8. Clean Up Documentation

User-facing and contributor-facing docs must describe the product as `b-ui`.

Allowed `s-ui` references in docs are limited to:

- upstream attribution
- migration documentation
- explaining old-to-new conversion behavior

Disallowed doc references include instructions that imply normal `b-ui` operation still uses `s-ui` executable or service naming.

## File Areas Expected To Change

### Backend

- `src/backend/internal/infra/web/web.go`
- `src/backend/internal/infra/logging/logger.go`
- any migration-focused backend files that intentionally detect legacy `s-ui` state

### Frontend

- `src/frontend/src/router/index.ts`
- any auth/session-related frontend code that still assumes `s-ui`

### Runtime, Install, Packaging

- `src/services/systemd/b-ui.service`
- `src/services/runtime/b-ui.sh`
- `src/services/container/entrypoint.sh`
- `src/services/windows/*.bat`
- `src/services/windows/*.ps1`
- `scripts/build/build-backend.sh`
- `scripts/dev/run-local.sh`
- `scripts/release/package-linux.sh`
- `scripts/release/package-windows.ps1`
- `scripts/release/install.sh`
- release and Windows workflow files under `.github/workflows/`

### Docs And Tests

- `README.md`
- `docs/manual.md`
- `CONTRIBUTING.md`
- tests that assert package layout, executable names, or migration behavior

### Deferred Evaluation

- `go.mod`

The module path is currently still `github.com/alireza0/s-ui`. This may be retained temporarily if changing it would create disproportionate churn or break internal imports beyond the cleanup scope. The implementation must explicitly evaluate this point and either:

- migrate it now as part of the cleanup, or
- document why it is intentionally deferred as a separate follow-up

Leaving it unchanged without explicit evaluation is not acceptable.

## Error Handling

1. Migration must not leave the system in a mixed half-`s-ui`, half-`b-ui` steady state.
2. If migration fails, existing rollback and backup behavior must continue to protect user data.
3. New install and normal update paths must fail clearly if they unexpectedly depend on legacy naming.
4. Temporary compatibility reads must not silently become permanent defaults.

## Testing Strategy

### Automated Tests

- add or update tests for frontend authentication/session naming
- update script and packaging tests to assert `b-ui` executable naming
- update install and migration tests to verify migration still accepts `s-ui` input
- add or update tests ensuring post-migration output is `b-ui`

### Verification Scenarios

1. New install
Expected result: all visible runtime names are `b-ui`.

2. Normal update of an existing `b-ui` install
Expected result: update path does not require `s-ui` naming.

3. Migration from upstream `s-ui`
Expected result: legacy state is detected, converted, and the resulting installation operates as `b-ui`.

4. Packaging and release flow
Expected result: produced artifacts are named for `b-ui` and downstream scripts consume them correctly.

5. Failure and rollback during migration
Expected result: no mixed final state and existing rollback protections remain intact.

## Risks

1. Renaming executable outputs may break scripts, workflows, or tests that currently assume `sui`.
2. Session or cookie renaming may invalidate active sessions unless transition behavior is handled intentionally.
3. Windows scripts may contain multiple hard-coded references that need to change together.
4. The Go module path may create wider refactor scope if included now.

## Mitigations

1. Update tests and CI together with build output changes.
2. Scope any temporary dual-read compatibility to migration or transition only.
3. Verify Linux, Windows, and container entrypoints as one coordinated change.
4. Evaluate `go.mod` explicitly before implementation completes.

## Success Criteria

The cleanup is complete when all of the following are true:

1. The repository builds and packages `b-ui`-named runtime artifacts by default.
2. The default runtime state no longer depends on `s-ui` names.
3. The only remaining intentional `s-ui` references are clearly migration-only or upstream-attribution-only.
4. Migration from upstream `s-ui` still works and ends in a `b-ui` steady state.
5. Root-level local binary residue `sui` has been removed.
