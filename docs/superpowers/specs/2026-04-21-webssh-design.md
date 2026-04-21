# Admin-Only WebSSH Design

Date: 2026-04-21
Status: Draft approved in conversation, updated for review
Scope: `src/backend/` and `src/frontend/` WebSSH access, routing, and terminal interaction

## Goal

Add a WebSSH feature to the panel frontend that is visible and usable only after an administrator logs in.

The change must ensure:

- only the first user in the database is treated as the administrator
- only that administrator can see and open the WebSSH panel
- backend authorization enforces the same rule even if a user manually navigates or calls the API
- the terminal can send commands and display command output back to the user in near real time
- the first version stays small and avoids a broader remote-host management system

## Non-Goals

- Introducing a general role-based access control model
- Adding a full multi-user permission system to the existing admin pages
- Supporting multiple SSH targets, host pickers, or per-user remote credentials
- Shipping a full PTY-grade terminal emulator with resize handling, alternate screen support, or interactive TUIs in the first iteration
- Allowing the frontend to submit raw SSH passwords or private keys directly

## Current-State Findings

The current codebase already provides the pieces needed for a minimal admin-gated addition, but it does not yet have a role model or WebSSH transport.

- `src/backend/internal/http/api/session.go` stores only the logged-in username in the session and exposes `GetLoginUser()` and `IsLogin()`.
- `src/backend/internal/domain/services/user.go` authenticates users and exposes `GetFirstUser()`, which is the cleanest existing way to identify the system administrator without changing the data model.
- `src/backend/internal/infra/web/web.go` already enforces login for non-login frontend routes, so route-level login gating exists today.
- `src/frontend/src/router/index.ts` uses a single authenticated shell with route-based views, including existing admin-focused pages such as `/admins` and `/settings`.
- The frontend currently has no explicit permission state for "administrator" and no WebSocket-based terminal UI.

That means the smallest consistent design is:

1. Define administrator as the first user record.
2. Add a lightweight backend endpoint for frontend permission discovery.
3. Add a dedicated admin-only WebSSH route and menu item.
4. Add a backend WebSocket endpoint that bridges browser input to an SSH-backed shell session and streams output back.

## Requirements

### Functional Requirements

1. The system must treat the first user record in the database as the administrator.
2. Only a logged-in administrator may see the WebSSH navigation entry in the frontend.
3. Only a logged-in administrator may open the WebSSH page.
4. Only a logged-in administrator may connect to the WebSSH backend endpoint.
5. The frontend must allow the administrator to connect to a shell session from within the panel.
6. The frontend must allow the administrator to submit commands through the terminal UI.
7. The backend must execute commands through the SSH-backed session and stream output back to the frontend.
8. The frontend must display command output and connection status as the session progresses.
9. If the session disconnects or fails, the frontend must preserve previous output and show a recoverable error state.

### Security Requirements

1. Frontend visibility alone is not sufficient; backend authorization must independently reject non-admin access.
2. The frontend must not send raw SSH credentials from the browser.
3. SSH credentials or connection material must be sourced by the backend from local controlled configuration.
4. The backend must enforce connection cleanup on disconnect, timeout, and server shutdown.
5. The backend must limit idle or abandoned sessions so WebSSH cannot remain open indefinitely.

### UI Requirements

1. The WebSSH entry must appear only for the administrator after login state is known.
2. The WebSSH page must clearly show disconnected, connecting, connected, and error states.
3. The terminal UI must include an output area, an input path for commands, and a reconnect action.
4. The page should match the existing operations-console visual language rather than introducing a separate theme.

## Design

### 1. Administrator Identity Rule

Do not add a new role field in this feature.

The backend should add one focused helper that answers whether the current logged-in username matches the first user record returned by `UserService.GetFirstUser()`.

This rule should be reused in:

- the new lightweight permission/introspection endpoint used by the frontend
- the HTTP route guard for the WebSSH page data if needed
- the WebSocket upgrade path for the terminal session

If the first user cannot be loaded, the request should be treated as unauthorized for WebSSH rather than falling back to permissive behavior.

### 2. Frontend Permission Discovery

Add a small authenticated API response that tells the frontend enough to decide whether to show admin-only UI.

Recommended response shape:

```json
{
  "username": "admin",
  "isAdmin": true
}
```

The frontend should request this once after login or on app initialization inside the authenticated shell, then store the result in shared state already used by the layout or a small new permission store.

The WebSSH menu item and route entry should render only when `isAdmin` is true.

This keeps the browser behavior aligned with the backend rule and avoids hardcoding `username === 'admin'`, which would be wrong for this project.

### 3. Navigation And Route Design

Add a new route in `src/frontend/src/router/index.ts` under the authenticated shell, for example `/webssh`.

Add a matching navigation item in the existing shell menu, but only render it when the shared permission state says the current user is the administrator.

If a non-admin manually enters `/webssh`, the page should not expose terminal functionality. The frontend may redirect away once permission state loads, but the hard security boundary remains the backend WebSSH authorization.

### 4. Terminal Session Architecture

The recommended implementation is a backend-managed WebSocket session that bridges browser messages to one long-lived SSH shell session.

This is preferred over per-command HTTP execution because it preserves shell context across commands, including:

- current working directory
- exported environment variables
- shell history inside the active session
- incremental output streaming

The backend should create the SSH session after the WebSocket is upgraded and the admin check passes. The browser then sends structured messages to that session and receives streamed events back.

