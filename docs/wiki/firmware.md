# Firmware updates

Open the firmware indicator in the inventory or a device's firmware panel to
review an offered update. **Update devices…** builds a selection from the current
filtered view. Stable releases are selected by default; beta releases require
explicit selection. Confirm the named devices and versions before installation.
Updates restart devices and can interrupt their operation.

## What happens after confirmation

1. Updates run one device at a time.
2. The app rechecks device identity and the offered release.
3. It captures current configuration, then sends one update command.
4. It checks the running version for up to two minutes after restart.
5. Once installation is verified, it captures configuration again.

Approved baselines stay unchanged. Closing the dialog or browser does not cancel
an accepted update. Progress and results are retained in storage and displayed
in firmware status indicators.

## Interpreting results

An accepted OTA request alone is not success. The running version must match the
selected version. An unchanged version is a failure even if the device reported
OTA success. A lost command response triggers verification without repeating the
write. A failure to capture settings after installation is reported separately
from a verified firmware installation.

After an app restart, interrupted attempts are marked unconfirmed. Check the
installed version before retrying. Shellyficator does not automatically retry
firmware writes or roll back firmware.
