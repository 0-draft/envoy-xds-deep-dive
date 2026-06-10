[English](README.md) | **日本語**

# 06 — EDS (Endpoint Discovery Service)

EDS は **ClusterLoadAssignment** を配信する。cluster を支える具体的なエンドポイント IP と
ポートのリスト、そして健全性とロケーションだ。依存チェーンの最下段であり、最も頻繁に変わる
データ。

```mermaid
flowchart LR
    L[Listener LDS] --> R[RouteConfiguration RDS]
    R --> C[Cluster CDS]
    C --> E[ClusterLoadAssignment EDS]
    style E stroke-width:3px
```

## ClusterLoadAssignment が持つもの

- **cluster_name**: cluster の `eds_cluster_config.service_name` と一致しなければならない。
- **endpoints**: **ロケーション**（region/zone）でグループ化され、各グループが
  **lb_endpoints** のリストを持ち、各々が `socket_address`（IP とポート）を持つ。
- エンドポイントごとの **health_status** と **load_balancing_weight**。

```yaml
- "@type": type.googleapis.com/envoy.config.endpoint.v3.ClusterLoadAssignment
  cluster_name: service_backend       # <- CDS の cluster と一致
  endpoints:
    - lb_endpoints:
        - endpoint:
            address:
              socket_address: { address: 10.77.0.11, port_value: 5678 }
        - endpoint:
            address:
              socket_address: { address: 10.77.0.12, port_value: 5678 }
```

## EDS のエンドポイントは名前ではなく IP

ここでつまずく人が多い。`STRICT_DNS` cluster は自分でホスト名を解決する。`EDS` cluster は
DNS を解決**しない** — コントロールプレーンが解決済みの IP を Envoy に渡す前提だ。これはまさに
Kubernetes でコントロールプレーンがやることだ。API を監視してポッド IP を取り、EDS として
プッシュする。

このリポジトリも同じ構図を写している。

- Lab 01 と 02 は upstream コンテナを固定 IP に留め、その IP を EDS に並べる。
- Lab 03 のコントロールプレーンは headless Service を解決して**ポッド IP**を取り、EDS として
  プッシュする — 現実的なパターンだ。

## なぜ EDS は CDS から分かれているのか（その見返り）

エンドポイントこそが入れ替わりの主役。ポッドが 2 から 3 にスケールし、ノードが死に、
ヘルスチェックが反転する。そのどれもが、エンドポイントリスト*だけ*を更新すべきだ — cluster の
TLS 設定を再評価せず、ルートテーブルをやり直さず、listener に触れない。

この隔離は直接観察できる。Lab 03 で `app-b` を 2 から 3 ポッドにスケールすると、起きるプッシュは
ちょうど 1 種類だ。

```text
app-b endpoints changed -> [10.244.1.3 10.244.1.4 10.244.1.7]
PUSH node=app-a-sidecar version=4 (cds=1 eds=1 rds=1 lds=1 resources)
ACK  ClusterLoadAssignment version="4"
```

呼び出し側の Envoy は数秒で新しい集合をロードバランス対象にする。再起動なしだ。

## 依存ルール

- EDS は ADS ストリームで CDS の**後**に送られる。cluster は、その load assignment より先に
  存在しなければならない。
- Envoy が持たない `cluster_name` を指す EDS 更新は無視される。
- 全エンドポイントの削除は合法。そのとき cluster は健全なホストを持たず 503 を返す。
  （これがバックエンドを穏当にドレインするやり方だ。）

## 確認する

```bash
# cluster のエンドポイント + 健全性（実行時ビュー）
curl -s localhost:9901/clusters | grep -E 'service_backend.*(::|health)'

# Envoy が保持する厳密な ClusterLoadAssignment とそのバージョン
curl -s localhost:9901/config_dump?resource=dynamic_endpoint_configs
```

## 落とし穴

- **`cluster_name` の不一致**: cluster の `service_name` と等しくないと、エンドポイントは
  黙って一切アタッチされない。この 2 つの文字列は常に突き合わせること。
- **健全性とメンバシップ**: EDS はエンドポイントを削除する代わりに `UNHEALTHY` と印を付けられる。
  見えるがローテーションからは外れる。（cluster 側の）アクティブヘルスチェックと EDS の
  health status は相互作用する。
- **全エンドポイント消失 = エラーではなく 503**: 空の load assignment は妥当な状態なので、
  NACK は出ない — 出るのはトラフィックの失敗だ。ACK だけでなく `/clusters` の健全性を見ること。

## やってみる

[Lab 01](../../labs/01-filesystem-xds/README.ja.md) では `xds/eds.yaml` からエンドポイントを
1 つ消し、cluster が縮むのを見る。[Lab 02](../../labs/02-grpc-control-plane/README.ja.md) では
`POST /scale?n=1` で EDS がライブにプッシュされるのを見る。
[Lab 03](../../labs/03-pod-to-pod-kind/README.ja.md) では `kubectl scale` で実ポッドを増減させ、
EDS が追従するのを見る。次は [07 — Pod-to-pod](../07-pod-to-pod/README.ja.md)。
