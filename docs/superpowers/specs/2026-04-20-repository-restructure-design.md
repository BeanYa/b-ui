# B-UI Repository Restructure Design

Date: 2026-04-20
Status: Draft approved in conversation, written for review
Scope: Repository-wide restructuring for source layout, build flow, service assets, packaging, and CI organization

## 1. Goal

Restructure the B-UI repository into a clearer and more maintainable engineering layout that separates backend source, frontend source, runtime service assets, automation scripts, packaging assets, and documentation, while preserving external install and release compatibility.

The redesign must:
- consolidate scattered source code into `src/backend` and `src/frontend`
- introduce `src/services` for runtime and platform service assets
- centralize repository automation under `scripts/`
- separate packaging concerns into `packaging/`
- keep external installation commands and release asset naming compatible
- reduce path sprawl in CI, local development, and release flows

## 2. Context

The current repository mixes several concerns at the top level:
- Go backend packages live directly at the repository root
- the frontend lives in a separate `frontend/` tree beside backend packages
- root-level scripts handle unrelated duties such as development, build, migration, install, and packaging
- platform service assets are split across root files and `windows/`
- Docker and release packaging materials are not grouped by responsibility

This increases maintenance cost because structural changes require updates across many disconnected paths, and because the top-level directory does not communicate which files are source code versus delivery assets versus engineering tooling.

## 3. Constraints

### 3.1 Compatibility Constraints

The refactor should preserve external compatibility for user-facing installation and release behavior:
- `install.sh` remains available at the repository root
- existing installation and upgrade command shapes remain valid
- existing release asset naming remains unchanged
- user-facing migration and install behavior remains stable

The refactor may change internal repository paths and engineering entrypoints:
- CI workflow internals can be redesigned
- build scripts can move and be reorganized
- backend source locations can change
- frontend source location can change

### 3.2 Technical Constraints

- `go.mod` and `go.sum` stay at the repository root
- the Go module root remains the repository root
- the frontend remains an independent Vite project with its own package manifest
- the final built static frontend assets continue to land in root `web/`
- the first refactor phase should prioritize structural clarity over behavioral rewrites

## 4. Evaluated Approaches

### Approach A: Full internal reorganization with compatibility shims at root

Move source and engineering assets into a layered structure, while keeping thin root-level compatibility wrappers for user-facing entrypoints.

Pros:
- best long-term maintainability
- clear ownership by directory responsibility
- allows CI and packaging to be simplified around script entrypoints
- preserves user-facing compatibility

Cons:
- requires broad path updates
- needs disciplined staged rollout

### Approach B: Build and script cleanup only

Reorganize scripts, packaging, and CI while leaving most backend source directories where they are.

Pros:
- lower execution risk
- faster initial delivery

Cons:
- repository shape still remains confusing
- backend package sprawl remains unresolved
- likely leads to a second refactor later

### Approach C: Deep backend architecture rewrite first

Start by redesigning domain boundaries in the backend, then reorganize repository layout around that rewrite.

Pros:
- could produce a cleaner long-term architecture

Cons:
- too large for a first repository restructure pass
- mixes structural cleanup with behavioral redesign
- materially increases regression risk

### Recommendation

Adopt Approach A.

This matches the project goal: make source layout, scripts, build flow, and service assets coherent in one pass while preserving user-facing installation and release compatibility.

## 5. Target Repository Layout

```text
/
├─ src/
│  ├─ backend/
│  ├─ frontend/
│  └─ services/
├─ scripts/
├─ packaging/
├─ docs/
├─ web/
├─ build/
├─ dist/
├─ install.sh
├─ b-ui.sh
├─ go.mod
├─ go.sum
└─ README.md
```

### 5.1 Top-Level Directory Roles

- `src/backend`: all Go application source code
- `src/frontend`: the frontend Vite application and its local tooling
- `src/services`: runtime and platform service assets used by the product at install or deployment time
- `scripts`: repository automation entrypoints for development, build, release, migration, and CI
- `packaging`: Docker and release packaging assets, templates, and assembly metadata
- `docs`: design docs, migration docs, contributor docs, and repository guidance
- `web`: built static frontend assets consumed by the backend or release packaging
- `build`: local build outputs, temporary files, and reports
- `dist`: assembled release outputs

### 5.2 Root-Level Files Kept Intentionally

These stay at the repository root:
- `go.mod`
- `go.sum`
- `install.sh`
- `b-ui.sh`
- `README.md`
- `.github/`

Reasoning:
- the Go module root should remain stable
- user-facing install and command entrypoints need compatibility shims
- GitHub workflows naturally live at the root

## 6. Backend Structure

The backend should move under `src/backend` and stop exposing many unrelated packages directly from the repository root.

Target layout:

