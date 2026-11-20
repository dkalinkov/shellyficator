#!/bin/sh
set -eu

# Change symlink ownership without following links out of /data.
find /data ! -user shellyficator -exec chown -h shellyficator:shellyficator {} +

exec su-exec shellyficator:shellyficator shellyficator "$@"
