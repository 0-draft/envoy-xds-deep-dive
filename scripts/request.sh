#!/usr/bin/env bash
# Send N requests through an Envoy listener and tally the responses, so you can
# see load balancing across EDS endpoints at a glance.
#
# Usage:
#   ./request.sh [url] [count]          # defaults: http://localhost:10000/  10
#
# Example:
#   ./request.sh http://localhost:10000/ 12
set -euo pipefail

URL="${1:-http://localhost:10000/}"
COUNT="${2:-10}"

echo "sending ${COUNT} requests to ${URL}"
for _ in $(seq 1 "${COUNT}"); do
  curl -s --max-time 3 "${URL}" || echo "(request failed)"
done | sort | uniq -c | sort -rn
