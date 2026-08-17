# Project Decisions

This document records the initial decisions for mm-web so implementation can
start from a shared baseline.

## 1. Use the simple container deployment model

Decision: mm-web will start with a single container that connects to the host
system D-Bus socket.

Rationale:

- The host already has the right place to run ModemManager.
- The container does not need direct access to modem device nodes.
- The setup stays understandable for homelab and appliance-style deployments.

Non-goals for the first version:

- Running ModemManager itself inside the container.
- Requiring `--privileged` by default.
- Building a separate host agent.

The host-agent architecture remains a future option if D-Bus permissions,
multi-user control, or audit requirements become more complex.

## 2. Treat ModemManager as the source of truth

Decision: runtime modem state should come from ModemManager over D-Bus.

The application may store local UI preferences, sampled signal history, and
operation logs, but it should not maintain an independent modem state machine
that competes with ModemManager.

## 3. Prefer direct D-Bus integration over shelling out to mmcli

Decision: the backend should use a D-Bus client library as the primary
integration path.

`mmcli` can still be useful for debugging and manual verification, but the
application should not parse command output as its core API.

Rationale:

- D-Bus gives structured objects, properties, methods, and signals.
- It avoids fragile text parsing.
- It makes live updates cleaner.

## 4. Build mobile-first

Decision: the primary interface is a mobile Web app.

The first screen should be useful on a phone during modem debugging. Desktop
layouts can expand from the same information model, but desktop is not the
primary design target.

## 5. Use a network appliance console style

Decision: mm-web should look and behave like a compact device management
console.

The design should be calm, dense, and status-forward. It should not look like a
landing page, a marketing site, or a decorative dashboard.

## 6. Start with REST plus server-sent events

Decision: use REST endpoints for commands and server-sent events for live
status updates.

Rationale:

- REST is simple for control actions.
- SSE is enough for one-way modem status updates.
- WebSocket can be added later if bidirectional realtime interaction becomes
  necessary.

## 7. First-release feature boundary

The first implementation should focus on:

- Overview.
- Modem details.
- Signal and registration state.
- Bearer connect and disconnect.
- SMS conversation list, sending, and deletion through ModemManager.
- Clear error reporting.

SIM PIN operations, USSD, GPS/location, alerts, and historical analytics
can follow after the control loop is proven.

## 8. Organize the Go backend by concern

Decision: keep `cmd/server/main.go` limited to process bootstrap and place the
HTTP server implementation in `internal/server`.

Within `internal/server`, route composition, shared D-Bus access, response and
static-file helpers, and each product area live in separate files. New backend
features should normally add a focused feature file instead of growing the
entry point or introducing a package before it has a distinct responsibility.
