**English** | [日本語](README.ja.md)

# 04 — RDS (Route Discovery Service)

RDS discovers **RouteConfigurations**: the rules that match an incoming HTTP
request (by host and path) and decide which cluster it goes to. It sits between
LDS and CDS.

```mermaid
flowchart LR
    accTitle: Where RDS sits in the dependency chain
    accDescr: The RouteConfiguration, served by RDS, is highlighted. A Listener points to it, and it routes to a Cluster by name, which references a ClusterLoadAssignment.
    L[Listener<br/>LDS] --> R[RouteConfiguration<br/>RDS]
    R -- routes to cluster by name --> C[Cluster<br/>CDS]
    C --> E[ClusterLoadAssignment<br/>EDS]
    class L lds
    class R rds
    class C cds
    class E eds
    style R stroke:#fff,stroke-width:4px
    classDef lds fill:#1e3a8a,stroke:#60a5fa,color:#fff
    classDef rds fill:#134e4a,stroke:#2dd4bf,color:#fff
    classDef cds fill:#78350f,stroke:#fbbf24,color:#fff
    classDef eds fill:#881337,stroke:#fb7185,color:#fff
```

## What a RouteConfiguration contains

- a **name** — the same name the listener asked for (`route_config_name`).
- a list of **virtual_hosts**, each matching a set of `domains` (Host headers).
- inside each vhost, a list of **routes**, each with a `match` (path prefix,
  regex, headers) and an `action` (usually `route: { cluster: ... }`).

```yaml
- "@type": type.googleapis.com/envoy.config.route.v3.RouteConfiguration
  name: local_route                  # <- matches the listener's route_config_name
  virtual_hosts:
    - name: backend
      domains: ["*"]
      routes:
        - match: { prefix: "/" }
          route: { cluster: service_backend }   # <- names a CDS cluster
```

## Why RDS is split from LDS

Routing is the part of config that changes most often *for L7 reasons*: shifting
traffic between versions, canary weights, adding a path, changing a timeout.
Splitting RDS from LDS means you can reshape routing **without touching the
listener** — no socket churn, no connection draining. The listener stays up; only
its route table is swapped.

This is the backbone of progressive delivery. A weighted route is just data:

```yaml
routes:
  - match: { prefix: "/" }
    route:
      weighted_clusters:
        clusters:
          - { name: service_v1, weight: 90 }
          - { name: service_v2, weight: 10 }
```

Push that via RDS and 10% of traffic shifts to v2 instantly, with no listener
change.

## Dependency rules

- A route names clusters. Those clusters should exist (CDS) **before** the route
  references them, or requests matching that route get a 503 "no healthy
  upstream" / "cluster not found".
- RDS is delivered after CDS/EDS and after (or with) LDS on the ADS stream.

## Inspecting it

```bash
# Dynamic route configs and the clusters they target
curl -s localhost:9901/config_dump?resource=dynamic_route_configs | \
  grep -E 'name|cluster'
```

You can also see routing decisions live by sending requests with different paths
and Host headers and watching which upstream answers.

## Gotchas

- **`domains` must be unique** across virtual hosts in one route config; an
  overlap is a NACK.
- A route that names a **nonexistent cluster** is accepted by RDS but fails at
  request time (503). RDS does not validate cluster existence at push time.
- Route config name mismatch is a common bug: if the listener asks for
  `local_route` but you serve `local-route`, Envoy never gets routes and every
  request 404s.

## Try it

In [Lab 01](../../labs/01-filesystem-xds/README.md), edit `xds/rds.yaml` to add a
second route (e.g. match `prefix: /healthz` to a different cluster), reload, and
confirm the new routing without the listener ever restarting. Next:
[05 — CDS](../05-cds/README.md).
