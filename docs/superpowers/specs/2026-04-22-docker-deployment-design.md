# Docker Deployment Bootstrap Design

## Goal

Add a dedicated Docker deployment path for B-UI that:

- stays separate from the existing bare-metal `install.sh` flow
- interactively gathers deployment parameters
- generates a local Docker Compose deployment
- starts the container
- initializes B-UI settings inside the running container
- optionally bootstraps one protocol-ready TLS template, one client, and one inbound

The deployment must default to direct `IP:port` access for the panel. It must not configure ACME for panel access, host Nginx reverse proxying, or domain routing.

## Scope

### In scope

- new dedicated Docker installer script
- generated `docker-compose.yml` and `.env` for operator-managed deployments
- container startup checks
- interactive initialization of:
  - panel port and path
  - subscription port and path
  - admin username and password
- optional bootstrap of one protocol stack:
  - `VLESS + TLS`
  - `VLESS + Reality`
  - `Hysteria2`
- optional bootstrap of one client with:
  - username
  - traffic limit
  - expiry
  - auto reset flag
  - reset interval days
- port mapping generation for panel, subscription, inbound, and reserved ports
- documentation for Docker deployment and generated artifacts

### Out of scope

- changing the current Linux bare-metal installer semantics
- host Nginx setup
- host firewall automation
- panel ACME automation
- host-level domain or HTTPS setup for the panel
- multi-client or multi-inbound bootstrap in the first version
- full lifecycle management beyond initial deployment and documented rerun paths

## Why a Separate Docker Installer

The current `scripts/release/install.sh` is built around package download, filesystem install, systemd service registration, and CLI-based post-install setup. Docker deployment has different concerns: image selection, volume layout, Compose generation, container health checks, and API-driven object bootstrap after the application is already running.

Keeping Docker deployment in a dedicated script avoids mixing two deployment models with different failure modes and operator expectations.

## Operator Experience

The operator runs one script dedicated to Docker deployment. The script:

1. checks Docker prerequisites
2. asks a bounded set of interactive questions
3. writes deployment files to a chosen directory
4. starts the container with `docker compose up -d`
5. waits for the panel to become reachable
6. initializes panel settings and admin credentials
7. optionally creates one TLS template, one client, and one inbound using the same backend save APIs the panel uses
8. prints the final panel URL, subscription URL, mapped ports, and generated deployment paths

The default success path is a running panel reachable via `http://<server-ip>:<panel-port>`.

## Proposed Files

- `scripts/release/install-docker.sh`
- `scripts/release/templates/docker-compose.bootstrap.yml.tpl`
- documentation updates in:
  - `README.md`
  - `docs/manual.md`

The generated runtime files live in an operator-owned deployment directory, not inside tracked repository files.

## Deployment Directory Layout

The script generates a deployment directory such as `./b-ui-docker/` containing:

- `docker-compose.yml`
- `.env`
- `db/`
- `cert/`
- optional bootstrap logs such as `bootstrap.log`

The generated files become the operator's persistent deployment entrypoint.

## Interactive Inputs

### Deployment settings

- deployment directory
- image reference, default `ghcr.io/beanya/b-ui:latest`
- container name
- panel host port
- panel path
- subscription host port
- subscription path
- reserved extra host port mappings
- whether to bootstrap a protocol stack

### Admin settings

- admin username
- admin password

### Protocol selection

If bootstrap is enabled, choose exactly one:

- `VLESS + TLS`
- `VLESS + Reality`
- `Hysteria2`

### Client settings

- client display name
- traffic limit
- expiry mode
- expiry timestamp or valid-for-days value
- auto reset enabled/disabled
- reset interval days if enabled

### Protocol-specific settings

#### VLESS + TLS

- inbound tag
- listen port
- public server address for exported links
- TLS template name
- certificate source mode:
  - generate self-signed content
  - use mounted file paths
- if using file paths:
  - certificate path in container
  - key path in container
- optional server name / SNI value

#### VLESS + Reality

- inbound tag
- listen port
- public server address for exported links
- TLS template name
- handshake server
- handshake port
- short id, or allow script generation
- optional server name / SNI value
- VLESS flow value, following the current panel defaults and template behavior

#### Hysteria2

- inbound tag
- listen port
- public server address for exported links
- TLS template name
- certificate source mode:
  - generate self-signed content
  - use mounted file paths
- if using file paths:
  - certificate path in container
  - key path in container
- optional server name / SNI value
- protocol-specific transport fields only when required by existing defaults

## Compose Generation

The generated Compose file should preserve the current container runtime model:

- image from GHCR
- mounted database directory
- mounted certificate directory
- restart policy
- container entrypoint unchanged

Ports are generated from the selected inputs:

- panel port
- subscription port
- inbound listen port if a bootstrap inbound is created
- any extra reserved ports

Port generation must deduplicate identical values before writing Compose output.

## Initialization Strategy

### Stage 1: container deployment

The script writes Compose assets and starts the service.

### Stage 2: readiness check

