# Contributing to B-UI

Thank you for your interest in contributing to B-UI. This repository keeps the upstream Go backend and install layout compatible, while the frontend, branding, release assets, and contributor workflow are maintained here in the `BeanYa/b-ui` fork.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Development Environment Setup](#development-environment-setup)
- [Coding Conventions and Style Guide](#coding-conventions-and-style-guide)
- [Testing](#testing)
- [Features That Need Help](#features-that-need-help)
- [Pull Request Process](#pull-request-process)
- [Reporting Bugs and Requesting Features](#reporting-bugs-and-requesting-features)

---

## Code of Conduct

Please be respectful and constructive when interacting with maintainers and other contributors. This project is for personal learning and communication; use it responsibly and legally.

---

## Development Environment Setup

Before changing code or docs, use the root [`README.md`](./README.md) for the repository overview, [`docs/manual.md`](./docs/manual.md) for the user/operator workflow, and [`MIGRATION.md`](./MIGRATION.md) for upgrade or install-path migration details. This guide stays focused on contributor workflow.

This project also keeps a local `docs/superpowers/` workspace for design specs, implementation plans, and other Superpower skill outputs used to support development. These documents are auxiliary engineering artifacts that help structure feature work, debugging, review, and release preparation alongside the normal Git workflow.

### Prerequisites

- **Go**: 1.25 or later (see `go.mod` for the exact version).
- **Git**: For cloning and normal source control operations.
- **C compiler**: Required for CGO (e.g. `gcc`, `musl-dev` on Alpine).
- **Node.js** (optional): Only if you plan to work on or rebuild the frontend. The repo can be run with pre-built frontend assets.

### Clone Repository

```bash
git clone https://github.com/BeanYa/b-ui.git
cd b-ui
```

The **frontend** now lives directly in the `src/frontend/` directory of this repository. There is no extra frontend submodule sync or separate frontend commit to manage. If you only work on the backend, you can use the checked-in backend web assets or rebuild the frontend once (see below).

### Local Development (quickstart)

1. Build and run with the centralized dev script:

   ```bash
   bash ./scripts/dev/run-local.sh
   ```

   This is the fastest way to verify local changes across the frontend and backend together.

2. Or build manually with the centralized build scripts:

   ```bash
    bash ./scripts/build/build-frontend.sh
    bash ./scripts/build/build-backend.sh
    BUI_DB_FOLDER=db BUI_DEBUG=true ./build/out/b-ui
    ```

   Use this path when you only need to rebuild one side of the app or want to inspect failures step by step.

### Build Tags

The backend is built with these tags for full functionality:

- `with_quic`, `with_grpc`, `with_utls`, `with_acme`, `with_gvisor`, `with_naive_outbound`, `with_tailscale`
- platform-specific builds additionally layer in tags such as `with_musl` or `with_purego`, plus compatibility tags like `badlinkname` and `tfogo_checklinkname0`

Use the centralized build scripts when you need release-parity behavior locally, because they resolve the correct tag set per host and target.

### Release and Versioning

- Contributor-facing release automation lives under `scripts/build/` and `scripts/release/`.
- User-facing release packaging, install-layout, and migration details belong in [`README.md`](./README.md), [`docs/manual.md`](./docs/manual.md), and [`MIGRATION.md`](./MIGRATION.md).

### Environment Variables (development)

| Variable       | Description                    | Example   |
|----------------|--------------------------------|-----------|
| `BUI_DB_FOLDER`| Directory for SQLite DB files  | `db`      |
| `BUI_DEBUG`    | Enable debug mode              | `true`    |
| `BUI_LOG_LEVEL`| Log level                      | `debug`   |
| `BUI_BIN_FOLDER` | Directory for binaries       | `bin`     |

### Docker (optional)

Contributors who need container-based workflows should inspect `packaging/docker/` and the related scripts directly. End-user deployment steps belong in the root [`README.md`](./README.md) and [`docs/manual.md`](./docs/manual.md).

---

## Coding Conventions and Style Guide

### General

- Write clear, maintainable code. Prefer small, focused functions and packages.
- Comment non-obvious logic and public APIs.
- Handle errors explicitly; avoid ignoring `err` unless intentional.

### Go Style

- Follow **standard Go style** and **[Effective Go](https://go.dev/doc/effective_go)**.
- Run **gofmt** (or **goimports**) before committing:

  ```bash
  gofmt -w .
  # or: goimports -w .
  ```

- Use **camelCase** for unexported names and **PascalCase** for exported names.
- Keep package names short and lowercase (e.g. `api`, `service`, `util`).
- Group imports: standard library, then third-party, then project imports (as in existing files).

### Project Structure Conventions

- **`src/backend/cmd/b-ui/`**: backend executable entrypoint.
- **`src/backend/internal/http/`**: HTTP handlers and API routing.
- **`src/backend/internal/app/`**: application services and orchestration.
- **`src/backend/internal/domain/`**: core domain models and business rules.
- **`src/backend/internal/infra/`**: database, runtime, and framework integrations.
- **`src/backend/internal/shared/`**: shared helpers and cross-cutting utilities.
- **`src/services/`**: runtime/system service assets for Linux and Windows.

When adding new features, place code in the appropriate layer (handler → service → model/util) and avoid circular dependencies.

### Naming and Patterns

- Handlers: suffix `Handler` (e.g. `APIHandler`, `APIv2Handler`).
- Services: suffix `Service` or use package name (e.g. `ApiService`, `LinkService`).
- Models: clear struct names with JSON/gorm tags (see `src/backend/internal/infra/db/model/`).

---

## Testing

### Current State

- The repository already includes focused Go and frontend tests, for example `src/backend/internal/domain/config/config_test.go` and the Vitest suites under `src/frontend/`.
- Centralized verification now includes `go test ./...` and the build scripts exposed through `scripts/ci/verify.sh`.

### What You Can Do Now

1. **Build and test verification**: Before submitting a PR, ensure the project passes the centralized verification flow:

   ```bash
   bash ./scripts/ci/verify.sh
   ```

2. **Manual testing**: Run with `bash ./scripts/dev/run-local.sh`, test the changed area (panel, API, subscription, etc.).

   If you need the current end-user setup or runtime flow while testing, follow the root [`README.md`](./README.md) and [`docs/manual.md`](./docs/manual.md) instead of duplicating those steps in contribution notes.

3. **Future tests**: Contributions that add **unit tests** (e.g. for `src/backend/internal/shared/util/`, `src/backend/internal/domain/services/`, or API handlers) or **integration tests** are very welcome. Prefer the standard library `testing` package and table-driven tests where appropriate.

### Running the Linter (optional)

```bash
go vet ./...
# Optional: staticcheck, golangci-lint, etc.
```

---

## Features That Need Help

Community help is especially valuable in these areas. Check the [Issues](https://github.com/BeanYa/b-ui/issues) for current tasks and ideas.

### High-Value Areas

- **Multi-inbound per user**: Core differentiator inherited from upstream; improvements to UX, docs, and robustness are welcome.
- **API (v1 and v2)**: Completeness, consistency, and documentation. Upstream API docs remain a useful reference: [API Documentation](https://github.com/alireza0/b-ui/wiki/API-Documentation).
- **Subscription service**: Link conversion, JSON subscription, and info endpoints (`src/backend/internal/http/sub/`, `src/backend/internal/shared/util/`).
- **Testing**: Adding unit and integration tests for critical paths.
- **Documentation**: User docs, migration/update docs, release notes, and contribution docs.
- **Platform support**: macOS is experimental; Windows and Linux improvements are welcome (see `src/services/windows/` and `.github/workflows/`).
- **Frontend and design system**: UI work in `src/frontend/` should follow [`DESIGN.md`](./DESIGN.md) and keep the darker desktop-tool direction intact.

### How to Find Tasks

- **Good first issue**: Look for issues labeled `good first issue` or `help wanted`.
- **Feature requests**: [Feature request template](.github/ISSUE_TEMPLATE/feature_request.md).
- **Bugs**: [Bug report template](.github/ISSUE_TEMPLATE/bug_report.md).

If you want to work on a larger feature, open an issue first to discuss approach and avoid duplicate work.

---

## Pull Request Process

1. **Fork and branch**

   - Fork the repository on GitHub.
   - Create a branch from `main`: e.g. `git checkout -b fix/issue-123` or `feature/sub-improvements`.

2. **Make your changes**

   - Follow the [Coding Conventions](#coding-conventions-and-style-guide).
   - Run `gofmt` and ensure the project builds (see [Testing](#testing)).
   - Keep commits focused and messages clear (e.g. "Fix link conversion for VMess", "Add tests for outJson").

3. **Push and open a PR**

   - Push your branch and open a Pull Request against `main`.
   - Use the PR description to explain:
     - What problem or feature the PR addresses.
     - What you changed and how to verify it.
   - Reference any related issue (e.g. "Fixes #123").

4. **Review and CI**

   - Maintainers will review your code. CI (e.g. build workflows) must pass.
   - Address feedback by pushing new commits to the same branch.

5. **Merge**

   - Once approved and CI is green, a maintainer will merge your PR. Thank you for contributing!

### PR Guidelines

- Prefer **small, reviewable PRs**. Split large features into logical steps.
- Avoid unrelated changes (e.g. formatting-only or refactors in a feature PR).
- Ensure your branch is up to date with `main` before submitting (rebase or merge as the project prefers).
- When a change affects onboarding, operations, or upgrades, update the matching user-facing doc in [`README.md`](./README.md), [`docs/manual.md`](./docs/manual.md), or [`MIGRATION.md`](./MIGRATION.md) alongside the code change.

---

## Reporting Bugs and Requesting Features

- **Bugs**: Use the [bug report template](.github/ISSUE_TEMPLATE/bug_report.md). Include version, OS, steps to reproduce, and expected vs actual behavior.
- **Features**: Use the [feature request template](.github/ISSUE_TEMPLATE/feature_request.md). Describe the use case and, if possible, a proposed approach.
- **Questions**: Use the [question template](.github/ISSUE_TEMPLATE/question-template.md) or discussions if enabled.

---

Thank you for helping B-UI evolve. Contributions here keep upstream compatibility intact while moving the fork’s UI, packaging, and release workflow forward.
