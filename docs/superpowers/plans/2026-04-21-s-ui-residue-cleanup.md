# S-UI Residue Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove non-migration `s-ui` residue so the repository and shipped product present as `b-ui`, while migration code still accepts legacy `s-ui` inputs and converts them into `b-ui` steady-state outputs.

**Architecture:** Split the work into three coordinated areas: product identity in code, runtime/package naming, and migration-preserving compatibility. Keep migration detection for legacy paths, services, databases, and commands, but standardize all default outputs and visible runtime names to `b-ui`. Evaluate the Go module path explicitly and migrate it in the same change if verification remains tractable.

**Tech Stack:** Go, Vue 3 + TypeScript, Bash, PowerShell, Windows batch, GitHub Actions, shell-based regression tests.

---

### Task 1: Rename Product Identity In Code

**Files:**
- Modify: `go.mod`
- Modify: `src/backend/**/*.go`
- Modify: `src/frontend/src/router/index.ts`

- [ ] **Step 1: Write the failing test**

Use the existing shell test to lock session naming and module import expectations by adding assertions for `b-ui` steady-state naming in the relevant test files before changing production code.

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/install-update-mode.sh`
Expected: existing tests pass, and any new assertions around `b-ui` runtime naming fail before implementation.

- [ ] **Step 3: Write minimal implementation**

Change default code-facing identifiers from `s-ui` to `b-ui`, including:

- Go module path and internal imports
- backend session store name
- backend logger module name
- frontend session cookie lookup

Keep migration-only compatibility reads where needed.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 5: Commit**

Do not commit unless the user explicitly asks for it.

### Task 2: Standardize Runtime, Packaging, And Service Naming

**Files:**
- Modify: `scripts/build/build-backend.sh`
- Modify: `scripts/dev/run-local.sh`
- Modify: `scripts/release/package-linux.sh`
- Modify: `scripts/release/package-windows.ps1`
- Modify: `scripts/release/install.sh`
- Modify: `src/services/systemd/b-ui.service`
- Modify: `src/services/runtime/b-ui.sh`
- Modify: `src/services/container/entrypoint.sh`
- Modify: `src/services/windows/*.bat`
- Modify: `src/services/windows/*.ps1`
- Modify: `src/services/windows/*.xml`
- Modify: `.github/workflows/*.yml`

- [ ] **Step 1: Write the failing test**

Extend packaging and installer regression tests to assert `b-ui` executable names in archive contents, install/update version detection, and runtime entrypoints.

- [ ] **Step 2: Run test to verify it fails**

Run: `bash scripts/release/package-linux.test.sh`
Expected: FAIL because the archive still contains `sui`.

- [ ] **Step 3: Write minimal implementation**

Rename default output binaries and service targets from `sui`/`sui.exe` to `b-ui`/`b-ui.exe`, while preserving migration-time detection for legacy `s-ui` assets.

- [ ] **Step 4: Run test to verify it passes**

Run:
- `bash scripts/release/package-linux.test.sh`
- `bash tests/install-update-mode.sh`

Expected: PASS

- [ ] **Step 5: Commit**

Do not commit unless the user explicitly asks for it.

### Task 3: Clean Remaining Visible Residue And Verify End State

**Files:**
- Modify: `README.md`
- Modify: `docs/manual.md`
- Modify: `CONTRIBUTING.md`
- Modify: repository files matching visible non-migration `s-ui` residue
- Delete: `sui`

- [ ] **Step 1: Write the failing test**

Add or tighten test assertions so packaged output, runtime commands, and docs no longer describe normal `b-ui` operation using `s-ui` names.

- [ ] **Step 2: Run test to verify it fails**

Run the relevant shell regression tests after adding those assertions.
Expected: FAIL until docs and visible outputs are updated.

- [ ] **Step 3: Write minimal implementation**

Delete the root-level binary residue and clean non-migration-visible `s-ui` references in docs and scripts.

- [ ] **Step 4: Run test to verify it passes**

Run:
- `go test ./...`
- `bash scripts/release/package-linux.test.sh`
- `bash tests/install-update-mode.sh`

Expected: PASS

- [ ] **Step 5: Commit**

Do not commit unless the user explicitly asks for it.
