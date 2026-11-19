# Shellyficator

Shellyficator keeps an inventory of your Shelly Gen2+ devices and warns when a
device's configuration differs from a baseline you approved.

This app is experimental. It opens in its own browser tab on port 8090 and uses
its own login; it does not appear in the Home Assistant sidebar yet.

## First start

1. Start the app, then choose **Open Web UI**.
2. Create your account. Passwords need at least 12 characters.

Do this right after the first start: until an account exists, anyone who can
reach port 8090 on your Home Assistant host can create it.

Setup also creates a separate data administrator at `http://<home-assistant-host>:8090/_/`
with the same initial credentials. It can edit every record; use it only for
maintenance.

Then choose **Discover** to find your devices. The
[getting started guide](https://github.com/dkalinkov/shellyficator/blob/main/docs/wiki/getting-started.md#add-and-review-devices)
explains baselines and change warnings.

## Configuration

**Shared device password:** the password for Shelly devices that have
authentication enabled, used with the device username `admin`. Shellyficator only
reads devices and never changes their passwords. Restart the app after changing it.

## Network

The app uses the host's network so it can find devices through mDNS and the
neighbor (ARP) cache. It listens on port 8090 on every network interface of the
Home Assistant host, so that port must be free. Home Assistant's remote access
does not reach it.

If devices are missing, use **Advanced discovery** with a device address or a
private subnet such as `192.168.1.0/24`. See
[discovery](https://github.com/dkalinkov/shellyficator/blob/main/docs/wiki/discovery.md).

## Backups

The app's data is included in Home Assistant backups. The app stops for the
duration of a backup so its database is copied consistently, then starts again.

## Support

Report problems on [GitHub Issues](https://github.com/dkalinkov/shellyficator/issues)
with the app version and the app log, after removing passwords and private network
details.