```text
src/backend/
├─ cmd/
│  └─ b-ui/
│     └─ main.go
├─ internal/
│  ├─ app/
│  ├─ http/
│  │  ├─ api/
│  │  └─ middleware/
│  ├─ domain/
│  │  ├─ core/
│  │  ├─ services/
│  │  ├─ config/
│  │  └─ jobs/
│  ├─ infra/
│  │  ├─ db/
│  │  ├─ network/
│  │  ├─ logging/
│  │  └─ web/
│  └─ shared/
│     └─ util/
└─ tests/
```

### 6.1 Backend Layer Intent

- `cmd/b-ui`: the only binary entrypoint
- `internal/app`: startup wiring, lifecycle, application assembly
- `internal/http/api`: router registration, handlers, request mapping, sessions
- `internal/http/middleware`: HTTP middleware only
- `internal/domain/services`: business logic currently living under broad `service/`
- `internal/domain/core`: sing-box and runtime core integration
- `internal/domain/config`: application configuration and name/version support
- `internal/domain/jobs`: scheduled jobs and cron orchestration
- `internal/infra/*`: database, networking, logging, and static asset serving concerns
- `internal/shared/util`: small shared helpers that are genuinely cross-cutting

### 6.2 Current-to-Target Mapping

- `main.go` -> `src/backend/cmd/b-ui/main.go`
- `app/` -> `src/backend/internal/app/`
- `api/` -> `src/backend/internal/http/api/`
- `middleware/` -> `src/backend/internal/http/middleware/`
- `service/` -> `src/backend/internal/domain/services/`
- `core/` -> `src/backend/internal/domain/core/`
- `config/` -> `src/backend/internal/domain/config/`
- `cronjob/` -> `src/backend/internal/domain/jobs/`
- `database/` -> `src/backend/internal/infra/db/`
- `network/` -> `src/backend/internal/infra/network/`
- `logger/` -> `src/backend/internal/infra/logging/`
- `web/` backend-serving code -> `src/backend/internal/infra/web/`
- `util/` -> `src/backend/internal/shared/util/`

### 6.3 Boundary Rules

- HTTP handlers should not absorb business rules
- domain services should not directly contain HTTP concerns
- infrastructure packages should not become generic utility dumping grounds
- `internal/` should be used to keep repository-local architecture boundaries enforceable
- deeper domain splitting can be deferred until after the structural migration is stable

## 7. Frontend and Static Assets

The frontend should move from `frontend/` to `src/frontend/` as a self-contained Vite application.

### 7.1 Frontend Rules

- keep frontend package management local to `src/frontend`
- keep frontend dev/build/test/lint commands defined in `src/frontend/package.json`
- do not spread repository-wide build concerns back into frontend-local package scripts

### 7.2 Static Asset Output

The built frontend assets should continue to be copied or synced into root `web/html/`.

Reasoning:
- `web/` is a final delivery artifact location, not a source directory
- root `web/` is easier to consume from packaging, backend serving, Docker builds, and compatibility paths
- there is no need to conflate backend source layout with final static artifact placement

## 8. Runtime Service Assets

`src/services` is reserved for service and platform runtime assets, not for Go business logic.

Target layout:

```text
src/services/
├─ systemd/
│  └─ b-ui.service
├─ windows/
│  ├─ b-ui-windows.xml
│  ├─ install-windows.bat
│  ├─ uninstall-windows.bat
│  ├─ build-windows.bat
│  ├─ build-windows.ps1
│  └─ README.md
└─ runtime/
   ├─ entrypoint.sh
   └─ b-ui.sh
```

### 8.1 Intent

- `systemd/`: Linux service unit assets
- `windows/`: Windows service and install assets
- `runtime/`: runtime wrapper scripts and service-adjacent launcher assets

This avoids overloading the meaning of `services` and keeps platform/runtime delivery files distinct from backend business code.

## 9. Repository Scripts

All repository-level automation should be centralized under `scripts/`.

Target layout:

```text
scripts/
├─ dev/
├─ build/
├─ release/
├─ migration/
└─ ci/
```

### 9.1 Responsibilities

- `scripts/dev`: local development workflows, front-back orchestration, convenience runners
- `scripts/build`: deterministic build entrypoints for frontend, backend, and combined builds
- `scripts/release`: release assembly, install implementation, archive packaging
- `scripts/migration`: migration helpers and compatibility migration scripts
- `scripts/ci`: CI orchestration and verification entrypoints

### 9.2 Current-to-Target Mapping

- `runSUI.sh` -> `scripts/dev/run-local.sh`
- `build.sh` -> `scripts/build/build-linux.sh`
- `docker-build-test.sh` -> `scripts/ci/docker-build-test.sh`
- `migrate-to-b-ui.sh` -> `scripts/migration/migrate-to-b-ui.sh`
- root install implementation -> `scripts/release/install.sh`

