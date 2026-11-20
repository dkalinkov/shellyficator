# Shellyficator

## First start

Start the app and choose **Open Web UI** or **Show in sidebar**. Turn on
**Watchdog** under **Info** to restart the app if it hangs. Choose **Discover**
to find devices. [Getting started](https://github.com/dkalinkov/shellyficator/blob/main/docs/wiki/getting-started.md#add-and-review-devices)

## Configuration

For protected devices, set **Shared device password** and restart the app.
Shellyficator uses the password with the device username `admin` for reads
and firmware updates you confirm.

## Network

Keep port 8090 free on the host. For devices across VLANs, use **Advanced discovery**
to scan their subnet. Shellyficator remembers subnets where it finds devices.
[Discovery guide](https://github.com/dkalinkov/shellyficator/blob/main/docs/wiki/discovery.md)

## Backups

Home Assistant stops the app while backing up its data.

## Support

Include the app version and sanitized log when reporting a problem.
[GitHub Issues](https://github.com/dkalinkov/shellyficator/issues)