### 5. SSH Target And Credential Scope

The first iteration should connect to one fixed administrative SSH target only.

Preferred default: the panel's own managed server environment, using backend-controlled connection material read from local configuration.

This keeps the feature small and avoids turning WebSSH into a host inventory system.

The frontend must not choose arbitrary remote hosts in this scope.

### 6. Browser-To-Server Message Model

Use structured JSON messages over WebSocket rather than raw terminal byte forwarding in the first iteration.

Recommended message types:

- `connect`: optional frontend signal to request session start if the backend does not auto-start after upgrade
- `input`: frontend sends a command or input line
- `output`: backend sends command output chunks
- `status`: backend sends lifecycle events such as connecting, connected, disconnected, unauthorized, auth_failed, or error
- `close`: either side indicates the session is ending

Recommended payload examples:

```json
{ "type": "input", "data": "pwd\n" }
```

```json
{ "type": "output", "stream": "stdout", "data": "/root\n" }
```

```json
{ "type": "status", "status": "connected" }
```

This structure keeps status reporting explicit and makes the frontend easier to test.

### 7. First-Version Interaction Model

The first version should support command submission and output replay without attempting full terminal emulation.

That means:

- users can type a command and submit it
- the backend writes that command to the active shell session
- stdout and stderr are streamed back and appended to the output pane
- connection and error states are rendered distinctly

The page does not need to support advanced TUI programs such as `top`, `vim`, or full-screen ncurses applications in this scope.

This keeps the implementation aligned with the user's required outcome: execute commands and echo results.

### 8. Frontend Page Structure

Add a new `Webssh.vue` view or equivalent page component in the same style family as the other operational pages.

Recommended page structure:

- page header with `WebSSH` title and short admin-only description
- status strip showing disconnected, connecting, connected, or error
- scrollable output panel using monospace styling
- command input area with submit action
- reconnect and disconnect controls

Behavior expectations:

1. On first load, the page shows disconnected state.
2. The admin can click connect to start the session.
3. Commands append to the transcript in order.
4. Output chunks append below prior output without clearing history.
5. If the socket drops, history remains visible and reconnect becomes available.

### 9. Backend Session Lifecycle

The backend WebSSH service should explicitly manage:

- WebSocket connection lifetime
- SSH client lifetime
- SSH session lifetime
- output reader goroutines
- idle timeout
- cleanup on browser disconnect and server shutdown

Expected lifecycle:

1. Verify session login.
2. Verify current user is the first user.
3. Upgrade to WebSocket.
4. Open SSH client and shell session.
5. Start stdout and stderr readers.
6. Forward browser input into the shell.
7. Emit status and output events.
8. Close all resources once any side terminates or times out.

### 10. Error Handling

The design should treat errors as explicit session states, not generic silent failures.

Backend conditions that should produce structured failure states include:

- not logged in
- logged in but not administrator
- SSH authentication failure
- SSH connection timeout
- broken WebSocket connection
- shell session creation failure

Frontend behavior for each failure should:

- preserve output history already shown
- show a clear status message
- allow reconnect when the error is transient

For unauthorized states, the frontend should stop offering interactive controls after the denial is known.

## File Scope

Expected backend changes:

- `src/backend/internal/domain/services/user.go` or a nearby auth helper area for first-user admin checks
- `src/backend/internal/http/api/` for a permission/introspection endpoint and WebSSH handler wiring
- `src/backend/internal/infra/web/web.go` or nearby routing setup only if route registration changes are needed there
- one or more new backend files for WebSSH session management and WebSocket-to-SSH bridging

Expected frontend changes:

- `src/frontend/src/router/index.ts`
- layout or drawer files that define authenticated navigation
- one new frontend view for the WebSSH page
- shared state for login-derived admin capability if no suitable store already exists

Expected tests to be added:

- backend tests covering admin authorization and session message flow
- frontend tests covering admin-only rendering and terminal state updates

## Testing Strategy

### Backend Tests

Follow TDD for each backend behavior addition.

Minimum required tests:

1. A failing test proving non-logged-in requests cannot open the WebSSH endpoint.
2. A failing test proving logged-in non-admin users cannot open the WebSSH endpoint.
3. A failing test proving the first user can pass authorization.
4. A failing test proving an input message results in output forwarding through the session abstraction.

To keep these tests reliable, isolate SSH transport behind a small interface so tests can use a fake shell bridge rather than a real SSH server.

### Frontend Tests

Follow TDD for each frontend behavior addition.

Minimum required tests:

1. A failing test proving the WebSSH navigation item is hidden when `isAdmin` is false.
2. A failing test proving the WebSSH navigation item is shown when `isAdmin` is true.
3. A failing test proving the WebSSH page renders connection state transitions.
4. A failing test proving output messages are appended to the terminal transcript.

### Manual Verification

1. Log in as the first user.
Expected result: the WebSSH entry is visible.

2. Open the WebSSH page as the first user and connect.
Expected result: status changes from disconnected to connected.

3. Run a simple command such as `pwd` or `whoami`.
Expected result: the command output appears in the output panel.

4. Log in as a non-first user.
Expected result: the WebSSH entry is not visible.

5. Attempt to reach the WebSSH backend as a non-first user by direct request.
Expected result: the request is rejected by backend authorization.

6. Disconnect the browser session or stop the socket.
Expected result: prior output remains visible and the UI offers recovery rather than blanking the panel.

## Open Questions

No open questions remain for the approved scope.
