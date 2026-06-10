[English](README.md) | **日本語**

# 04. RDS (Route Discovery Service)

RDS は **RouteConfiguration** を配信する。届いた HTTP リクエストを（host と path で）一致させ、どの cluster へ送るかを決める規則だ。LDS と CDS の間に座る。

```mermaid
flowchart LR
    accTitle: 依存チェーンにおける RDS の位置
    accDescr: RDS が配信する RouteConfiguration が強調されている。Listener がそれを指し、RouteConfiguration は cluster を名前で指し、Cluster は ClusterLoadAssignment を参照する。
    L[Listener<br/>LDS] --> R[RouteConfiguration<br/>RDS]
    R -- cluster を名前で指す --> C[Cluster<br/>CDS]
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

## RouteConfiguration が持つもの

- **name**。listener が要求したのと同じ名前（`route_config_name`）。
- **virtual_hosts** のリスト。各々が `domains`（Host ヘッダ）の集合に一致する。
- 各 vhost の中に **routes** のリスト。各 route は `match`（path prefix, 正規表現, ヘッダ）と `action`（通常 `route: { cluster: ... }`）を持つ。

```yaml
- "@type": type.googleapis.com/envoy.config.route.v3.RouteConfiguration
  name: local_route                  # <- listener の route_config_name と一致
  virtual_hosts:
    - name: backend
      domains: ["*"]
      routes:
        - match: { prefix: "/" }
          route: { cluster: service_backend }   # <- CDS の cluster を名前で指す
```

## なぜ RDS は LDS から分かれているのか

ルーティングは、*L7 的な理由で*最も頻繁に変わる設定だ。バージョン間のトラフィック移行、カナリアの重み、パス追加、タイムアウト変更。RDS を LDS から分けると、**listener に触れずに** ルーティングを作り変えられる。ソケットの入れ替えも接続ドレインもない。listener は上がったまま、ルートテーブルだけが差し替わる。

これがプログレッシブデリバリの背骨だ。重み付きルートは単なるデータにすぎない。

```yaml
routes:
  - match: { prefix: "/" }
    route:
      weighted_clusters:
        clusters:
          - { name: service_v1, weight: 90 }
          - { name: service_v2, weight: 10 }
```

これを RDS でプッシュすれば、listener を変えずに 10% のトラフィックが即座に v2 へ移る。

## 依存ルール

- route は cluster を名前で指す。その cluster は、route が参照する**前**に存在すべき（CDS）。さもないとそのルートに一致するリクエストは 503「no healthy upstream / cluster not found」になる。
- RDS は ADS ストリーム上で CDS/EDS の後、かつ LDS の後（または同時）に配信される。

## 確認する

```bash
# 動的 route config と、それが指す cluster
curl -s localhost:9901/config_dump?resource=dynamic_route_configs | \
  grep -E 'name|cluster'
```

異なる path や Host ヘッダでリクエストを送り、どのアップストリームが応答するかを見れば、ルーティング判断をライブで観察できる。

## 落とし穴

- 1 つの route config 内で **`domains` は一意**でなければならない。重複は NACK。
- **存在しない cluster** を指す route は RDS には受理されるが、リクエスト時に失敗する（503）。 RDS はプッシュ時に cluster の存在を検証しない。
- route config 名の不一致はよくあるバグ。listener が `local_route` を要求しているのに `local-route` を配ると、Envoy はルートを得られず、全リクエストが 404 になる。

## やってみる

[Lab 01](../../labs/01-filesystem-xds/README.ja.md) で `xds/rds.yaml` を編集して 2 つ目の route を足し（例: `prefix: /healthz` を別 cluster へ）、リロードし、listener が一度も再起動せずに新しいルーティングが効くのを確かめる。次は [05 CDS](../05-cds/README.ja.md)。
