[English](README.md) | **日本語**

# 99. 用語集・参考文献

## 用語集

| 用語                      | 意味                                                                                              |
| ------------------------- | ------------------------------------------------------------------------------------------------- |
| **xDS**                   | Envoy が設定を動的に取得するための「x Discovery Service」API ファミリー。                         |
| **データプレーン**        | リクエストのバイトを動かすプロキシ（Envoy）。ホットパス上。                                       |
| **コントロールプレーン**  | 設定を計算しデータプレーンへプッシュするサービス。ホットパス外。                                  |
| **Bootstrap**             | 起動時に Envoy が読む静的設定ファイル。最低限、コントロールプレーンへの到達方法。                 |
| **Listener**              | Envoy がバインドするソケットと、そのトラフィックを処理するフィルタチェーン。**LDS** が配信。      |
| **フィルタチェーン**      | 接続に適用される順序付きネットワークフィルタ。HTTP では HTTP connection manager。                 |
| **HCM**                   | HTTP connection manager: HTTP を解析しルーティングを走らせる L7 フィルタ。                        |
| **RouteConfiguration**    | リクエストを cluster に一致させる virtual host + route 群。**RDS** が配信。                       |
| **Cluster**               | 接続ポリシーを持つ名前付きアップストリーム群。**CDS** が配信。                                    |
| **ClusterLoadAssignment** | cluster を支えるエンドポイントリスト（IP、健全性、ロケーション）。**EDS** が配信。                |
| **Endpoint**              | 具体的なアップストリーム 1 つ `ip:port`。                                                         |
| **ADS**                   | Aggregated Discovery Service: 全リソース型を 1 本の順序付き gRPC ストリームに。                   |
| **SotW**                  | State-of-the-World: 各レスポンスがその型の全リソース集合を運ぶ。                                  |
| **Delta / Incremental**   | 各レスポンスが追加/削除されたリソースだけを運ぶ。                                                 |
| **DiscoveryRequest**      | Envoy → コントロールプレーンのメッセージ。リソースを要求し、直前のレスポンスを ACK/NACK する。    |
| **DiscoveryResponse**     | コントロールプレーン → Envoy のメッセージ。あるバージョンのリソースを運び、nonce が刻印される。   |
| **version_info**          | Envoy が正常に適用した設定バージョン。ACK でエコーされる。                                        |
| **nonce**                 | request（ACK/NACK）が、それが答えるレスポンスと対応することを示す識別子。                         |
| **ACK**                   | Envoy がプッシュされた設定バージョンを受理・適用した。                                            |
| **NACK**                  | Envoy がプッシュを拒否した。直前のバージョンを保ち `error_detail` を報告する。                    |
| **node id**               | Envoy が自分を名乗る方法。コントロールプレーンはこれでプロキシごとの設定をキーする。              |
| **サイドカー**            | 同居する単一アプリの inbound/outbound を代理する Envoy。                                          |
| **ウォーミング**          | 新しい cluster/listener を使い始める前に Envoy が準備する（エンドポイント取得、ヘルスチェック）。 |
| **SDS**                   | Secret Discovery Service: TLS 証明書/鍵を配信。メッシュの相互 TLS を支える。                      |
| **go-control-plane**      | xDS コントロールプレーンを構築する Go の参照ライブラリ（Lab 02〜03 で使用）。                     |

## 4 つの API の関係（1 行のおさらい）

```mermaid
flowchart LR
    accTitle: 4 つの xDS API と相互参照の仕方
    accDescr: LDS は route_config_name で RDS を、RDS は cluster で CDS を、CDS は service_name で EDS を参照する。
    LDS -- route_config_name --> RDS
    RDS -- cluster --> CDS
    CDS -- service_name --> EDS
    class LDS lds
    class RDS rds
    class CDS cds
    class EDS eds
    classDef lds fill:#1e3a8a,stroke:#60a5fa,color:#fff
    classDef rds fill:#134e4a,stroke:#2dd4bf,color:#fff
    classDef cds fill:#78350f,stroke:#fbbf24,color:#fff
    classDef eds fill:#881337,stroke:#fb7185,color:#fff
```

読み方: listener は route config を名前で、route は cluster を名前で、cluster はエンドポイント集合を名前で指す。ADS はこの依存順（LDS/RDS より先に CDS/EDS）で送るので、参照が宙に浮かない。

## スコープ: このリポジトリは L7（HTTP）

ここでは全部 HTTP をルーティングするので、RDS（L7 の概念）が常に関わる。Envoy は生の
**L4（TCP/UDP）**も代理できる。その場合 listener は HTTP connection manager の代わりに
`tcp_proxy` ネットワークフィルタを使い、cluster を直接指す。このモードでは **RDS は無い**
（一致させる path や host が無い）が、**LDS・CDS・EDS はそのまま効く**し、ADS も同じように
運ぶ。だからメンタルモデルは転用できる。抜けるのは L7 ルーティング層だけだ。

## 参考文献

- Envoy。xDS プロトコル: <https://www.envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol>
- Envoy。Listener / LDS: <https://www.envoyproxy.io/docs/envoy/latest/configuration/listeners/lds>
- Envoy。Route / RDS: <https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_conn_man/rds>
- Envoy。Cluster / CDS: <https://www.envoyproxy.io/docs/envoy/latest/configuration/upstream/cluster_manager/cds>
- Envoy。Endpoint / EDS: <https://www.envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol#endpoint-discovery-service-eds>
- Envoy。動的設定サンドボックス: <https://www.envoyproxy.io/docs/envoy/latest/start/sandboxes/dynamic-configuration-filesystem>
- Envoy。管理インターフェース: <https://www.envoyproxy.io/docs/envoy/latest/operations/admin>
- go-control-plane: <https://github.com/envoyproxy/go-control-plane>
- kind: <https://kind.sigs.k8s.io/>
- Istio アーキテクチャ（本物の xDS コントロールプレーン）: <https://istio.io/latest/docs/ops/deployment/architecture/>

[トップ README](../../README.ja.md) へ戻る。
