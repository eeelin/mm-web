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
    build: .
    ports:
      - "8080:8080"
    volumes:
      - /run/dbus/system_bus_socket:/run/dbus/system_bus_socket
    environment:
      - DBUS_SYSTEM_BUS_ADDRESS=unix:path=/run/dbus/system_bus_socket
    restart: unless-stopped
```

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
