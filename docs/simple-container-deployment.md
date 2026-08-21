# Simple Container Deployment

The first deployment mode keeps ModemManager on the host and runs mm-web as a
containerized D-Bus client.

## Host Requirements

- Linux host with ModemManager installed.
- ModemManager service running on the host.
- Docker or another OCI-compatible container runtime.
- The host system D-Bus socket available at `/run/dbus/system_bus_socket`.

## Compose Shape

```yaml
services:
  mm-web:
    image: yuhuntero/mm-web:latest
    ports:
      - "8080:8080"
    volumes:
      - /run/dbus/system_bus_socket:/run/dbus/system_bus_socket:ro
      - mm-web-data:/var/lib/mm-web
    environment:
      - DBUS_SYSTEM_BUS_ADDRESS=unix:path=/run/dbus/system_bus_socket
    restart: unless-stopped

volumes:
  mm-web-data:
```

This example is available as `compose.yaml` in the repository root. Run
`docker compose up -d` to start it and open `http://localhost:8080`.

Use a numbered image tag in production when reproducibility matters. The
release workflow publishes `latest` from `main`, semantic-version tags from Git
tags such as `v1.2.3`, and an immutable `sha-<commit>` tag for every image.

## Background message notifications

mm-web checks ModemManager for newly received SMS records in the backend and
sends a standards-based Web Push notification to subscribed browsers. By
default, the notification says only that a new message arrived. Operators can
explicitly enable message previews in System Settings; previews use a known
sender name when available, otherwise the phone number, and include the SMS
body. Preview content may be visible on the lock screen.

The application generates a VAPID key pair on first startup. Persist the
`/var/lib/mm-web` volume or existing browser subscriptions will stop working
after the keys change. `MM_WEB_VAPID_SUBJECT` may be set to an administrator
contact such as `mailto:admin@example.com`.

The public site must use HTTPS. For iOS/iPadOS, install the site to the Home
Screen first, open the installed mmOS app, then enable notifications from the
Messages screen. Browser tabs on iOS cannot subscribe without the Home Screen
installation.

## Modem health notifications

The backend checks ModemManager through the mounted system D-Bus every two
minutes by default. Operators can change the interval from the Detection app;
the setting persists in the application data directory. It reports D-Bus
availability, modem presence, and whether an AT modem's signal value is fresh.
The Detection app presents the latest result,
and `GET /api/health` exposes the same state under `modemManager`.

After three consecutive failures, mm-web sends one Web Push notification for
that incident. A successful check resets the alert so a later incident can
notify again. The monitor does not open modem device nodes or restart
ModemManager automatically, so the standard container mounts and permissions
remain unchanged.

## Permission Model

The container connects to the host system bus through the mounted Unix socket.
Read-only modem state is usually the easiest path to make work. Control actions
such as connect, disconnect, enable, disable, SIM operations, and SMS may require
host policy changes depending on the distribution and container runtime.

For the initial version, the image may run as root inside the container to keep
D-Bus credential handling simple. A hardened non-root mode can be added after
the basic integration works.

## What Not To Mount By Default

Do not mount modem device nodes by default:

- `/dev/ttyUSB*`
- `/dev/cdc-wdm*`
- `/dev/wwan*`

Do not require `--privileged` by default.

Those are only needed if ModemManager itself runs inside the container, which is
not the first-version architecture.

## Troubleshooting Checklist

1. Confirm the host sees the modem:

   ```bash
   mmcli -L
   ```

2. Confirm the host ModemManager service is running:

   ```bash
   systemctl status ModemManager
   ```

3. Confirm the D-Bus socket is mounted in the container:

   ```bash
   ls -l /run/dbus/system_bus_socket
   ```

4. Confirm write operations are not blocked by host policy.

If write operations fail while reads work, investigate Polkit and D-Bus policy
on the host before changing container device mounts.

## SMS write permission

Reading SMS may work while creating, sending, and deleting messages fails with
a PolicyKit authorization error. Native development processes are not treated
as an active desktop session, so the default ModemManager policy may reject
them.

The repository includes narrowly scoped rules for both current PolicyKit and
older `pklocalauthority` systems. They grant only
`org.freedesktop.ModemManager1.Messaging` to members of the `mm-web` group.
Install it for the current user with:

```bash
./scripts/install-polkit-rule.sh
```

Start a new login session after installation, or run the development server in
the new group immediately with:

```bash
sg mm-web -c 'npm run dev'
```

Do not replace this with a blanket ModemManager authorization rule. SMS write
access allows sending and deleting messages and should be limited to the
service account or administrators that run mm-web.
