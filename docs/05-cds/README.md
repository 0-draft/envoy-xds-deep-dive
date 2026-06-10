**English** | [日本語](README.ja.md)

# 05 — CDS (Cluster Discovery Service)

CDS discovers **Clusters**: named pools of upstream hosts that Envoy can route
to. A cluster defines *what* a backend is and *how* to talk to it (load balancing
policy, timeouts, health checks, TLS, circuit breakers) — but, crucially, it can
defer *which* hosts back it to EDS.

```mermaid
flowchart LR
    accTitle: Where CDS sits in the dependency chain
    accDescr: The Cluster, served by CDS, is highlighted. A Route points to it, and when its type is EDS it defers its endpoints to a ClusterLoadAssignment.
    L[Listener<br/>LDS] --> R[RouteConfiguration<br/>RDS]
    R --> C[Cluster<br/>CDS]
    C -- "type: EDS" --> E[ClusterLoadAssignment<br/>EDS]
    class L lds
    class R rds
    class C cds
    class E eds
    style C stroke:#fff,stroke-width:4px
    classDef lds fill:#1e3a8a,stroke:#60a5fa,color:#fff
    classDef rds fill:#134e4a,stroke:#2dd4bf,color:#fff
    classDef cds fill:#78350f,stroke:#fbbf24,color:#fff
    classDef eds fill:#881337,stroke:#fb7185,color:#fff
```

## How a cluster discovers its endpoints

The cluster's `type` field decides where endpoints come from:

| type | Endpoints come from | Used in |
| --- | --- | --- |
| `STATIC` | inline `load_assignment` (literal IPs) | Lab 00, Lab 03 (loopback) |
| `STRICT_DNS` / `LOGICAL_DNS` | DNS resolution of a hostname | Lab 00 upstream |
| `EDS` | the EDS API | Labs 01, 02, 03 |

The EDS form is the interesting one. The cluster says "do not look for endpoints
in me; ask EDS for a load assignment named X":

```yaml
- "@type": type.googleapis.com/envoy.config.cluster.v3.Cluster
  name: service_backend
  type: EDS                          # <- endpoints come from EDS
  connect_timeout: 1s
  lb_policy: ROUND_ROBIN
  eds_cluster_config:
    service_name: service_backend    # <- the EDS resource name to fetch
    eds_config: { ads: {} }          # <- over the ADS stream
```

## What else lives on a cluster

Everything about *connecting to* a backend that should not change when a single
pod scales:

- **lb_policy**: `ROUND_ROBIN`, `LEAST_REQUEST`, `RING_HASH`, etc.
- **connect_timeout**, **health_checks**, **outlier_detection**.
- **circuit_breakers**: max connections / requests / retries.
- **transport_socket**: upstream TLS (often via SDS).

These are why CDS is separate from EDS: the *policy* for a backend is stable,
while the *membership* of that backend churns. Push policy rarely (CDS), push
membership constantly (EDS).

## Dependency rules

- CDS is the **first** thing sent on an ADS stream. A cluster must exist before
  the routes that target it and before the endpoints that fill it.
- A cluster of `type: EDS` with no endpoints yet is valid — it just has zero
  hosts and returns 503 until EDS provides some.
- `connect_timeout` is **required** and must be positive; `0` is a NACK.

## Inspecting it

```bash
# Cluster names + their discovery type and lb policy
curl -s localhost:9901/config_dump?resource=dynamic_active_clusters | \
  grep -E 'name|type|lb_policy'

# The runtime view: clusters and their current endpoints + health
curl -s localhost:9901/clusters | grep service_backend
```

## Gotchas

- **`type: EDS` but no `eds_cluster_config`** → NACK. If you ask for EDS you must
  say which service name and config source.
- **Bootstrap xDS cluster must speak HTTP/2.** The static cluster that points at
  your gRPC control plane needs `http2_protocol_options` (see the Lab 02
  bootstrap) — gRPC is HTTP/2. Forgetting this is a classic "control plane
  unreachable" bug.
- **Cluster warming**: when a new EDS cluster is added, Envoy "warms" it (fetches
  endpoints, runs health checks) before using it, so there is a brief window
  where it exists but is not yet serving.

## Try it

[Lab 02](../../labs/02-grpc-control-plane/README.md) serves this cluster over gRPC
ADS. Watch the control-plane log: `SEND Cluster version="1"` arrives first, then
`ACK Cluster`. Next: [06 — EDS](../06-eds/README.md).
