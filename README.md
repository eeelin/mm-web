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

- List detected modems.
- Show modem registration, access technology, operator, SIM status, and bearer
  status.
- Show signal values such as RSSI, RSRP, RSRQ, and SINR when available.
- Connect and disconnect a modem bearer.
- Surface ModemManager errors in a form that is useful during field debugging.
- Provide a mobile-first overview page that works well on a phone.

## Deployment Sketch

```yaml
services:
  mm-web:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - /run/dbus/system_bus_socket:/run/dbus/system_bus_socket
    environment:
      - DBUS_SYSTEM_BUS_ADDRESS=unix:path=/run/dbus/system_bus_socket
    restart: unless-stopped
```

The host must already have ModemManager installed and running.

## Documents

- [Agent guidance](AGENTS.md)
- [Project decisions](docs/project-decisions.md)
- [Mobile Web design direction](docs/mobile-web-design.md)
- [Deployment notes](docs/simple-container-deployment.md)
