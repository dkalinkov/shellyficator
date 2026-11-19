# Shellyficator

Shellyficator keeps an inventory of your Shelly Gen2+ devices and warns when a
device's configuration differs from a baseline you approved.

This app is experimental. It opens inside Home Assistant, which signs you in;
only Home Assistant administrators can open it.

## First start

Start the app, then choose **Open Web UI**, or turn on **Show in sidebar**.
Turn on **Watchdog** on the Info tab so Home Assistant restarts the app if it
stops responding. Then choose **Discover** to find your devices. The
[getting started guide](https://github.com/dkalinkov/shellyficator/blob/main/docs/wiki/getting-started.md#add-and-review-devices)
explains baselines and change warnings.

## Configuration

**Shared device password:** the password for Shelly devices that have
authentication enabled, used with the device username `admin`. Shellyficator only
reads devices and never changes their passwords. Restart the app after changing it.

## Network

The app uses the host's network so it can find devices through mDNS and the
neighbor (ARP) cache. It listens on port 8090 only on Home Assistant's internal
network, so that port must be free on the host.

If your Shelly devices are on another network or VLAN, scan their subnet, such as
`192.168.1.0/24`, under **Advanced discovery**. See
[discovery](https://github.com/dkalinkov/shellyficator/blob/main/docs/wiki/discovery.md).

## Backups

The app's data is included in Home Assistant backups. The app stops for the
duration of a backup so its database is copied consistently, then starts again.

## Support

Report problems on [GitHub Issues](https://github.com/dkalinkov/shellyficator/issues)
with the app version and the app log, after removing passwords and private network
details.
