[English](README.md) | **日本語**

# ラボ

完全に静的な Envoy から、Kubernetes 上の 2 ポッドメッシュまで登る 4 つのハンズオン。 Envoy データプレーン側はほとんど変わらない。変わるのは**設定の配信方法**だ。それこそが xDS の学びのすべて。

| ラボ                                                               | コントロールプレーン | トランスポート   | 実行基盤       | 対応する章                                            |
| ------------------------------------------------------------------ | -------------------- | ---------------- | -------------- | ----------------------------------------------------- |
| [00 静的ブートストラップ](00-static-bootstrap/README.ja.md)        | なし                 | 静的ファイル     | Docker Compose | [docs 01](../docs/01-envoy-config-model/README.ja.md) |
| [01 filesystem xDS](01-filesystem-xds/README.ja.md)                | エディタ             | ファイルシステム | Docker Compose | [docs 02-04](../docs/02-xds-overview/README.ja.md)    |
| [02 gRPC コントロールプレーン](02-grpc-control-plane/README.ja.md) | `go-control-plane`   | gRPC ADS         | Docker Compose | [docs 05-06](../docs/05-cds/README.ja.md)             |
| [03 kind での pod-to-pod](03-pod-to-pod-kind/README.ja.md)         | メッシュ制御プレーン | gRPC ADS         | `kind`         | [docs 07](../docs/07-pod-to-pod/README.ja.md)         |

## 前提

- **全ラボ**: `docker` + `docker compose`。
- **Lab 03 のみ追加**: `kind` と `kubectl`。
- コントロールプレーンのイメージ再ビルドには `go` 1.25+ が要るが、`docker compose up --build` の中で Docker が代わりにやってくれる。

## 約束ごと

- Envoy の管理インターフェースは全ラボで `:9901`。状態確認はまずここから: `/config_dump`, `/clusters`, `/listeners`, `/stats`。
- [`../scripts`](../scripts) のヘルパースクリプトが、よく使う admin クエリをラップしている。
- 各ラボ README の末尾に **片付け（Teardown）** がある。次に進む前に実行して、ポートと Docker ネットワークを解放すること。

まずは [Lab 00](00-static-bootstrap/README.ja.md) から。
