[English](README.md) | **日本語**

# 02. xDS 概観

**xDS** = 「**x** Discovery Service」。**x** は Listener / Route / Cluster / Endpoint / Secret などの総称。すべて同じプロトコルを共有し、運ぶリソース型だけが違う。この章はそのプロトコルの話。章 03〜06 で各 API を個別に掘る。

## ファミリー

| API | リソース型            | 配信するもの                          |
| --- | --------------------- | ------------------------------------- |
| LDS | Listener              | Envoy がどこで待ち受けるか            |
| RDS | RouteConfiguration    | リクエストをどう一致・転送するか      |
| CDS | Cluster               | どんなアップストリーム群があるか      |
| EDS | ClusterLoadAssignment | cluster を支えるエンドポイント IP     |
| SDS | Secret                | TLS 証明書と鍵                        |
| ADS | (上記すべて)          | 全型を 1 本の順序付きストリームで運ぶ |

このリポジトリは LDS / RDS / CDS / EDS に集中し、**ADS** でそれらを運ぶ。

## なぜ 4 つの API があるのか: 振り分けを目で見る

なぜ API が 4 つあるのかを一番つかみやすいのは、1 つのルーティング判断が 4 つすべてを通って振り分けられる様子を見ることだ。各層は**1 対多のファンアウト**で、互いに独立して広がる。

```mermaid
flowchart LR
    accTitle: LDS, RDS, CDS, EDS が 1 つのルーティング判断をどう分担するか
    accDescr: 2 つの listener がトラフィックを受ける。route が path で cluster に振り分ける。各 cluster は多数の endpoint ポッドにファンアウトする。listener はまれに、route はときどき、cluster はときどき、endpoint は絶えず変わる。
    cA((公開<br/>クライアント)) --> L1
    cB((内部<br/>クライアント)) --> L2
    subgraph LDSg["LDS: どこで受けるか (listener)"]
        L1["0.0.0.0:8080"]
        L2["0.0.0.0:8443 mTLS"]
    end
    subgraph RDSg["RDS: どう一致させるか (route)"]
        R1["/shop"]
        R2["/api"]
        R3["* default"]
    end
    subgraph CDSg["CDS: どのサービスか (cluster)"]
        C1["shop"]
        C2["api"]
        C3["web"]
    end
    subgraph EDSg["EDS: どのポッドか (endpoint)"]
        E1[("shop pods x3")]
        E2[("api pods x5")]
        E3[("web pods x2")]
    end
    L1 --> R1 & R2 & R3
    L2 --> R2
    R1 --> C1
    R2 --> C2
    R3 --> C3
    C1 --> E1
    C2 --> E2
    C3 --> E3
    class L1,L2 lds
    class R1,R2,R3 rds
    class C1,C2,C3 cds
    class E1,E2,E3 eds
    class cA,cB ext
    classDef lds fill:#1e3a8a,stroke:#60a5fa,color:#fff
    classDef rds fill:#134e4a,stroke:#2dd4bf,color:#fff
    classDef cds fill:#78350f,stroke:#fbbf24,color:#fff
    classDef eds fill:#881337,stroke:#fb7185,color:#fff
    classDef ext fill:#374151,stroke:#9ca3af,color:#fff
```

左から右に読むと、役割分担がはっきり見える。

- **LDS** は*どこで Envoy がトラフィックを受けるか*を決める。listener は数個（平文・mTLS）。めったに変わらない。
- **RDS** は*各リクエストをどう一致させるか*を決める。listener ごとに多数の route が path や host で振り分ける。トラフィックを作り変えるときに変わる。
- **CDS** は*どのバックエンドサービス*を route が指すかを決める。サービスごとに 1 cluster。サービスの追加・削除で変わる。
- **EDS** は*どの具体的なポッド*が cluster を支えるかを決める。cluster ごとに多数の endpoint。ポッドのスケールや再起動で絶えず変わる。

