# Repository Restructure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reorganize B-UI into `src/backend`, `src/frontend`, `src/services`, `scripts`, `packaging`, `build`, and `dist` while preserving external install and release compatibility.

**Architecture:** Keep the Go module root at the repository root, move all source code into `src/`, centralize engineering entrypoints into `scripts/`, and retain thin root-level compatibility wrappers for `install.sh` and `b-ui.sh`. The first pass prioritizes path consolidation and workflow simplification over behavioral rewrites, so verification focuses on builds, tests, and packaging smoke checks rather than new product behavior.

**Tech Stack:** Go 1.25, Gin, Gorm, Vue 3, Vite, Vitest, shell scripts, PowerShell, GitHub Actions

**Branch Strategy:** Perform the entire restructure on a dedicated feature branch created from the current mainline branch. Execute the work with multiple subagent-driven workers in parallel where tasks are independent, keep all work off `main`, and only merge back after all stage validations pass and the branch has been reviewed.

---

## File Structure Map

## Branch Setup

- Create and use a dedicated branch for the restructure, for example `refactor/repository-layout`.
- Keep all refactor commits on that branch until the end-to-end verification in Task 6 passes.
- Merge the branch back to `main` only after the final verification and review are complete.
- Execute this plan with multiple subagent-driven workers, but only in parallel for tasks or subtasks that do not edit the same files.
- Group related tasks into larger stages, complete the full stage verification, and then create one commit for that stage.
- Do not commit partial stage work unless there is an explicit recovery need; the default is one commit per completed stage.

Start with:

```bash
git checkout -b refactor/repository-layout
git status
```

Expected: the branch is `refactor/repository-layout` and the working tree is ready for the restructure commits.

Execution discipline for all stages:

```text
Parallelize only independent work -> finish the whole stage -> run the stage verification -> commit the stage -> start the next stage
```

## Parallel Execution Model

- Use subagent-driven execution as the default implementation mode for this plan.
- Dispatch multiple implementer subagents in parallel only when their file scopes do not overlap.
- Keep review centralized: each completed task still needs spec review and code-quality review before it is counted inside a completed stage.
- Prefer stage-local worktree isolation or tightly scoped file ownership when two subagents could otherwise touch adjacent paths.

Recommended stage ownership:

- Stage 1: repository scaffolding and frontend relocation
- Stage 2: backend relocation and script/service/packaging centralization
- Stage 3: workflow rewiring, docs cleanup, and final verification

Recommended parallel splits inside stages:

- Stage 1: one subagent for layout scaffolding, one subagent for frontend move
- Stage 2: one subagent for backend tree move and imports, one subagent for scripts plus service assets plus packaging
- Stage 3: one subagent for GitHub Actions rewiring, one subagent for docs and obsolete entrypoint cleanup

Avoid parallel execution when:

- two tasks modify the same workflow file
- two tasks modify the same root wrapper or shared script
- backend import rewrites are still in progress and dependent scripts rely on the new paths

### Create

- `src/backend/cmd/b-ui/main.go`
- `src/backend/internal/app/`
- `src/backend/internal/cli/`
- `src/backend/internal/http/api/`
- `src/backend/internal/http/middleware/`
- `src/backend/internal/http/sub/`
- `src/backend/internal/domain/core/`
- `src/backend/internal/domain/config/`
- `src/backend/internal/domain/jobs/`
- `src/backend/internal/domain/services/`
- `src/backend/internal/infra/db/`
- `src/backend/internal/infra/logging/`
- `src/backend/internal/infra/network/`
- `src/backend/internal/infra/web/`
- `src/backend/internal/shared/util/`
- `src/frontend/`
- `src/services/systemd/`
- `src/services/windows/`
- `src/services/runtime/`
- `scripts/dev/run-local.sh`
- `scripts/build/build-frontend.sh`
- `scripts/build/build-backend.sh`
- `scripts/build/build-all.sh`
- `scripts/release/install.sh`
- `scripts/release/package-linux.sh`
- `scripts/release/package-windows.ps1`
- `scripts/migration/migrate-to-b-ui.sh`
- `scripts/ci/verify.sh`
- `scripts/ci/check-layout.sh`
- `packaging/docker/Dockerfile`
- `packaging/docker/Dockerfile.frontend-artifact`
- `packaging/release/`
- `packaging/install/`
- `build/out/.gitkeep`
- `build/tmp/.gitkeep`
- `build/reports/.gitkeep`
- `dist/release/.gitkeep`

