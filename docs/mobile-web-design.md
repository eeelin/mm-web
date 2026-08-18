# Mobile Web Design Direction

mm-web is a phone-first operations console for ModemManager. The interface
should help a user understand modem health quickly and recover from common
connection problems without needing a terminal.

## Design Personality

- Compact.
- Diagnostic.
- Calm.
- Appliance-like.
- Touch-friendly.
- The product shell is a virtual phone: users enter a familiar home screen and
  open focused system apps. System Settings exposes notification preferences,
  detected modem diagnostics, and runtime information about mmOS.

Avoid large hero sections, decorative cards, gradient backgrounds, marketing
copy, or empty dashboard ornamentation.

## First Screen

The virtual desktop is the first screen. It shows the active cellular network
at a glance and provides System Settings, Phone, Messages, and About icons.
Messages opens the real SMS experience; About shows build, runtime, service,
Push, PWA, and privacy status; Phone remains inactive until that capability
ships.
The Messages app refreshes while it is open. When installed as a PWA, it can
notify the user about newly received SMS messages while closed. Sender and
message text are hidden by default; an explicit System Settings switch can show
them in notifications. Incoming pushes set the app icon badge; opening Messages
or tapping the notification clears it.
Operational modem detail remains one tap away through System Settings instead
of being presented as a marketing dashboard. On mobile browsers, the product
fills the viewport without a simulated device frame, duplicate status bar, or
decorative home indicator.

The first screen should immediately answer:

- Is the modem present?
- Is the SIM usable?
- Is the modem registered?
- Is data connected?
- What is the signal quality?
- What failed most recently?
- What action can I take now?

Suggested top-level layout:

```text
Status header
  Modem name / connection state / refresh indicator

Primary status
  Connected, Registered, Searching, Disabled, Failed, or SIM Locked

Signal strip
  RSRP / RSRQ / SINR / RSSI

Connection card
  Operator / access technology / APN / bearer / IP

Actions
  Enable / Connect / Disconnect / Details

Event feed
  Recent state changes and errors
```

## Navigation

Use bottom navigation on mobile:

- Overview.
- Connection.
- SIM.
- Messages.
- Logs.

Desktop can convert the same sections into a left rail later.

## Visual System

- Background: light neutral surface for readability in field conditions.
- Accent: blue-green or cyan for active connectivity.
- Success: green.
- Warning: amber.
- Error: red.
- Disabled or unknown: gray.
- Border radius: 6px to 8px.
- Typography: system sans-serif with tabular numbers for metrics.

Signal metrics, timestamps, and counters should align cleanly. The interface
should not shift when values update.

## Interaction Rules

- Show pending states for slow operations such as connect and disconnect.
- Disable duplicate action taps while an operation is in flight.
- Dangerous or disruptive operations require confirmation.
- Errors should show both the ModemManager reason and a short human-readable
  explanation.
- The UI should keep showing stale last-known data while it attempts to
  reconnect to the backend.

## Mobile Constraints

- Primary controls must be reachable with one thumb.
- Important modem state must fit above the fold on common phone screens.
- Avoid wide tables on mobile.
- Prefer stacked metric rows, compact cards, and short event entries.
- Touch targets should be at least 44px high.