4 つの API が存在するのは、この 4 つの問いに 4 つの異なる所有者と 4 つの異なる変化頻度があるからだ。分割することで、忙しいデータ（endpoint）が安定したデータ（listener）を乱さずに動ける。

## 同じリソースを配る 3 つの方法

リソースは、どう届こうと同一の protobuf メッセージ。ラボはこのはしごを意図的に登る。

1. **静的**（Lab 00）: bootstrap ファイルに焼き込む。ディスカバリは一切ない。
2. **ファイルシステム**（Lab 01）: Envoy がファイルパスを監視し、あなたがファイルを編集する。コントロールプレーンのコードなしで「何が」配られるかを理解するのに最適。
3. **gRPC**（Lab 02・03）: 本物のコントロールプレーンがリソースをストリームし、Envoy が ACK/NACK する。本番システム（Istio など）が使うやり方。

## トランスポート: request, response, ACK

gRPC 上で、Envoy とコントロールプレーンは長命ストリーム上で 2 種類のメッセージを交換する。

- **DiscoveryRequest**。Envoy が送る。「型 T のリソースが欲しい」（型によっては特定の名前も）。直前のレスポンスに対する ACK/NACK も兼ねる。
- **DiscoveryResponse**。コントロールプレーンが送る。「バージョン V 時点の型 T のリソースはこれだ」。**nonce** が刻印される。

ループはこう動く。

```mermaid
sequenceDiagram
    accTitle: xDS の request, response, ACK, NACK ループ
    accDescr: Envoy がリソースを要求し、コントロールプレーンがバージョンと nonce 付きで応答し、Envoy は成功時に ACK、失敗時は前バージョンを保ったまま error 付きで NACK する。
    participant E as Envoy
    participant C as コントロールプレーン
    E->>C: DiscoveryRequest (type=Cluster, version="", nonce="")
    C->>E: DiscoveryResponse (version=1, nonce=a1, resources=[...])
    Note over E: 検証して適用
    E->>C: DiscoveryRequest (type=Cluster, version=1, nonce=a1)  ← ACK
    Note over E,C: のちに設定が変わる
    C->>E: DiscoveryResponse (version=2, nonce=a2, resources=[...])
    Note over E: 検証失敗!
    E->>C: DiscoveryRequest (version=1, nonce=a2, error_detail=...)  ← NACK
    Note over E: version 1 のまま動き続ける
```

これを成立させるのは 2 つのフィールド。

- **version_info**: Envoy が*正常に適用した*設定のバージョン。ACK では新バージョンをエコーする。 NACK では**古い**バージョンをエコーし（動いていない）、`error_detail` を埋める。
- **nonce**: この request が*どのレスポンス*に答えているかを識別する。複数が飛行中でもコントロールプレーンが ACK とプッシュを対応付けられる。

### ACK と NACK を具体的に

[Lab 02](../../labs/02-grpc-control-plane/README.ja.md) ではコントロールプレーンが両方をログに出す。健全なプッシュ:

```text
stream 1  SEND Listener version="1" (1 resources)
stream 1   ACK Listener version="1"
```

Envoy が拒否するプッシュ（わざと listener のポートを 70000 (65535 超) にした）:

```text
stream 1  NACK Listener version="2": ... PortValue: value must be <= 65535
```

決定的に重要な性質: NACK は**安全**だ。Envoy は不正なリソースを拒否し、直前の正常なものを配り続ける。コントロールプレーンからの不正設定は、障害ではなく「変更なし」に縮退する。

## State-of-the-World と Delta

トランスポートには 2 つの方式がある。

```mermaid
stateDiagram-v2
    accTitle: State-of-the-World と Delta の比較
    accDescr: 2 つのトランスポート方式。State-of-the-World は各レスポンスでその型の全リソースを送る。Delta は追加または削除された差分だけを送る。
    [*] --> SotW
    [*] --> Delta
    SotW: State-of-the-World
    SotW: 各レスポンス = その型の全リソース集合
    Delta: Incremental (Delta)
    Delta: 各レスポンス = 追加/削除された差分のみ
```

