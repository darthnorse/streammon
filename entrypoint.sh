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
# the chown below is what set it that way. Skip it in that case. /app/data
# and /app/geoip are checked (and chowned) independently, since a fresh
# mount on one (e.g. a new geoip volume added to an already-correct data
# volume) can leave their ownership out of sync with each other. Each dir
# is guarded with -d since a missing dir would make `stat` fail under
# set -e.
for dir in /app/data /app/geoip; do
    if [ -d "$dir" ]; then
        current_owner=$(stat -c '%u:%g' "$dir")
        if [ "$current_owner" != "${PUID}:${PGID}" ]; then
            chown -R streammon:streammon "$dir"
        fi
    fi
done

exec su -s /bin/sh streammon -c "./streammon"
