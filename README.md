# mm-web

Mobile-first Web console for hosts running ModemManager.

## Product Direction

mm-web is a compact modem operations panel for phones and small screens. It
should feel like a network appliance console: fast to scan, calm under failure,
and focused on diagnosis and control instead of marketing or decoration.

The first release targets a simple deployment model:

```text
Browser
  -> mm-web container
  -> host /run/dbus/system_bus_socket
  -> host ModemManager
  -> modem device
```

The container acts as a D-Bus client. The host keeps ownership of
ModemManager, udev, and the modem devices.

## MVP Scope

- Read live modem inventory and status directly from the host ModemManager
  service over system D-Bus.
- Read, search, send, and delete SMS messages through ModemManager.
- List detected modems.
- Show modem registration, access technology, operator, SIM status, and bearer
  status.
- Show mmcli-style device diagnostics including identifiers, power and
  registration state, supported modes and IP families, ports, drivers, plugin,
  own numbers, and SIM unlock retries.
- Show signal values such as RSSI, RSRP, RSRQ, and SINR when available.
- Connect and disconnect a modem bearer.
- Surface ModemManager errors in a form that is useful during field debugging.
- Provide a mobile-first overview page that works well on a phone.

The current development UI presents this information as a full-screen virtual
phone. System Settings opens the detected modem list and device details. The
Messages app groups real SMS records into conversations and supports composing,
sending, and deleting conversations. It refreshes while the Messages app is
open and can deliver privacy-preserving Web Push notifications for newly
received SMS messages while the frontend is closed. Phone provides real
ModemManager Voice dialing, live call state, hangup, and in-call DTMF controls;
audio remains on the host/modem voice path rather than passing through the browser.
Incoming pushes also mark the installed PWA icon; opening Messages or tapping
the notification clears the app badge.
System Settings includes a message-preview switch. It is off by default; when
enabled, notifications use the sender phone number as the title and SMS text as
the body. The setting is stored in the persistent application data directory.
The About screen reports build and runtime details, ModemManager and Push
status, PWA/notification state, and privacy behavior. Its diagnostic copy action
intentionally excludes phone numbers, IMEI values, SMS text, Push endpoints,
and keys.

## Deployment Sketch

```yaml
services:
  mm-web:
    image: yuhuntero/mm-web:latest
    ports:
      - "8080:8080"
    volumes:
      - /run/dbus/system_bus_socket:/run/dbus/system_bus_socket
      - mm-web-data:/var/lib/mm-web
    environment:
      - DBUS_SYSTEM_BUS_ADDRESS=unix:path=/run/dbus/system_bus_socket
    restart: unless-stopped

volumes:
  mm-web-data:
```

The host must already have ModemManager installed and running.

The repository includes the same configuration in `compose.yaml`. Start it
with:

```bash
docker compose up -d
```

Published images support `linux/amd64` and `linux/arm64`. Pull the current main
branch build with `docker pull yuhuntero/mm-web:latest`. Version tags such as
`v1.2.3` publish `1.2.3` and `1.2` image tags; every release also gets an
immutable `sha-<commit>` tag.

For local development, install the frontend dependencies and start the Vite UI
and Go API together:

```bash
npm install
npm run dev
```

The UI is served on `http://localhost:5173`; Vite proxies `/api` to the Go
server on `127.0.0.1:8080`. The API exposes `GET /api/health`,
`GET /api/modems`, `GET /api/messages`, `POST /api/messages`, and
`DELETE /api/messages/{id}`, plus push subscription endpoints under
`/api/push`. Go and access to the host system D-Bus are required.

Web Push requires HTTPS outside localhost. On iPhone and iPad, add mmOS to the
Home Screen, open that installed app, enter Messages, and tap the bell to grant
notification permission. The server creates VAPID keys on first startup and
stores keys and browser subscriptions in `MM_WEB_DATA_DIR` (`/var/lib/mm-web`
in the image), so that directory must persist across container replacements.

For staging diagnostics, set `MM_WEB_DEBUG_PUSH_TOKEN` on the API and run
`MM_WEB_DEBUG_PUSH_TOKEN=... scripts/debug-push.sh`. The endpoint is disabled
when the token is unset and reports the subscription/delivery counts without
exposing subscription details.

## CI and releases

Pull requests and pushes to `main` run Go vet/tests (including the race
detector), build the frontend, and build the container without publishing it.
Pushes to `main`, version tags matching `v*`, and manual release runs publish a
multi-architecture image to `yuhuntero/mm-web` on Docker Hub after all tests
pass.

Configure these GitHub Actions repository secrets before publishing:

- `DOCKERHUB_USERNAME`: Docker Hub user allowed to push `yuhuntero/mm-web`.
- `DOCKERHUB_TOKEN`: Docker Hub personal access token; do not use the account
  password.

## Documents

- [Agent guidance](AGENTS.md)
- [Project decisions](docs/project-decisions.md)
- [Mobile Web design direction](docs/mobile-web-design.md)
- [Deployment notes](docs/simple-container-deployment.md)
