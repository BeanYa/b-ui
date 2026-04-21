# B-UI User Documentation Restructure Design

Date: 2026-04-20
Status: Draft approved in conversation, written for review
Scope: Public-facing installation, quickstart, protocol setup, update, and migration documentation

## 1. Goal

Restructure the user-facing documentation so new users first see how to install B-UI, then how to get to a working proxy site quickly, while still providing a complete operator manual for update, force-update, migration, configuration concepts, and protocol-specific setup.

The redesigned docs must:
- lead with fresh installation rather than migration
- guide a new user from install to a working proxy site with the shortest clear path
- document how to create TLS settings and inbound settings
- use `VLESS + TLS` as the default end-to-end example
- add explicit setup sections for `Hysteria2` and `VLESS + Reality`
- consolidate migration content into the main documentation set instead of keeping it as the dominant top-level path
- explain the logic and basic usage of the main panel features at a practical level

## 2. Current Problems

The current documentation has several mismatches with the intended user journey:

- `README.md` gives migration a more prominent role than first-time installation
- installation exists, but it is not framed as the primary entrypoint for new users
- the docs do not provide one obvious “fastest way to create a working proxy site” path
- TLS and inbound setup are not documented as a short operational flow
- migration content is isolated in `MIGRATION.md`, but installation, update, and usage guidance are split across files without one complete operator manual
- protocol examples are incomplete for the desired audience because `Hysteria2` and `VLESS + Reality` need dedicated setup guidance

## 3. Audience

The target documentation should serve both:

- new users who need a successful first install and a minimum viable setup path
- operators who need update, force-update, migration, and configuration guidance

The writing priority should be:

1. get a new user installed and running quickly
2. provide a complete follow-up manual for deeper operations

## 4. Evaluated Approaches

### Approach A: README as install-first entrypoint plus one complete operator manual

`README.md` becomes a short install-first landing page, while a separate full manual covers install, setup, protocols, update, migration, and feature usage.

Pros:
- best fit for new users
- avoids an oversized README
- keeps one authoritative long-form manual
- easy to extend later

Cons:
- users must follow a second link for full details

### Approach B: One large all-in-one README

Put installation, quickstart, TLS, inbound setup, updates, migration, and feature usage all into `README.md`.

Pros:
- single file for everything

Cons:
- README becomes too long and harder to maintain
- weak separation between landing-page content and detailed operations content

### Approach C: README plus many topic-specific documents

Keep `README.md` minimal and split installation, setup, migration, and protocol guides into multiple separate documents.

Pros:
- strongest topical separation

Cons:
- more navigation overhead for new users
- heavier initial rewrite

### Recommendation

Adopt Approach A.

This best matches the desired outcome: installation first, quickstart immediately after, and a complete operator manual for everything else.

## 5. Target Documentation Structure

### 5.1 README

`README.md` should become a concise entrypoint focused on:

- what B-UI is
- how to install it as a new user
- how to access the panel after installation
- where to find the quickstart and full manual
- brief pointers for update and migration

### 5.2 Full Manual

Create a new long-form operator manual under `docs/`.

Recommended path:
- `docs/manual.md`

Alternative acceptable path if naming needs to be more explicit:
- `docs/guide.md`

This manual becomes the authoritative detailed documentation for:

- installation
- first login and initialization
- fastest setup path
- TLS settings
- inbound settings
- protocol walkthroughs
- update and force-update
- migration from upstream
- feature logic and basic usage
- troubleshooting

### 5.3 Migration Document Handling

`MIGRATION.md` should no longer act as the main top-level operational path.

Recommended handling:

- move the substantive migration content into the new full manual
- keep `MIGRATION.md` as a short compatibility page that links to the migration section in the full manual

This preserves old links while moving the center of gravity to the new install-first documentation flow.

## 6. README Design

The README should be restructured in this order:

### Section 1: Project Summary

Keep this short:
- one short explanation of the fork
- one short explanation of what B-UI helps users do

Do not lead with repository internals or migration.

### Section 2: Installation

This must be the first operational section.

It should include:
- fresh install command
- optional install-to-version example
- short explanation of what gets installed
- install result expectations: command name, service name, panel location

### Section 3: Quickstart

This should immediately answer:
- how to open the panel
- first login/default access expectations if applicable
- what to do next

