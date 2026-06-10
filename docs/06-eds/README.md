**English** | [日本語](README.ja.md)

# 06 — EDS (Endpoint Discovery Service)

EDS discovers **ClusterLoadAssignments**: the concrete list of endpoint IPs and
ports that back a cluster, plus their health and locality. It is the bottom of
the dependency chain and the data that changes most often.

```mermaid
flowchart LR
    L[Listener LDS] --> R[RouteConfiguration RDS]
    R --> C[Cluster CDS]
    C --> E[ClusterLoadAssignment EDS]
    style E stroke-width:3px
```

## What a ClusterLoadAssignment contains

- **cluster_name**: must match the cluster's `eds_cluster_config.service_name`.
- **endpoints**: grouped by **locality** (region/zone), each group holding a
  list of **lb_endpoints**, each with a `socket_address` (the IP and port).
- Per-endpoint **health_status** and **load_balancing_weight**.

```yaml
- "@type": type.googleapis.com/envoy.config.endpoint.v3.ClusterLoadAssignment
  cluster_name: service_backend       # <- matches the CDS cluster
  endpoints:
    - lb_endpoints:
        - endpoint:
            address:
              socket_address: { address: 10.77.0.11, port_value: 5678 }
        - endpoint:
            address:
              socket_address: { address: 10.77.0.12, port_value: 5678 }
```

## EDS endpoints are IPs, not names

This trips people up. A `STRICT_DNS` cluster resolves a hostname itself. An `EDS`
cluster does **not** resolve DNS — the control plane is expected to hand Envoy
already-resolved IPs. That is exactly what a control plane does in Kubernetes:
it watches the API for pod IPs and pushes them as EDS.

This repo mirrors that:

- Labs 01 and 02 pin upstream containers to fixed IPs and list those IPs in EDS.
- Lab 03's control plane resolves a headless Service to get **pod IPs** and
  pushes them as EDS — the realistic pattern.

## Why EDS is split from CDS (the payoff)

Endpoints are the churn. Pods scale from 2 to 3, a node dies, a health check
flips. Each of those should update *only* the endpoint list — not re-evaluate the
cluster's TLS config, not redo the route table, not touch the listener.

You can watch this isolation directly. In Lab 03, scaling `app-b` from 2 to 3
pods produces exactly one kind of push:

```text
app-b endpoints changed -> [10.244.1.3 10.244.1.4 10.244.1.7]
PUSH node=app-a-sidecar version=4 (cds=1 eds=1 rds=1 lds=1 resources)
ACK  ClusterLoadAssignment version="4"
```

The caller's Envoy load-balances over the new set within seconds, no restart.

## Dependency rules

- EDS is sent **after** CDS on the ADS stream: the cluster must exist before its
  load assignment.
- An EDS update naming a `cluster_name` Envoy does not have is ignored.
- Removing all endpoints is legal; the cluster then has no healthy hosts and
  returns 503. (This is how you drain a backend gracefully.)

## Inspecting it

```bash
# Endpoints + health for a cluster (the runtime view)
curl -s localhost:9901/clusters | grep -E 'service_backend.*(::|health)'

# The exact ClusterLoadAssignment Envoy holds, with its version
curl -s localhost:9901/config_dump?resource=dynamic_endpoint_configs
```

## Gotchas

- **`cluster_name` mismatch**: if it does not equal the cluster's `service_name`,
  the endpoints silently never attach. Always cross-check those two strings.
- **Health vs membership**: EDS can mark an endpoint `UNHEALTHY` instead of
  removing it, which keeps it visible but out of rotation. Active health checks
  (on the cluster) and EDS health status interact.
- **All endpoints gone = 503, not an error**: an empty load assignment is a valid
  state, so you will not see a NACK — you will see traffic fail. Watch
  `/clusters` health, not just the ACK.

## Try it

In [Lab 01](../../labs/01-filesystem-xds/README.md) you remove an endpoint from
`xds/eds.yaml` and watch the cluster shrink. In
[Lab 02](../../labs/02-grpc-control-plane/README.md) you `POST /scale?n=1` and see
EDS push live. In [Lab 03](../../labs/03-pod-to-pod-kind/README.md) you
`kubectl scale` real pods and watch EDS follow them. Next:
[07 — Pod-to-pod](../07-pod-to-pod/README.md).
