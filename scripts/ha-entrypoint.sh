#!/bin/sh
# Entrypoint of the Home Assistant app image. Home Assistant mounts /data owned
# by root, so hand it to the app user, then drop root before starting.
set -eu

# -h changes symbolic links themselves rather than their targets, so a link
# placed in /data cannot make this root step take ownership of other files.
find /data ! -user shellyficator -exec chown -h shellyficator:shellyficator {} +

exec su-exec shellyficator:shellyficator shellyficator "$@"