### Move

- `frontend/**` -> `src/frontend/**`
- `api/*.go` -> `src/backend/internal/http/api/*.go`
- `middleware/*.go` -> `src/backend/internal/http/middleware/*.go`
- `sub/*.go` -> `src/backend/internal/http/sub/*.go`
- `service/*.go` -> `src/backend/internal/domain/services/*.go`
- `core/*.go` -> `src/backend/internal/domain/core/*.go`
- `config/*.go` -> `src/backend/internal/domain/config/*.go`
- `config/name` -> `src/backend/internal/domain/config/name`
- `config/version` -> `src/backend/internal/domain/config/version`
- `cronjob/*.go` -> `src/backend/internal/domain/jobs/*.go`
- `database/**/*.go` -> `src/backend/internal/infra/db/**`
- `logger/*.go` -> `src/backend/internal/infra/logging/*.go`
- `network/*.go` -> `src/backend/internal/infra/network/*.go`
- `web/web.go` -> `src/backend/internal/infra/web/web.go`
- `util/*.go` -> `src/backend/internal/shared/util/*.go`
- `app/*.go` -> `src/backend/internal/app/*.go`
- `cmd/*.go` -> `src/backend/internal/cli/*.go`
- `cmd/migration/*.go` -> `src/backend/internal/cli/migration/*.go`
- `b-ui.service` -> `src/services/systemd/b-ui.service`
- `windows/*` -> `src/services/windows/*`
- `entrypoint.sh` -> `src/services/runtime/entrypoint.sh`
- `Dockerfile` -> `packaging/docker/Dockerfile`
- `Dockerfile.frontend-artifact` -> `packaging/docker/Dockerfile.frontend-artifact`
- `migrate-to-b-ui.sh` -> `scripts/migration/migrate-to-b-ui.sh`

### Modify

- `go.mod`
- `README.md`
- `CONTRIBUTING.md`
- `MIGRATION.md`
- `install.sh`
- `b-ui.sh`
- `.github/workflows/release.yml`
- `.github/workflows/windows.yml`
- `.github/workflows/docker.yml`
- `src/frontend/package.json`
- `src/frontend/scripts/sync-web-html.mjs`
- moved Go files that import old root package paths

### Remove After Verification

- `build.sh`
- `runSUI.sh`
- `docker-build-test.sh`
- empty source directories left at repository root after `git mv`

## Stage 1: Repository Skeleton And Frontend Move

Stage scope:

- Task 1: scaffold the repository layout
- Task 2: move the frontend into `src/frontend`

Stage validation:

- `bash scripts/ci/check-layout.sh`
- `npm ci && npm run build && npm run test` in `src/frontend`

Stage commit target:

```bash
git add .gitignore scripts/ci/check-layout.sh build/out/.gitkeep build/tmp/.gitkeep build/reports/.gitkeep dist/release/.gitkeep src web/html
git commit -m "refactor: scaffold repo layout and move frontend"
```

## Task 1: Scaffold The New Repository Layout

**Files:**
- Create: `scripts/ci/check-layout.sh`
- Create: `build/out/.gitkeep`
- Create: `build/tmp/.gitkeep`
- Create: `build/reports/.gitkeep`
- Create: `dist/release/.gitkeep`
- Create: `src/`, `scripts/`, `packaging/`
- Modify: `.gitignore`

- [ ] **Step 1: Write the failing repository layout check**

Create `scripts/ci/check-layout.sh` with this content:

```bash
#!/usr/bin/env bash
set -euo pipefail

required_paths=(
  "src"
  "scripts/build"
  "scripts/ci"
  "scripts/dev"
  "scripts/migration"
  "scripts/release"
  "packaging/docker"
  "build/out"
  "build/tmp"
  "build/reports"
  "dist/release"
)

for path in "${required_paths[@]}"; do
  if [ ! -e "$path" ]; then
    echo "missing required path: $path" >&2
    exit 1
  fi
done

echo "repository layout check passed"
```