The script waits until the panel API responds. This is required before running API-based bootstrap for TLS, clients, and inbounds.

### Stage 3: base settings

The script uses the container-managed CLI commands already present in the project for the base setup path:

- set panel port and path
- set subscription port and path
- set admin username and password

This preserves the existing operational behavior for settings and credentials.

### Stage 4: object bootstrap via API

Once the panel is reachable, the script logs in and uses the existing `/api/save` backend entrypoints to create:

- TLS template
- client
- inbound

This keeps bootstrap behavior aligned with the same save pipeline the panel UI uses, including config validation, object persistence, link generation, and core restarts.

## TLS Bootstrap Rules

Behavior for TLS, Hysteria2, and Reality must match the existing panel templates and generation flow as closely as possible.

### VLESS + TLS and Hysteria2

These bootstrap flows support two certificate source modes.

#### Mode A: generate self-signed content

The script calls the same API the panel uses for the TLS modal generate action:

- `GET /api/keypairs?k=tls&o=<server_name>`

The returned PEM blocks are split into:

- `certificate`
- `key`

The TLS object is then saved using inline file contents, matching the panel's "use file content" behavior.

#### Mode B: use mounted file paths

The script saves:

- `certificate_path`
- `key_path`

matching the panel's "use external path" behavior.

No ACME path is included in this feature. Panel access remains `IP:port`, and host-managed TLS or reverse proxying remains external to this workflow.

### VLESS + Reality

The script calls the same API the panel uses for Reality key generation:

- `GET /api/keypairs?k=reality`

The generated values populate:

- server-side `reality.private_key`
- client-side `reality.public_key`

Other Reality fields such as handshake target, short IDs, and server name are collected interactively or generated using the same practical defaults as the current panel flow.

## Template Consistency Requirements

The bootstrap payloads must follow the existing template behavior rather than inventing a new Docker-only configuration model.

This means the implementation should source defaults from the same frontend template assumptions used today, including:

- protocol preset structure
- TLS preset structure
- Reality enablement shape
- default ALPN values where already established
- default TLS version bounds where already established
- protocol-specific required fields only

The Docker bootstrap flow may prefill only the fields needed to produce one working, operator-visible result. It should avoid adding optional settings that the current UI does not default to.

## Client Bootstrap Rules

The script creates exactly one client in the first version.

The saved client object includes:

- `enable = true`
- `name`
- inbound association
- traffic `volume`
- `expiry`
- `autoReset`
- `resetDays`

Traffic and expiry values must be normalized into the backend's expected stored fields rather than kept as raw prompt text.

## Inbound Bootstrap Rules

The script creates exactly one inbound in the first version.

The inbound object includes:

- protocol type
- tag
- listen port
- any required addrs/public address information for link generation
- bound TLS template when relevant

The script must only include fields necessary for the chosen protocol and existing template defaults.

## Failure Handling

The Docker flow should prefer retention and inspectability over destructive rollback.

If deployment fails:

- keep generated Compose and `.env` files
- keep mounted data directories
- print the failed step clearly
- print suggested inspection commands

If base container startup succeeds but API bootstrap fails:

- do not delete the deployment
- report which object creation step failed
- point the operator to the running panel if available
- keep a bootstrap log if one was generated

The goal is to leave the operator with a debuggable environment rather than a partially hidden rollback.

## Verification Requirements

Implementation verification should cover:

- generated Compose file includes expected ports and mounts
- deployment script can start the container from a clean directory
- panel responds on the configured `IP:port`
- admin credentials are set correctly
- settings reflect the configured panel and subscription ports and paths
- optional bootstrap correctly creates one TLS template, one client, and one inbound
- protocol-specific key generation paths work:
  - self-signed TLS generation for `VLESS + TLS` and `Hysteria2`
  - Reality key generation for `VLESS + Reality`
- documentation examples match the generated deployment behavior

## Documentation Changes

### README

Add a short Docker deployment section that:

- points to the dedicated Docker installer
- explains that panel access defaults to `IP:port`
- explains that host HTTPS and reverse proxying are outside the installer

### Manual

Add a fuller Docker deployment section that explains:

- generated files and directories
- how the interactive bootstrap behaves
- protocol choices
- self-signed vs mounted cert behavior for TLS-based protocols
- that host Nginx and panel domain exposure are separate concerns

## Recommended First-Version Boundaries

To keep the first version reliable:

- bootstrap exactly one client and one inbound
- support only interactive mode first
- avoid editing existing user-managed Compose files in place
- write a fresh deployment directory instead
- refuse to overwrite existing generated files unless explicitly confirmed

## Open Decisions Resolved in This Design

- Docker support is implemented as a separate installer, not as `install.sh --docker`
- panel access is initialized as direct `IP:port`
- panel ACME is excluded
- host reverse proxy configuration is excluded
- TLS-based protocol bootstrap supports both generated certificate content and mounted file paths
- protocol bootstrap should mirror current template behavior instead of introducing Docker-specific presets