### 9.3 Compatibility Shims

The root-level `install.sh` and `b-ui.sh` should remain, but become thin wrappers that delegate into their new implementation locations.

Root wrappers should:
- preserve existing invocation syntax
- avoid retaining complex logic long-term
- fail clearly if an internal delegated script is missing

## 10. Packaging Layout

Packaging concerns should move into `packaging/`.

Target layout:

```text
packaging/
├─ docker/
│  ├─ Dockerfile
│  └─ Dockerfile.frontend-artifact
├─ release/
└─ install/
```

### 10.1 Intent

- `packaging/docker`: Docker build definitions and image-related packaging assets
- `packaging/release`: release archive templates, asset manifests, assembly helpers
- `packaging/install`: install-time templates or static packaging resources needed by install flows

This separates packaging descriptions from executable scripts.

## 11. Build Output Layout

Introduce explicit build output directories:

```text
build/
├─ out/
├─ tmp/
└─ reports/

dist/
└─ release/
```

### 11.1 Roles

- `build/out`: local build binaries and generated artifacts for development
- `build/tmp`: temporary assembly files and intermediate staging content
- `build/reports`: logs and verification reports from CI or local checks
- `dist/release`: final archives intended for release distribution

This prevents scripts from scattering outputs into unrelated directories.

## 12. Build Flow

The build flow should be standardized around scripts rather than ad hoc commands in docs and workflows.

### 12.1 Frontend Flow

1. enter `src/frontend`
2. install dependencies if needed
3. run frontend build
4. sync output into root `web/html`

Recommended script entrypoint:
- `scripts/build/build-frontend.sh`

### 12.2 Backend Flow

1. build from root module using `src/backend/cmd/b-ui`
2. output binary into `build/out/`
3. reuse `web/` as the static artifact source for serving or packaging

Recommended script entrypoint:
- `scripts/build/build-backend.sh`

### 12.3 Combined Build Flow

1. build frontend
2. sync static assets
3. build backend binary
4. optionally stage for packaging

Recommended script entrypoint:
- `scripts/build/build-all.sh`

## 13. CI/CD Design

CI should call repository scripts rather than encode business logic directly in workflow YAML.

### 13.1 Workflow Roles

Recommended CI capability split:

- `verify`: lint, test, and build validation for frontend and backend
- `package`: build release artifacts for Linux and Windows
- `docker`: build container images from packaging assets
- `release`: tag-driven release publication

### 13.2 CI Rules

- workflows should primarily delegate to `scripts/ci` or `scripts/release`
- workflows should avoid embedding complex path logic
- path changes should mostly require script updates, not workflow redesign
- Docker workflows should reference `packaging/docker/`

### 13.3 Verification Scope

The verify flow should cover:
- frontend lint
- frontend tests
- frontend production build
- backend tests
- backend build
- script and path sanity checks where appropriate

## 14. Migration Strategy

This refactor should be performed in phases rather than as a single unreviewable move.

### Phase 1: Structural relocation and compatibility shelling

- create target directories
- move frontend to `src/frontend`
- move backend packages to `src/backend`
- move runtime service assets to `src/services`
- move scripts and packaging files into their new homes
- keep root wrappers and compatibility entrypoints in place
- update imports and paths without changing behavior intentionally

### Phase 2: Script unification

- create stable repository-level build, release, migration, and CI entrypoints
- ensure workflows call scripts instead of inline logic
- normalize artifact destinations into `build/` and `dist/`

### Phase 3: Documentation and path cleanup

- update README and contributor documentation to new paths
- document new engineering entrypoints
- remove obsolete references to old directory locations

### Phase 4: Optional deeper backend cleanup

- after the structure is stable, evaluate whether domain packages should be split further by feature area
- keep this out of the first structural migration unless required by blockers

## 15. Out of Scope for the First Pass

The first repository restructure pass should not attempt to do all of the following at once:
- redesign product behavior
- rewrite backend domain logic wholesale
- change user-facing install commands
- change release asset naming
- redesign frontend application architecture beyond path relocation needs
- introduce a new package manager, build system, or monorepo framework

The purpose of the first pass is structure, consistency, and maintainability.

## 16. Success Criteria

The repository restructure is successful when:
- all source code is clearly grouped under `src/backend` and `src/frontend`
- runtime service assets are clearly grouped under `src/services`
- repository automation is discoverable under `scripts/`
- packaging assets are grouped under `packaging/`
- root-level clutter is materially reduced
- existing external install and release compatibility is preserved
- CI workflows become thinner and easier to maintain
- developers can identify where to place new code or scripts without guessing

## 17. Open Execution Guidance

During implementation, prefer minimal behavioral changes while moving files. The first objective is to establish a durable structure. Once that structure is working and verified, deeper cleanup can proceed with lower risk.