- [ ] **Step 2: Run the layout check and verify it fails before scaffolding**

Run: `bash scripts/ci/check-layout.sh`

Expected: FAIL with `missing required path:` because the new repository scaffolding does not exist yet.

- [ ] **Step 3: Create the new directory skeleton and tracking files**

Run:

```bash
mkdir -p src scripts/dev scripts/build scripts/release scripts/migration scripts/ci packaging/docker packaging/release packaging/install build/out build/tmp build/reports dist/release
touch build/out/.gitkeep build/tmp/.gitkeep build/reports/.gitkeep dist/release/.gitkeep
```

Update `.gitignore` so transient build output is ignored while the directory placeholders remain tracked:

```gitignore
build/out/*
!build/out/.gitkeep
build/tmp/*
!build/tmp/.gitkeep
build/reports/*
!build/reports/.gitkeep
dist/release/*
!dist/release/.gitkeep
```

- [ ] **Step 4: Re-run the layout check**

Run: `bash scripts/ci/check-layout.sh`

Expected: PASS with `repository layout check passed`.

- [ ] **Step 5: Mark Task 1 complete for Stage 1 aggregation**

```bash
git status --short
```

## Task 2: Move The Frontend Into `src/frontend`

**Files:**
- Move: `frontend/**` -> `src/frontend/**`
- Modify: `src/frontend/package.json`
- Modify: `src/frontend/scripts/sync-web-html.mjs`
- Test: `src/frontend/package.json` scripts via `npm run build` and `npm run test`

- [ ] **Step 1: Write the failing frontend sync path update**

Replace `src/frontend/scripts/sync-web-html.mjs` with this target content after the move:

```js
import { cpSync, existsSync, mkdirSync, readdirSync, rmSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const scriptDir = dirname(fileURLToPath(import.meta.url))
const frontendRoot = resolve(scriptDir, '..')
const distDir = resolve(frontendRoot, 'dist')
const outputDir = resolve(frontendRoot, '..', '..', 'web', 'html')

if (!existsSync(distDir)) {
  throw new Error(`Missing frontend build output: ${distDir}`)
}

mkdirSync(outputDir, { recursive: true })

for (const entry of readdirSync(outputDir)) {
  rmSync(resolve(outputDir, entry), { recursive: true, force: true })
}

for (const entry of readdirSync(distDir)) {
  cpSync(resolve(distDir, entry), resolve(outputDir, entry), { recursive: true })
}

console.log(`Synced ${distDir} -> ${outputDir}`)
```

- [ ] **Step 2: Create the target source directory and move the frontend tree**

Run:

```bash
mkdir -p src
git mv frontend src/frontend
```

Ensure `src/frontend/package.json` keeps the existing local commands and still uses the moved sync script:

```json
{
  "scripts": {
    "dev": "vite --host",
    "build:dist": "vue-tsc --noEmit && vite build",
    "build": "npm run build:dist && node ./scripts/sync-web-html.mjs",
    "build:embed": "npm run build",
    "test": "vitest run",
    "preview": "vite preview",
    "lint": "eslint . --fix --ignore-path .gitignore"
  }
}
```

- [ ] **Step 3: Run the frontend build to verify the moved sync path works**

Run: `npm ci && npm run build`

Workdir: `src/frontend`

Expected: PASS, with `web/html` refreshed from `src/frontend/dist`.

- [ ] **Step 4: Run the frontend test suite**

Run: `npm run test`

Workdir: `src/frontend`

Expected: PASS.

- [ ] **Step 5: Mark Task 2 complete for Stage 1 aggregation**

```bash
git status --short
```

## Stage 2: Backend Move And Script Centralization

Stage scope:

- Task 3: move the backend into `src/backend` and fix imports
- Task 4: centralize scripts, service assets, and packaging

Stage validation:

- `gofmt -w src/backend`
- `go test ./...`
- `bash scripts/build/build-frontend.sh`
- `bash scripts/build/build-backend.sh`
- `bash scripts/build/build-all.sh`

Stage commit target:

```bash
git add src/backend scripts src/services packaging install.sh b-ui.sh build dist go.mod go.sum
git commit -m "refactor: move backend and centralize scripts"
```

## Task 3: Move The Backend Into `src/backend` And Fix Imports

**Files:**
- Create: `src/backend/cmd/b-ui/main.go`
- Move: `app/*.go` -> `src/backend/internal/app/*.go`
- Move: `cmd/*.go` -> `src/backend/internal/cli/*.go`
- Move: `cmd/migration/*.go` -> `src/backend/internal/cli/migration/*.go`
- Move: `api/*.go` -> `src/backend/internal/http/api/*.go`
- Move: `middleware/*.go` -> `src/backend/internal/http/middleware/*.go`
- Move: `sub/*.go` -> `src/backend/internal/http/sub/*.go`
- Move: `service/*.go` -> `src/backend/internal/domain/services/*.go`
- Move: `core/*.go` -> `src/backend/internal/domain/core/*.go`
- Move: `config/*.go` and `config/{name,version}` -> `src/backend/internal/domain/config/`
- Move: `cronjob/*.go` -> `src/backend/internal/domain/jobs/*.go`
- Move: `database/**/*.go` -> `src/backend/internal/infra/db/**`
- Move: `logger/*.go` -> `src/backend/internal/infra/logging/*.go`
- Move: `network/*.go` -> `src/backend/internal/infra/network/*.go`
- Move: `web/web.go` -> `src/backend/internal/infra/web/web.go`
- Move: `util/*.go` -> `src/backend/internal/shared/util/*.go`
- Modify: all moved Go files that import old root package paths
- Test: `go test ./...`

- [ ] **Step 1: Write the new backend binary entrypoint**

Create `src/backend/cmd/b-ui/main.go` with this content:

```go
package main

import (
    "log"
    "os"
    "os/signal"
    "syscall"

    backendapp "github.com/alireza0/s-ui/src/backend/internal/app"
    backendcli "github.com/alireza0/s-ui/src/backend/internal/cli"
)

func runApp() {
    app := backendapp.NewApp()

    if err := app.Init(); err != nil {
        log.Fatal(err)
    }

    if err := app.Start(); err != nil {
        log.Fatal(err)
    }

    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGTERM)

    for {
        sig := <-sigCh
        switch sig {
        case syscall.SIGHUP:
            app.RestartApp()
        default:
            app.Stop()
            return
        }
    }
}

func main() {
    if len(os.Args) < 2 {
        runApp()
        return
    }

    backendcli.ParseCmd()
}
```

- [ ] **Step 2: Move backend packages with `git mv` into the target tree**

Run:

```bash
mkdir -p src/backend/cmd/b-ui src/backend/internal/http src/backend/internal/domain src/backend/internal/infra src/backend/internal/shared
git mv app src/backend/internal/app
git mv cmd src/backend/internal/cli
git mv api src/backend/internal/http/api
git mv middleware src/backend/internal/http/middleware
git mv sub src/backend/internal/http/sub
git mv service src/backend/internal/domain/services
git mv core src/backend/internal/domain/core
git mv config src/backend/internal/domain/config
git mv cronjob src/backend/internal/domain/jobs
git mv database src/backend/internal/infra/db
git mv logger src/backend/internal/infra/logging
git mv network src/backend/internal/infra/network
mkdir -p src/backend/internal/infra/web
git mv web/web.go src/backend/internal/infra/web/web.go
git mv util src/backend/internal/shared/util
rm -f main.go
```

- [ ] **Step 3: Update import paths in moved Go files**

Use these replacements across the moved backend files:

