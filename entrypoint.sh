#!/bin/sh
# groupmod/usermod are no-ops when PUID/PGID already match streammon's
# current uid/gid (the common case on every restart after the first), but
# some shadow-utils versions can still return non-zero in that situation --
# their failures are intentionally non-fatal so a harmless remap doesn't
# block startup. Every other command (stat, chown, su) must succeed under
# set -e: those failures are real and should stop the container.
set -euo pipefail

PUID=${PUID:-10000}
PGID=${PGID:-10000}

groupmod -o -g "$PGID" streammon || true
usermod -o -u "$PUID" streammon || true

# chown -R is slow on large or network-mounted volumes and unnecessary once
# ownership already matches -- true on every start after the first, since
# the chown below is what set it that way. Skip it in that case.
current_owner=$(stat -c '%u:%g' /app/data)
if [ "$current_owner" != "${PUID}:${PGID}" ]; then
    chown -R streammon:streammon /app/data /app/geoip
fi

exec su -s /bin/sh streammon -c "./streammon"