- **State-of-the-World (SotW)** が元祖で既定。各 DiscoveryResponse はその型の*完全な* リストを含む。リストから cluster が消えていれば、それは削除を意味する。
- **Delta / Incremental** は変更分だけ送る。エンドポイントが数千あって 1 つだけ変わった、という場面でスケールする。

このリポジトリは SotW を使う（`go-control-plane` のスナップショットキャッシュの既定）。学習中はそのほうが考えやすいからだ。メンタルモデル (バージョン、nonce、ACK/NACK) は両者で同じ。

## ADS と、なぜ順序が重要か

4 つのリソース型は独立していない。依存チェーンを成す。

```mermaid
flowchart LR
    accTitle: xDS リソースの依存チェーン
    accDescr: Listener は RouteConfiguration を参照し、それは Cluster を、Cluster は ClusterLoadAssignment を参照する。
    L[Listener<br/>LDS] -- 参照 --> R[RouteConfiguration<br/>RDS]
    R -- 参照 --> C[Cluster<br/>CDS]
    C -- 参照 --> E[ClusterLoadAssignment<br/>EDS]
    class L lds
    class R rds
    class C cds
    class E eds
    classDef lds fill:#1e3a8a,stroke:#60a5fa,color:#fff
    classDef rds fill:#134e4a,stroke:#2dd4bf,color:#fff
    classDef cds fill:#78350f,stroke:#fbbf24,color:#fff
    classDef eds fill:#881337,stroke:#fb7185,color:#fff
```

Listener は route config を名前で指し、route は cluster を指し、cluster は EDS サービスを指す。まだ学習していない cluster を指す Listener を Envoy が受け取れば、参照が宙に浮く。

Envoy の規則は **「make before break（壊す前に作る）」**。

- チェーンを*下る*（追加する）ときは、依存先を先に学ぶ: **CDS は EDS より先**、 **LDS は RDS より先**。cluster は、それを満たすエンドポイントより先に存在すべき。
- チェーンを*上る*（削除する）ときは、参照する側を先に消す。

各型が別々の gRPC ストリームにあると、コントロールプレーンはこの順序をストリーム間で保証できない。 **ADS（Aggregated Discovery Service）**は全型を**1 本**のストリームに多重化して解決する。そこではコントロールプレーンがレスポンスの順序を正確に制御できる。この順序は Lab 02 のログで直接見える。

```text
SEND Cluster       version="1"      ← まず CDS
SEND ClusterLoadAssignment ...      ← 次に EDS
ACK  Cluster
SEND Listener      version="1"      ← LDS
ACK  ClusterLoadAssignment
SEND RouteConfiguration ...         ← 次に RDS
ACK  Listener
ACK  RouteConfiguration
```

これが、02 以降のラボがすべて ADS を使う理由だ。依存関係的に正しい決定論的な更新を得る、唯一の方法だから。

## ノード ID の役割

各 Envoy は bootstrap で（そして全 DiscoveryRequest で）**ノード ID** で自分を名乗る。コントロールプレーンはそれを使って、*この* Envoy にどの設定を渡すかを決める。Lab 02 ではノードは 1 つ。Lab 03 ではコントロールプレーンが、ノード ID だけを根拠に `app-a-sidecar` と `app-b-sidecar` へ**別々の**リソースを配る。

## やってみる

[Lab 01 filesystem xDS](../../labs/01-filesystem-xds/README.ja.md) を実行して、4 つのリソース型が動的に配信される様子を見る（まだ gRPC ではないので*リソース*に集中できる）。そのあと章 [03 LDS](../03-lds/README.ja.md) 〜 [06 EDS](../06-eds/README.ja.md) で 1 つずつ扱う。