```text
github.com/alireza0/s-ui/app                           -> github.com/alireza0/s-ui/src/backend/internal/app
github.com/alireza0/s-ui/cmd                           -> github.com/alireza0/s-ui/src/backend/internal/cli
github.com/alireza0/s-ui/cmd/migration                 -> github.com/alireza0/s-ui/src/backend/internal/cli/migration
github.com/alireza0/s-ui/api                           -> github.com/alireza0/s-ui/src/backend/internal/http/api
github.com/alireza0/s-ui/middleware                    -> github.com/alireza0/s-ui/src/backend/internal/http/middleware
github.com/alireza0/s-ui/sub                           -> github.com/alireza0/s-ui/src/backend/internal/http/sub
github.com/alireza0/s-ui/service                       -> github.com/alireza0/s-ui/src/backend/internal/domain/services
github.com/alireza0/s-ui/core                          -> github.com/alireza0/s-ui/src/backend/internal/domain/core
github.com/alireza0/s-ui/config                        -> github.com/alireza0/s-ui/src/backend/internal/domain/config
github.com/alireza0/s-ui/cronjob                       -> github.com/alireza0/s-ui/src/backend/internal/domain/jobs
github.com/alireza0/s-ui/database                      -> github.com/alireza0/s-ui/src/backend/internal/infra/db
github.com/alireza0/s-ui/logger                        -> github.com/alireza0/s-ui/src/backend/internal/infra/logging
github.com/alireza0/s-ui/network                       -> github.com/alireza0/s-ui/src/backend/internal/infra/network
github.com/alireza0/s-ui/web                           -> github.com/alireza0/s-ui/src/backend/internal/infra/web
github.com/alireza0/s-ui/util                          -> github.com/alireza0/s-ui/src/backend/internal/shared/util
```

Key moved files should end up like this:

`src/backend/internal/app/app.go`

```go
package app

import (
    "log"

    "github.com/alireza0/s-ui/src/backend/internal/domain/config"
    "github.com/alireza0/s-ui/src/backend/internal/domain/core"
    "github.com/alireza0/s-ui/src/backend/internal/domain/jobs"
    "github.com/alireza0/s-ui/src/backend/internal/domain/services"
    "github.com/alireza0/s-ui/src/backend/internal/http/sub"
    "github.com/alireza0/s-ui/src/backend/internal/infra/db"
    "github.com/alireza0/s-ui/src/backend/internal/infra/logging"
    "github.com/alireza0/s-ui/src/backend/internal/infra/web"

    "github.com/op/go-logging"
)
```

`src/backend/internal/infra/web/web.go`

```go
import (
    "github.com/alireza0/s-ui/src/backend/internal/domain/config"
    "github.com/alireza0/s-ui/src/backend/internal/domain/services"
    "github.com/alireza0/s-ui/src/backend/internal/http/api"
    "github.com/alireza0/s-ui/src/backend/internal/http/middleware"
    "github.com/alireza0/s-ui/src/backend/internal/infra/logging"
    "github.com/alireza0/s-ui/src/backend/internal/infra/network"
)
```

`src/backend/internal/http/sub/sub.go`

```go
import (
    "github.com/alireza0/s-ui/src/backend/internal/domain/config"
    "github.com/alireza0/s-ui/src/backend/internal/domain/services"
    "github.com/alireza0/s-ui/src/backend/internal/http/middleware"
    "github.com/alireza0/s-ui/src/backend/internal/infra/logging"
    "github.com/alireza0/s-ui/src/backend/internal/infra/network"
)
```

- [ ] **Step 4: Run Go formatting and the backend test suite**

Run:

```bash
gofmt -w src/backend
go test ./...
```

Expected: PASS. If any package still imports the old root paths, `go test` should fail until those imports are fixed.

- [ ] **Step 5: Mark Task 3 complete for Stage 2 aggregation**

```bash
git status --short
```

## Task 4: Centralize Scripts, Service Assets, And Packaging

**Files:**
- Move: `b-ui.service` -> `src/services/systemd/b-ui.service`
- Move: `windows/*` -> `src/services/windows/*`
- Move: `entrypoint.sh` -> `src/services/runtime/entrypoint.sh`
- Move: `Dockerfile` -> `packaging/docker/Dockerfile`
- Move: `Dockerfile.frontend-artifact` -> `packaging/docker/Dockerfile.frontend-artifact`
- Create: `scripts/build/build-frontend.sh`
- Create: `scripts/build/build-backend.sh`
- Create: `scripts/build/build-all.sh`
- Create: `scripts/dev/run-local.sh`
- Create: `scripts/release/install.sh`
- Create: `scripts/release/package-linux.sh`
- Create: `scripts/release/package-windows.ps1`
- Create: `scripts/migration/migrate-to-b-ui.sh`
- Modify: `install.sh`
- Modify: `b-ui.sh`

