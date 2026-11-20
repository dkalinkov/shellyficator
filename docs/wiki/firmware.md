# Firmware updates

Open a device's firmware indicator to review its update, or choose **Update devices…**
for the current filtered inventory. Stable releases are selected by default.
Select beta releases explicitly. Updates restart devices and can interrupt operation.

## What happens after confirmation

Shellyficator updates one device at a time. It checks the device identity and
offer, captures configuration, sends one command, and verifies the installed
version after restart before capturing again. Approved baselines stay in place.
Closing the browser leaves the batch running.

## Interpreting results

The installed version must match your selection. Shellyficator verifies the
version even if the command response is lost. A failed post-update capture is
reported separately from the installation result.

If the app restarts after the command may have been sent, the result is
unconfirmed. Check the installed version before retrying. Firmware commands
require explicit confirmation and there is no automatic rollback.
