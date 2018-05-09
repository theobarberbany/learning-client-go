#!/bin/sh

set -euo pipefail

echo "Do my special initialization here then run the regular entrypoint"

exec docker-entrypoint.sh npm start