- [ ] **Step 1: Move service and packaging assets into their new homes**

Run:

```bash
mkdir -p src/services/systemd src/services/runtime
git mv b-ui.service src/services/systemd/b-ui.service
git mv windows src/services/windows
git mv entrypoint.sh src/services/runtime/entrypoint.sh
git mv Dockerfile packaging/docker/Dockerfile
git mv Dockerfile.frontend-artifact packaging/docker/Dockerfile.frontend-artifact
git mv migrate-to-b-ui.sh scripts/migration/migrate-to-b-ui.sh
```

- [ ] **Step 2: Create the repository build scripts**

Create `scripts/build/build-frontend.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
frontend_root="$repo_root/src/frontend"

cd "$frontend_root"
npm ci
npm run build
```

Create `scripts/build/build-backend.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
output_path="$repo_root/build/out/sui"
build_tags="with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_naive_outbound,with_musl,badlinkname,tfogo_checklinkname0,with_tailscale"

cd "$repo_root"
go build \
  -ldflags '-w -s -checklinkname=0 -extldflags "-Wl,-no_warn_duplicate_libraries"' \
  -tags "$build_tags" \
  -o "$output_path" \
  ./src/backend/cmd/b-ui
```

Create `scripts/build/build-all.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)

bash "$repo_root/scripts/build/build-frontend.sh"
bash "$repo_root/scripts/build/build-backend.sh"
```

Create `scripts/dev/run-local.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)

bash "$repo_root/scripts/build/build-all.sh"
SUI_DB_FOLDER="db" SUI_DEBUG=true "$repo_root/build/out/sui"
```

- [ ] **Step 3: Create release and migration entrypoints**

Create `scripts/release/install.sh` by moving the current `install.sh` implementation into this file unchanged first, then only update internal paths in a later task if needed.

Create `scripts/migration/migrate-to-b-ui.sh` with this adjusted wrapper content:

```bash
#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
TARGET_VERSION="${1:-}"

args=(--migrate)
if [ -n "$TARGET_VERSION" ]; then
  args+=("$TARGET_VERSION")
fi

bash "$SCRIPT_DIR/../release/install.sh" "${args[@]}"

if [ -z "$TARGET_VERSION" ]; then
  bash "$SCRIPT_DIR/../release/install.sh" --update
fi
```

Create `scripts/release/package-linux.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
stage_dir="$repo_root/build/tmp/b-ui-linux"
archive_path="$repo_root/dist/release/b-ui-linux-amd64.tar.gz"

rm -rf "$stage_dir"
mkdir -p "$stage_dir/b-ui"

bash "$repo_root/scripts/build/build-all.sh"

cp "$repo_root/build/out/sui" "$stage_dir/b-ui/"
cp "$repo_root/src/services/systemd/b-ui.service" "$stage_dir/b-ui/"
cp "$repo_root/b-ui.sh" "$stage_dir/b-ui/"

tar -czf "$archive_path" -C "$stage_dir" b-ui
echo "created $archive_path"
```

Create `scripts/release/package-windows.ps1`:

```powershell
$ErrorActionPreference = 'Stop'

$RepoRoot = Resolve-Path "$PSScriptRoot\..\.."
$StageDir = Join-Path $RepoRoot 'build\tmp\b-ui-windows'
$ArchivePath = Join-Path $RepoRoot 'dist\release\b-ui-windows-amd64.zip'

if (Test-Path $StageDir) { Remove-Item $StageDir -Recurse -Force }
New-Item -ItemType Directory -Path (Join-Path $StageDir 'b-ui-windows') | Out-Null

Push-Location $RepoRoot
go build -o (Join-Path $RepoRoot 'build\out\sui.exe') ./src/backend/cmd/b-ui
Pop-Location

Copy-Item (Join-Path $RepoRoot 'build\out\sui.exe') (Join-Path $StageDir 'b-ui-windows\sui.exe')
Copy-Item (Join-Path $RepoRoot 'src\services\windows\*') (Join-Path $StageDir 'b-ui-windows') -Recurse

Compress-Archive -Path (Join-Path $StageDir 'b-ui-windows') -DestinationPath $ArchivePath -Force
Write-Host "created $ArchivePath"
```

