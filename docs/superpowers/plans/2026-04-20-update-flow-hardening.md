# Update Flow Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `install.sh --update` and `--force-update` refuse fresh installs and legacy-only `s-ui` installs, compare versions correctly, and update only existing `b-ui` installs.

**Architecture:** Add a small install-state classification layer inside `install.sh`, expose script internals safely for shell tests, and drive the behavior change from tests that call the update guard logic directly. Keep install and migrate flows intact while making update-mode messaging explicit and non-migratory.

**Tech Stack:** Bash, GNU coreutils, existing install/update shell scripts

---

### Task 1: Add a failing shell test harness for update-mode guards

**Files:**
- Create: `tests/install-update-mode.sh`

- [ ] **Step 1: Write the failing test**

```bash
test_script_can_be_sourced_without_running_main
test_update_refuses_missing_b_ui_install
test_update_refuses_legacy_only_s_ui_install
test_update_exits_when_current_version_is_equal_or_newer
test_update_continues_when_current_version_is_older
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/install-update-mode.sh`
Expected: FAIL because `install.sh` currently runs top-level logic when sourced and treats `--update` without an install as a fresh install.

- [ ] **Step 3: Keep the harness minimal**

```bash
source "./install.sh"
MODE="update"
TARGET_VERSION="v1.2.0"
detect_existing_install
check_update_requirement
```

- [ ] **Step 4: Run test to verify the failure is about the current behavior**

Run: `bash tests/install-update-mode.sh`
Expected: FAIL with output showing the current script cannot safely expose the update guard behavior for isolated testing.

- [ ] **Step 5: Commit**

```bash
git add tests/install-update-mode.sh
git commit -m "test: cover update mode install guards"
```

### Task 2: Refactor install script entrypoints and update guard logic

**Files:**
- Modify: `install.sh`
- Test: `tests/install-update-mode.sh`

- [ ] **Step 1: Write the next failing assertions in the shell test**

```bash
assert_contains "$RUN_OUTPUT" 'System does not have b-ui installed'
assert_contains "$RUN_OUTPUT" 'bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh)'
assert_contains "$RUN_OUTPUT" 'Detected s-ui but b-ui is not installed'
assert_contains "$RUN_OUTPUT" 'bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) --migrate'
assert_contains "$RUN_OUTPUT" 'already up to date'
assert_contains "$RUN_OUTPUT" 'bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) --force-update'
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/install-update-mode.sh`
Expected: FAIL because `install.sh` still reports fresh-install behavior for `--update`, lacks legacy-only detection, and only short-circuits exact version matches.

- [ ] **Step 3: Write minimal implementation**

```bash
SYSTEMD_DIR="${SYSTEMD_DIR:-/etc/systemd/system}"
INSTALL_COMMAND='bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh)'
MIGRATE_COMMAND='bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) --migrate'
FORCE_UPDATE_COMMAND='bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) --force-update'

detect_existing_install() { ... }
version_is_gte() { ... }
check_update_requirement() { ... }

main() {
  parse_args "$@"
  require_root
  detect_os_release
  detect_existing_install
  resolve_target_version
  check_update_requirement
  install_base
  install_app
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
```

- [ ] **Step 4: Run test to verify it passes**

Run: `bash tests/install-update-mode.sh`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add install.sh tests/install-update-mode.sh
git commit -m "fix: harden update mode install checks"
```

### Task 3: Remove legacy migration messaging from normal b-ui updates and align docs

**Files:**
- Modify: `install.sh`
- Modify: `README.md`
- Modify: `MIGRATION.md`
- Test: `tests/install-update-mode.sh`

- [ ] **Step 1: Extend the failing check for non-migration update messaging**

```bash
assert_not_contains "$RUN_OUTPUT" 'Compatible legacy installation detected'
```

- [ ] **Step 2: Run test to verify it fails if update still emits migration wording**

Run: `bash tests/install-update-mode.sh`
Expected: FAIL until legacy-only messaging is separated from normal `b-ui` update behavior.

- [ ] **Step 3: Write minimal implementation**

```bash
if [[ "${INSTALLATION_KIND}" == "legacy-only" ]]; then
  echo 'Compatible legacy installation detected...'
fi
```

Update the docs so `--update` explicitly says:

```markdown
- exits when `b-ui` is not installed and points to the install command
- exits when only `s-ui` is installed and points to the migrate command
- exits when the installed `b-ui` version is already current or newer and points to `--force-update`
- only updates existing `b-ui` content and does not perform legacy migration
```

- [ ] **Step 4: Run tests and spot-check docs**

Run: `bash tests/install-update-mode.sh`
Expected: PASS

Run: `git diff -- README.md MIGRATION.md`
Expected: update wording matches the implemented behavior.

- [ ] **Step 5: Commit**

```bash
git add install.sh README.md MIGRATION.md tests/install-update-mode.sh
git commit -m "docs: clarify update and migrate behavior"
```
