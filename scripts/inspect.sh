#!/usr/bin/env bash
# Pretty-print the most useful views of an Envoy admin interface, grouped by xDS
# API so you can see what each discovery service produced.
#
# Usage:
#   ./inspect.sh [admin-host:port]      # default localhost:9901
#
# Examples:
#   ./inspect.sh                        # labs 00-02 (Envoy admin on localhost:9901)
#   ./inspect.sh localhost:9901
set -euo pipefail

ADMIN="${1:-localhost:9901}"
base="http://${ADMIN}"

hr() { printf '\n=== %s ===\n' "$1"; }

hr "server (${ADMIN})"
curl -s "${base}/server_info" | grep -E '"version"|"state"' || true

hr "LISTENERS (LDS)"
curl -s "${base}/listeners"

hr "CLUSTERS + ENDPOINTS (CDS/EDS)"
# cluster name :: endpoint :: health flags
curl -s "${base}/clusters" | grep -E '::(health_flags|cx_active|10\.|127\.|172\.|[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+:)' | head -40

hr "DYNAMIC CONFIG VERSIONS (ACK state)"
# version_info is the config version Envoy has successfully applied (the ACK).
curl -s "${base}/config_dump" | grep -E '"version_info"' | sort | uniq -c

hr "UPDATE STATS (ACK = success, NACK = rejected)"
curl -s "${base}/stats" | grep -E '\.(update_success|update_rejected|update_attempt):' | grep -vE ' 0$' || echo "(no non-zero update stats)"