- [ ] **Step 4: Replace the root wrappers with thin compatibility shims**

Set root `install.sh` to:

```bash
#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
exec bash "$SCRIPT_DIR/scripts/release/install.sh" "$@"
```

Set root `b-ui.sh` to:

```bash
#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
TARGET_SCRIPT="$SCRIPT_DIR/src/services/runtime/b-ui.sh"

if [ ! -f "$TARGET_SCRIPT" ]; then
  echo "missing runtime wrapper: $TARGET_SCRIPT" >&2
  exit 1
fi

exec bash "$TARGET_SCRIPT" "$@"
```

Move the original root `b-ui.sh` implementation into `src/services/runtime/b-ui.sh` without behavioral changes in this task.

- [ ] **Step 5: Verify script-based local builds and mark Task 4 complete for Stage 2 aggregation**

Run:

```bash
bash scripts/build/build-frontend.sh
bash scripts/build/build-backend.sh
bash scripts/build/build-all.sh
bash scripts/dev/run-local.sh --help || true
```

Expected: the build scripts pass and `build/out/sui` is produced.

## Stage 3: Workflow Rewiring, Docs Cleanup, And Final Verification

Stage scope:

- Task 5: rewire GitHub Actions to call repository scripts
- Task 6: update docs, remove obsolete entrypoints, and run final verification

Stage validation:

- `bash scripts/ci/verify.sh`
- `bash scripts/release/package-linux.sh`
- `pwsh -File scripts/release/package-windows.ps1`
- `go test ./...`

Stage commit target:

```bash
git add .github/workflows scripts/ci scripts/release README.md CONTRIBUTING.md MIGRATION.md install.sh b-ui.sh
git commit -m "ci: align workflows and docs with src layout"
```

## Task 5: Rewire GitHub Actions To Call Repository Scripts

**Files:**
- Modify: `.github/workflows/release.yml`
- Modify: `.github/workflows/windows.yml`
- Modify: `.github/workflows/docker.yml`
- Test: local dry-run equivalent commands from `scripts/ci/verify.sh`, `scripts/release/package-linux.sh`, and `scripts/release/package-windows.ps1`

- [ ] **Step 1: Create the CI verification entrypoint**

Create `scripts/ci/verify.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)

bash "$repo_root/scripts/ci/check-layout.sh"

cd "$repo_root/src/frontend"
npm ci
npm run lint
npm run test
npm run build

cd "$repo_root"
go test ./...
bash "$repo_root/scripts/build/build-backend.sh"
```

- [ ] **Step 2: Update the release workflow to call scripts**

Refactor `.github/workflows/release.yml` so the frontend job works from `src/frontend`, the backend build target is `./src/backend/cmd/b-ui`, and packaging delegates into repository scripts.

The critical command changes are:

```yaml
- name: Build frontend
  run: bash scripts/build/build-frontend.sh

- name: Verify repository
  run: bash scripts/ci/verify.sh

- name: Package Linux archive
  run: bash scripts/release/package-linux.sh
```

Also update any `cache-dependency-path` values from `frontend/package-lock.json` to `src/frontend/package-lock.json`.

- [ ] **Step 3: Update the Windows workflow to call scripts**

Change `.github/workflows/windows.yml` so the frontend artifact is built through `bash scripts/build/build-frontend.sh` and the Windows archive is assembled through PowerShell:

```yaml
- name: Build frontend
  run: bash scripts/build/build-frontend.sh

- name: Package Windows archive
  shell: pwsh
  run: ./scripts/release/package-windows.ps1
```

When downloading service files or packaging extras, use `src/services/windows/` instead of `windows/`.

