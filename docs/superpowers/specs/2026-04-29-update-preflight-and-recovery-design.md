# Update Preflight Check & Manual-Update Recovery

## Problem

After pushing a tag, GitHub Actions builds release artifacts across 7 platforms. The build takes time, and the GitHub Release API may report the release before all platform assets are uploaded. When auto-update triggers:

1. `fetchLatestReleaseTag()` gets the tag from the Release API
2. `StartUpdate()` launches the install script
3. `install.sh` tries to download the tarball but the asset isn't ready yet
4. After 8 retries × 15s (2 minutes), the download fails

Additionally, when an auto-update fails and the user completes the update manually (e.g. running `install.sh` directly), the cluster member status remains "offline" and the update state file still shows "failed" after restart.

## Design

### Part 1: Preflight Asset Check

Add a `preflight` phase to the update state machine. Before launching the detached systemd install unit, synchronously poll the release asset via HTTP HEAD to confirm it's downloadable.

**State machine change:**

```
Before: running → completed / failed
After:  preflight → running → completed / failed
                   ↓
                 failed (preflight_timeout)
```

**`StartUpdate()` flow:**

1. `compareVersions()` — confirm update is needed
2. `saveState("preflight")` — write preflight state, visible to frontend
3. `preflightReleaseAsset()` — HEAD poll, wait for asset
   - Success → `saveState("running")` → `launchDetachedPanelUpdate()`
   - Timeout → `saveState("failed", "preflight_timeout")`

**`preflightReleaseAsset()` implementation:**

- Detect current platform arch (port arch mapping from install.sh to Go)
- Build URL: `https://github.com/BeanYa/b-ui/releases/download/{version}/b-ui-linux-{arch}.tar.gz`
- HTTP HEAD request, 30s interval, max 15 minutes (30 attempts)
- Update `state.UpdatedAt` on each poll attempt so frontend can see the process is alive
- HTTP 200 = asset ready; 404 or other = keep waiting
- On timeout or non-recoverable error, return failure

**Arch mapping (Go):**

Port the arch detection logic from install.sh. Map `runtime.GOARCH` to the tarball suffix:

| runtime.GOARCH | Tarball suffix |
|---|---|
| amd64 | amd64 |
| arm64 | arm64 |
| arm (GOARM=7) | armv7 |
| arm (GOARM=6) | armv6 |
| arm (GOARM=5) | armv5 |
| 386 | 386 |
| s390x | s390x |

### Part 2: Manual-Update Recovery

When b-ui starts after a manual update that completed a previously failed auto-update, the system must detect the recovery and re-report online status.

**Current behavior:**

`reconcilePanelUpdateStateWithCurrentVersion()` already clears `failed` state when current version >= target version. But it doesn't trigger cluster status reporting.

**Change:**

- `reconcilePanelUpdateStateWithCurrentVersion()` returns an additional `recovered` bool indicating whether a failed state was cleared due to the current version reaching the target
- `GetUpdateInfo()` propagates this signal via `PanelUpdateInfo`
- `CheckAndBroadcastUpdate()` and `HandlePanelUpdateAvailable()` check for recovery and call `markLocalMemberOnline()` + `publishPanelUpdateStatus(online)` when detected

This covers:

- Auto-update failed → user manually installs → b-ui restarts → status auto-recovers
- Completion watch timed out but install actually succeeded → next poll cycle recovers

## Files to Modify

| File | Changes |
|---|---|
| `panel_update.go` | Add `preflightReleaseAsset()`, arch mapping, update state machine, add `recovered` to `PanelUpdateInfo` and reconcile return |
| `panel_update_test.go` | Tests for preflight logic and recovery detection |
| `cluster_sync.go` | Check `recovered` signal in `CheckAndBroadcastUpdate()` and `HandlePanelUpdateAvailable()`, trigger online status reporting |

## Constraints

- Preflight is synchronous in `StartUpdate()` — acceptable because auto-update is triggered from background polling, not user-facing API hot path
- Preflight only applies to Linux (same as the existing update capability check)
- The install.sh retry mechanism remains as a safety net for edge cases where preflight passes but the actual download fails (e.g. asset replaced, network issue)
