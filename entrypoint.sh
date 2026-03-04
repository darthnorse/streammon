#!/bin/sh
PUID=${PUID:-10000}
PGID=${PGID:-10000}

groupmod -o -g "$PGID" streammon
usermod -o -u "$PUID" streammon

chown -R streammon:streammon /app/data /app/geoip
exec su -s /bin/sh streammon -c "./streammon"