- [ ] **Step 4: Update the Docker workflow to use moved paths**

Change `.github/workflows/docker.yml` so:

```yaml
- name: Build frontend
  run: bash scripts/build/build-frontend.sh

with:
  file: packaging/docker/Dockerfile.frontend-artifact
```

and ensure any workflow step that previously entered `frontend` now works through `src/frontend` or a repository script.

- [ ] **Step 5: Run local CI-equivalent commands and mark Task 5 complete for Stage 3 aggregation**

Run:

```bash
bash scripts/ci/verify.sh
bash scripts/release/package-linux.sh
pwsh -File scripts/release/package-windows.ps1
```

Expected: all commands pass and produce archives in `dist/release/`.

## Task 6: Update Docs, Remove Obsolete Entrypoints, And Run Final Verification

**Files:**
- Modify: `README.md`
- Modify: `CONTRIBUTING.md`
- Modify: `MIGRATION.md`
- Delete: `build.sh`
- Delete: `runSUI.sh`
- Delete: `docker-build-test.sh`

- [ ] **Step 1: Update repository documentation to the new layout and commands**

Refresh the docs so they reference:

```markdown
- Frontend source lives in `src/frontend/`
- Backend source lives in `src/backend/`
- Runtime service assets live in `src/services/`
- Repository automation lives in `scripts/`
- Docker and release assets live in `packaging/`

Common commands:

```sh
bash scripts/build/build-frontend.sh
bash scripts/build/build-backend.sh
bash scripts/build/build-all.sh
bash scripts/dev/run-local.sh
bash scripts/ci/verify.sh
```
```

Also update any existing references to `frontend/`, `main.go`, `build.sh`, `runSUI.sh`, `windows/`, `Dockerfile.frontend-artifact`, and `docker-build-test.sh`.

- [ ] **Step 2: Remove obsolete root implementation files once replacements are verified**

Run:

```bash
git rm build.sh runSUI.sh docker-build-test.sh
```

If any empty directories remain at the repository root after prior `git mv` operations, remove them after confirming they contain no tracked files.

- [ ] **Step 3: Run final end-to-end verification**

Run:

```bash
bash scripts/ci/check-layout.sh
bash scripts/ci/verify.sh
go test ./...
```

Run frontend verification explicitly as a final smoke check:

```bash
npm ci && npm run lint && npm run test && npm run build
```

Workdir: `src/frontend`

Expected: PASS across all commands.

- [ ] **Step 4: Review the final tree and confirm compatibility shims remain in place**

Run:

```bash
git status --short
```

Confirm these still exist at the repository root:
- `install.sh`
- `b-ui.sh`
- `go.mod`
- `go.sum`
- `README.md`

- [ ] **Step 5: Mark Task 6 complete and create the final Stage 3 commit**

```bash
git add README.md CONTRIBUTING.md MIGRATION.md install.sh b-ui.sh
git status --short
```

## Self-Review Checklist

- Spec coverage:
  - `src/backend`, `src/frontend`, `src/services`, `scripts`, `packaging`, `build`, `dist`, root wrappers, and CI delegation are all covered by Tasks 1-6.
  - Backend layering from the approved spec is represented by the move plan in Task 3, with one explicit implementation addition: `src/backend/internal/cli` to house the existing CLI package currently named `cmd`.
  - The existing `sub/` package is explicitly migrated into `src/backend/internal/http/sub`, which is required to complete the approved `src/backend` structure.
- Placeholder scan:
  - No unresolved placeholder steps remain.
- Consistency:
  - All build commands point to `./src/backend/cmd/b-ui`.
  - All frontend commands point to `src/frontend`.
  - All workflow rewiring points to `scripts/` and `packaging/docker/`.

## Merge Back To Main

- Execution mode for this plan: option 1, subagent-driven development with multiple parallel implementers where file ownership is independent.
- Commit mode for this plan: one commit per completed stage after the full stage validation passes.
- After Task 6 passes, push the feature branch and open a pull request or merge request into `main`.
- Do not fast-forward local experimental commits directly onto `main`; merge the verified branch back after review.
