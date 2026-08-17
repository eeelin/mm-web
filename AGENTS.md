# AGENTS.md

This file captures the project choices and working principles for future agents
and contributors. Read it before making architecture, product, or UI changes.

## Product Shape

mm-web is a mobile-first Web console for ModemManager. It is an operations tool,
not a marketing site.

The default user is holding a phone and trying to understand or recover a modem
connection. Optimize for fast diagnosis, clear state, and safe control actions.

## Architecture Decisions

- Use the simple container deployment model first.
- Keep ModemManager running on the host.
- Let the container access the host system D-Bus socket at
  `/run/dbus/system_bus_socket`.
- Do not run ModemManager inside the application container by default.
- Do not require `--privileged` by default.
- Do not mount modem device nodes by default.
- Treat ModemManager as the source of truth for runtime modem state.
- Use direct D-Bus integration as the primary backend path.
- Keep `mmcli` as a debug and verification aid, not as the main API layer.
- Use REST for commands and server-sent events for live status updates.

## Technology Choices

Initial implementation choices:

- Frontend: React, Vite, and TypeScript.
- Backend: Go.
- Runtime integration: Go D-Bus client talking to
  `org.freedesktop.ModemManager1`.
- Packaging: Docker image plus Docker Compose example.
- Persistence: no required database for the MVP.

Allowed later additions:

- SQLite for signal history, operation logs, user preferences, and SMS cache.
- WebSocket if bidirectional realtime behavior becomes necessary.
- A host-side agent if the simple D-Bus mount becomes too limiting.

Avoid adding these before there is a concrete need:

- GraphQL.
- A full user/account system.
- Kubernetes-specific deployment.
- A custom modem state machine that competes with ModemManager.

## UI Principles

- Build mobile-first.
- The first screen must show useful modem state immediately.
- Use a compact network-appliance console style.
- Prefer dense but readable controls over large decorative panels.
- Do not build a landing page as the first experience.
- Avoid hero sections, decorative gradients, and promotional copy.
- Use stable layouts so live metric updates do not shift the page.
- Keep primary touch targets at least 44px high.
- Prefer bottom navigation on mobile.
- Make pending states obvious for slow operations.
- Require confirmation for disruptive operations.

## State And Error Handling

- Preserve last-known data while reconnecting to the backend.
- Represent intermediate modem states explicitly.
- Surface raw ModemManager reasons when they are useful.
- Add a short human-readable explanation next to raw errors.
- Use timeouts for every control action.
- Disable duplicate taps while an action is in flight.

## MVP Boundary

Focus the first version on:

- Modem list.
- Overview.
- Registration and access technology.
- Operator and SIM status.
- Signal values.
- Bearer status.
- Connect and disconnect.
- Read, send, and delete SMS messages.
- Recent events and errors.

Defer until after the main control loop works:

- SIM PIN operations.
- USSD.
- GPS/location.
- Alerts and webhooks.
- Prometheus metrics.
- Historical analytics.

## Documentation Discipline

When changing a core choice, update the relevant document:

- `docs/project-decisions.md` for architecture and scope.
- `docs/simple-container-deployment.md` for deployment behavior.
- `docs/mobile-web-design.md` for UX and visual direction.
- `AGENTS.md` for standing guidance that future agents must follow.
