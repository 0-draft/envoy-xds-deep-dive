**English** | [日本語](README.ja.md)

# Labs

Four hands-on labs that climb from a fully static Envoy to a two-pod mesh on Kubernetes. The Envoy data plane barely changes between them: what changes is **how its config is delivered**. That is the whole lesson of xDS.

| Lab                                                      | Control plane      | Transport   | Runs on        | Pairs with                                         |
| -------------------------------------------------------- | ------------------ | ----------- | -------------- | -------------------------------------------------- |
| [00 static bootstrap](00-static-bootstrap/README.md)     | none               | static file | Docker Compose | [docs 01](../docs/01-envoy-config-model/README.md) |
| [01 filesystem xDS](01-filesystem-xds/README.md)         | your editor        | filesystem  | Docker Compose | [docs 02-04](../docs/02-xds-overview/README.md)    |
| [02 gRPC control plane](02-grpc-control-plane/README.md) | `go-control-plane` | gRPC ADS    | Docker Compose | [docs 05-06](../docs/05-cds/README.md)             |
| [03 pod-to-pod on kind](03-pod-to-pod-kind/README.md)    | mesh control plane | gRPC ADS    | `kind`         | [docs 07](../docs/07-pod-to-pod/README.md)         |

## Prerequisites

- **All labs**: `docker` + `docker compose`.
- **Lab 03 also**: `kind` and `kubectl`.
- Rebuilding the control-plane images needs `go` 1.25+, but Docker does that for you during `docker compose up --build`.

## Conventions

- Envoy's admin interface is on `:9901` in every lab. Start there to inspect state: `/config_dump`, `/clusters`, `/listeners`, `/stats`.
- The helper scripts in [`../scripts`](../scripts) wrap the most useful admin queries.
- Each lab README ends with a **Teardown** section: run it before moving on so ports and Docker networks are freed.

Start with [Lab 00](00-static-bootstrap/README.md).