This section should link directly to:
- the fast setup path in the full manual

### Section 4: Documentation Links

Provide short links to:
- full manual
- migration section
- development/contributor docs

### Section 5: Developer Notes

Keep only concise engineering notes here:
- repository structure summary
- frontend/backend development entrypoints
- link to `CONTRIBUTING.md`

## 7. Full Manual Design

The full manual should use this section order.

### 7.1 Installation

Cover:
- fresh installation
- install to a specific version
- where the binary, service, and database live after install
- what names the user should expect after install

### 7.2 First Login and Initialization

Cover:
- panel access location
- first checks after login
- what needs to be configured before creating a site

### 7.3 Fastest Way To Create A Proxy Site

This is the primary guided flow in the full manual.

It must use `VLESS + TLS` as the default example.

The flow order should be:

1. prerequisites
2. create TLS settings
3. create inbound settings
4. add client or user
5. export/copy connection information
6. basic verification

### 7.4 TLS Settings

This section must explain:
- what TLS settings are for in the panel
- the smallest set of fields a user must understand
- how TLS relates to later inbound configuration
- common mistakes

The writing should be operational, not theoretical.

### 7.5 Inbound Settings

This section must explain:
- what an inbound is in the panel
- which fields are mandatory to get a site running
- how port, listen address, domain, protocol, transport, and TLS connect together
- what a user must choose versus what can stay default

### 7.6 Protocol Walkthroughs

Three protocol examples must be present:

- `VLESS + TLS`
- `Hysteria2`
- `VLESS + Reality`

#### `VLESS + TLS`

This is the canonical quickstart example.

It should be written as a copyable workflow:
- when to use it
- prerequisites
- exact panel flow
- minimum required fields
- how to verify success

#### `Hysteria2`

This should be a full secondary example, not just a note.

It must explain:
- when to choose it
- how its setup differs from `VLESS + TLS`
- the minimum required fields
- how to validate the result

#### `VLESS + Reality`

This should be documented as an advanced example.

It must explain:
- when to choose it
- how it differs from standard TLS setup
- which fields are easiest to misconfigure
- how to validate the result

The manual should explicitly tell users that `VLESS + TLS` is the recommended first successful path, while `VLESS + Reality` is a more advanced setup.

### 7.7 Update and Force-Update

Cover:
- `b-ui update`
- `b-ui update --force`
- version-specific update commands
- when to use each mode
- what update mode refuses to do

This section should align with the actual behavior in `scripts/release/install.sh`.

### 7.8 Migration From Upstream

This section should absorb the current migration logic from `MIGRATION.md` and explain:
- when migration is needed
- what is preserved
- what changes names after migration
- what `--migrate` does step by step
- what rollback behavior exists

### 7.9 Feature Logic and Basic Usage

This section should briefly explain the purpose of major panel concepts:

- TLS
- inbounds
- outbounds
- endpoints
- clients/users
- services

The goal is not full API-level reference. The goal is to help operators understand what each object does in normal usage.

### 7.10 Troubleshooting

Provide practical first-line checks for:
- service not starting
- panel not reachable
- certificate issues
- port conflicts
- connection export/client failures

## 8. Writing Rules

The rewritten docs should follow these rules:

- installation must be introduced before migration
- examples must focus on the minimum viable path before advanced explanation
- protocol setup must be written as step-by-step operations, not only as field descriptions
- TLS and inbound setup should be written with practical dependency order
- advanced content must not block the first-success path
- avoid duplicating the same operational details across README and full manual; README should summarize and link outward

## 9. Scope Boundaries

This documentation restructure should not attempt to become:
- full API reference
- exhaustive per-field encyclopedia for every protocol and every advanced option
- full UI redesign or screenshot overhaul unless existing screenshots are clearly wrong or misleading

The priority is a clearer user journey and a more coherent operational manual.

## 10. Success Criteria

The documentation restructure is successful when:
- a new user opening `README.md` first sees how to install B-UI
- a new user can find the shortest working setup path without reading migration content first
- TLS and inbound creation are documented clearly enough to complete a working proxy site
- `VLESS + TLS` is the main quickstart path
- `Hysteria2` and `VLESS + Reality` each have dedicated setup guidance
- migration content is still available, but no longer dominates the top-level user experience
- update, force-update, and migration behaviors documented in prose match the actual install script behavior
