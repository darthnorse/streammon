#!/bin/sh
chown -R streammon:streammon /app/data /app/geoip
exec su -s /bin/sh streammon -c "./streammon"
