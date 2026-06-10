#!/usr/bin/env bash
# Atomically swap an xDS file *inside the Envoy container* and let Envoy hot-reload it.
#
# Why not just edit the file on the host?
#   Envoy watches the xDS directory with inotify. On Docker Desktop / Rancher Desktop,
#   host-side file edits do NOT propagate inotify events into the Linux VM, so Envoy
#   never notices. Performing the move inside the container fires the event in the same
#   kernel Envoy runs in, so the reload is reliable on every platform.
#
# Usage:
#   ./reload.sh eds.yaml ./variants/eds-one-endpoint.yaml
#   ./reload.sh eds.yaml -            # read new content from stdin
set -euo pipefail

target="${1:?usage: reload.sh <xds-file-name> <source-file|->}"
source="${2:?usage: reload.sh <xds-file-name> <source-file|->}"
svc="${ENVOY_SERVICE:-envoy}"
dir="/etc/envoy/xds"

if [[ "$source" == "-" ]]; then
  content="$(cat)"
else
  content="$(cat "$source")"
fi

# Write to a temp file in the watched dir, then mv it over the target (atomic move-in).
printf '%s\n' "$content" | docker compose exec -T "$svc" sh -c \
  "cat > $dir/.reload.tmp && mv $dir/.reload.tmp $dir/$target"

echo "swapped $target inside container '$svc'. Envoy will hot-reload within ~1s."
echo "check:  curl -s localhost:9901/config_dump | grep version_info"
