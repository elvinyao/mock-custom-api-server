# mock-api-server

Go 製の汎用ファイル駆動 API モックサーバー。コード変更不要 — エンドポイント・ルール・レスポンスをすべて YAML ファイルで定義できます。

## 機能一覧

- **ルールベースルーティング** — リクエストボディフィールド・ヘッダー・クエリパラメータ・パスセグメントを exact / prefix / suffix / contains / regex / range でマッチング
- **AND / OR 条件ロジック** — ルール内の条件を AND（デフォルト）または OR で評価。`condition_groups` で複合条件も記述可能
- **レスポンステンプレート** — シンプルな `{{.var}}` 置換、または Go `text/template` エンジンによるリッチな動的レスポンス生成
- **ランダムレスポンス** — 重み付きランダムで複数レスポンスファイルから選択。カオステスト・負荷テストに活用可能
- **インラインレスポンス** — `inline_body` フィールドにより、JSON ファイル不要で YAML 内にレスポンスボディを直接記述可能
- **ホットリロード** — サーバー再起動なしで設定ファイルの変更を即時反映
- **CORS ミドルウェア** — 許可オリジン・メソッド・ヘッダー・プリフライト応答を細かく設定
- **リクエスト記録** — 全リクエスト／レスポンスをサーキュラーバッファに保存。Admin API または Web UI から閲覧可能
- **Admin REST API** — エンドポイント管理・リクエスト履歴参照・メトリクス取得・設定リロードをランタイムで操作
- **Web UI ダッシュボード** — Alpine.js 製 SPA をバイナリに内包。ビルドステップ不要
- **Swagger UI** — 登録済みエンドポイントから OpenAPI 3.0 仕様を自動生成し、ブラウザ上でインタラクティブに試せる
- **プロキシモード** — リクエストを実際のアップストリームに転送。インタラクションを YAML スタブとして記録し、後でオフライン再生も可能
- **エンドポイントメトリクス** — リクエスト数・エラー率・レイテンシ（平均 / 最小 / 最大）をインメモリで集計
- **レスポンス遅延** — `delay_ms` で任意のレスポンス遅延を設定
- **Content-Type 制御** — レスポンスごとに `content_type` を指定。デフォルトは `application/json`
- **TLS サポート** — 証明書ファイルを設定するだけで HTTPS 対応
- **IP 許可リスト** — Admin API へのアクセスを特定の IP / CIDR に制限

---

## クイックスタート

```bash
git clone <repository-url>
cd mock-api-server
go mod tidy
go run main.go                          # デフォルト: ./conf/config.yaml
go run main.go -config path/to/cfg.yaml # 設定ファイルのパスを明示指定
```

モックエンドポイントの動作確認:

```bash
curl -X POST http://localhost:8080/api/v1/payment/status \
  -H "Content-Type: application/json" \
  -H "X-User-Type: premium" \
  -d '{"order_id": "VIP_1001"}'
```

Web UI を開く: `http://localhost:8080/mock-admin/ui/`

Swagger UI を開く: `http://localhost:8080/mock-admin/ui/swagger.html`

---

## Docker Compose

```bash
cp docker-compose.example.yml docker-compose.yml
docker compose up
```

```yaml
# docker-compose.example.yml（抜粋）
services:
  mock-api-server:
    image: ghcr.io/elvinyao/mock-api-server:latest
    ports:
      - "8080:8080"
    command: ["/app/mock-api-server", "-config", "/app/conf/config.yaml"]
    volumes:
      - ./conf:/app/conf:ro      # 設定ファイル・エンドポイント定義・モックファイルをまとめてマウント
      - ./recorded:/app/recorded
      - ./logs:/app/logs
```

---

## Kubernetes（Kustomize）

`k8s/kustomization.yaml` が `conf/` 配下の実ファイルを参照して ConfigMap を自動生成します。

```bash
# 生成内容を確認（適用なし）
kubectl kustomize k8s/

# クラスターに適用
kubectl apply -k k8s/
```

---

## ディレクトリ構成

```
mock-api-server/
├── main.go                         エントリポイント；全サブシステムを接続
├── conf/                           ユーザー設定領域（Docker / K8s でここだけマウント）
│   ├── config.yaml                 メイン設定ファイル
│   ├── endpoints/                  エンドポイント YAML 定義
│   │   ├── holiday.yaml
│   │   ├── business_apis.yaml
│   │   └── ...
│   └── mocks/                      JSON レスポンスファイル
│       ├── payment/
│       ├── user/
│       └── ...
├── k8s/                            Kubernetes マニフェスト
│   ├── kustomization.yaml          ConfigMap を conf/ から生成
│   └── deployment.yaml
├── config/                         Go パッケージ（YAML なし）
│   ├── config.go                   全データ構造定義
│   ├── loader.go                   YAML ロード・デフォルト値・バリデーション
│   └── watcher.go                  ホットリロード用ファイルウォッチャー
├── handler/
│   ├── handler.go                  リクエストディスパッチ
│   ├── selector.go                 値抽出（body / header / query / path）
│   ├── matcher.go                  ルールマッチング（AND / OR・条件グループ）
│   └── response.go                 レスポンス構築・ランダム選択・遅延
├── middleware/
│   ├── logger.go                   zap アクセスログ
│   ├── recovery.go                 パニックリカバリー
│   ├── cors.go                     CORS ヘッダー注入・プリフライト
│   ├── recorder.go                 リクエスト／レスポンスボディ記録
│   └── metrics.go                  エンドポイント別 duration / status カウンター
├── admin/
│   ├── handler.go                  Admin API ルーター・認証
│   ├── config.go                   GET /config、POST /config/reload
│   ├── endpoints.go                CRUD /endpoints
│   ├── requests.go                 GET / DELETE /requests
│   ├── metrics.go                  GET /metrics
│   └── openapi.go                  OpenAPI 3.0 仕様生成（Swagger UI 用）
├── proxy/
│   ├── handler.go                  リバースプロキシ（フォールバック対応）
│   └── stub_writer.go              記録したインタラクションを YAML スタブに保存
├── recorder/
│   └── recorder.go                 サーキュラーバッファレコーダー
├── metrics/
│   └── metrics.go                  インメモリメトリクスストア
├── ui/
│   ├── ui.go                       embed.FS 登録
│   └── static/
│       ├── index.html              Web UI SPA（Alpine.js）
│       └── swagger.html            Swagger UI（vendored）
└── pkg/
    └── template/
        └── template.go             シンプル・Go text/template エンジン
```

---

## 設定リファレンス

### `conf/config.yaml`

```yaml
port: 8080
hot_reload: true
reload_interval_sec: 5

logging:
  level: "debug"          # debug | info | warn | error
  access_log: true
  log_format: "text"      # text | json
  log_file: ""            # 空 = 標準出力

error_handling:
  show_details: true
  custom_error_responses:
    404: "./conf/mocks/errors/not_found.json"
    500: "./conf/mocks/errors/internal_error.json"

cors:
  enabled: true
  allowed_origins: ["*"]
  allowed_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"]
  allowed_headers: ["Content-Type", "Authorization", "X-Request-ID"]
  allow_credentials: false
  max_age_seconds: 86400

admin_api:
  enabled: true
  prefix: "/mock-admin"
  auth:
    enabled: false
    username: "admin"
    password: "secret"
    allowed_ips:          # 指定した IP / CIDR 以外のアクセスを拒否（空 = 無制限）
      - "127.0.0.1"
      - "10.0.0.0/8"

recording:
  enabled: true
  max_entries: 1000       # サーキュラーバッファサイズ
  record_body: true
  max_body_bytes: 65536   # リクエスト／レスポンスボディの上限バイト数
  exclude_paths:
    - "/health"
    - "/mock-admin/"

health_check:
  enabled: true
  path: "/health"

tls:
  enabled: false
  cert_file: ""
  key_file: ""

endpoints:
  - "./endpoints/holiday.yaml"
  - "./endpoints/business_apis.yaml"
```

> **パス解決のルール**
> - `endpoints:` のパスは **config.yaml のあるディレクトリ**（`filepath.Dir`）基準で解決されます。
> - `response_file` と `custom_error_responses` のパスは**プロセスの作業ディレクトリ**（通常はリポジトリルート / `/app`）基準です。

---

### エンドポイント YAML — 単一エンドポイント

```yaml
path: "/api/v1/payment/status"
method: "POST"
description: "決済ステータスモック"

selectors:
  - name: "order_id"
    type: "body"      # body | header | query | path
    key: "order_id"   # body の場合は gjson パス
  - name: "user_type"
    type: "header"
    key: "X-User-Type"

rules:
  - conditions:
      - selector: "order_id"
        match_type: "prefix"   # exact | prefix | suffix | contains | regex | range
        value: "ERR_"
    response_file: "./conf/mocks/payment/error_order.json"
    status_code: 400
    delay_ms: 100

  - conditions:
      - selector: "order_id"
        match_type: "prefix"
        value: "VIP_"
      - selector: "user_type"
        match_type: "exact"
        value: "premium"
    response_file: "./conf/mocks/payment/vip_success.json"
    status_code: 200
    headers:
      X-VIP-Status: "active"

default:
  response_file: "./conf/mocks/payment/default.json"
  status_code: 200
```

---

### エンドポイント YAML — 複数エンドポイント

```yaml
paths:
  - path: "/api/v1/user/:user_id"
    method: "GET"
    selectors:
      - name: "user_id"
        type: "path"
        key: "user_id"
    rules:
      - conditions:
          - selector: "user_id"
            match_type: "exact"
            value: "admin"
        response_file: "./conf/mocks/user/admin.json"
        status_code: 200
    default:
      response_file: "./conf/mocks/user/guest.json"
      status_code: 200

  - path: "/api/v1/orders"
    method: "GET"
    ...
```

---

### OR 条件ロジック

```yaml
rules:
  - condition_logic: "or"   # いずれか 1 つの条件を満たせばマッチ
    conditions:
      - selector: "status"
        match_type: "exact"
        value: "pending"
      - selector: "status"
        match_type: "exact"
        value: "processing"
    response_file: "./conf/mocks/order/in_progress.json"
    status_code: 200
```

### 条件グループ

```yaml
rules:
  - conditions:
      - selector: "user_type"
        match_type: "exact"
        value: "vip"
    condition_groups:
      - logic: "or"
        conditions:
          - selector: "region"
            match_type: "exact"
            value: "us"
          - selector: "region"
            match_type: "exact"
            value: "eu"
    response_file: "./conf/mocks/vip_regional.json"
    status_code: 200
```

---

### インラインレスポンス

ファイル不要でレスポンスボディを YAML に直接埋め込みます。

```yaml
default:
  inline_body: '{"status": "ok", "message": "pong"}'
  status_code: 200
```

---

### レスポンステンプレート

**シンプルエンジン（デフォルト・後方互換）:**

```json
{
  "order_id": "{{.order_id}}",
  "created_at": "{{.timestamp}}",
  "trace_id": "{{.uuid}}",
  "request_id": "{{.request_id}}"
}
```

```yaml
default:
  response_file: "./conf/mocks/order/detail_template.json"
  status_code: 200
  template:
    enabled: true
    # engine: "simple"  # デフォルト
```

**Go テンプレートエンジン — よりリッチな表現:**

```json
{
  "id": "{{uuid}}",
  "score": {{randomInt 1 100}},
  "label": "{{randomChoice \"foo\" \"bar\" \"baz\"}}",
  "ts_ms": {{timestampMs}},
  "encoded": "{{base64Encode .order_id}}"
}
```

```yaml
default:
  response_file: "./conf/mocks/order/dynamic.json"
  status_code: 200
  template:
    enabled: true
    engine: "go"
```

**Go テンプレート組み込み関数:**

| 関数 | シグネチャ | 説明 |
|------|-----------|------|
| `randomInt` | `(min, max int) int` | [min, max) の乱数整数 |
| `randomFloat` | `(min, max float64) float64` | [min, max) の乱数小数 |
| `randomChoice` | `(items ...string) string` | ランダムに文字列を選択 |
| `timestampMs` | `() int64` | 現在の Unix タイムスタンプ（ミリ秒） |
| `timestamp` | `() string` | 現在時刻（RFC3339 形式） |
| `uuid` | `() string` | ランダム UUID v4 |
| `base64Encode` | `(s string) string` | Base64 エンコード |
| `jsonEscape` | `(s string) string` | JSON 文字列エスケープ |
| `add/sub/mul/div` | `(a, b int) int` | 整数四則演算 |
| `env` | `(key string) string` | 環境変数を読み取り |
| `upper/lower/trim` | `(s string) string` | 文字列変換 |

---

### ランダムレスポンス

```yaml
default:
  random_responses:
    enabled: true
    files:
      - file: "./conf/mocks/random/success.json"
        weight: 70
        status_code: 200
      - file: "./conf/mocks/random/error.json"
        weight: 20
        status_code: 500
      - file: "./conf/mocks/random/timeout.json"
        weight: 10
        status_code: 200
        delay_ms: 3000
```

---

### プロキシモード

```yaml
path: "/api/v1/external/*path"
method: "ANY"
mode: "proxy"
proxy:
  target: "https://api.example.com"
  strip_prefix: "/api/v1/external"
  timeout_ms: 5000
  record: true                  # インタラクションを YAML スタブとして保存
  record_dir: "./recorded"
  fallback_on_error: true       # アップストリーム障害時にモックルールへフォールバック
  headers:
    X-Internal-Token: "secret"  # アップストリームへのリクエストに追加するヘッダー
```

---

### Content-Type の上書き

```yaml
rules:
  - conditions: []
    response_file: "./conf/mocks/data.xml"
    status_code: 200
    content_type: "application/xml"
```

---

## Admin REST API

全エンドポイントは設定した prefix（デフォルト `/mock-admin`）以下にマウントされます。

| メソッド | パス | 説明 |
|---------|------|------|
| `GET` | `/mock-admin/health` | サーバーヘルス・稼働時間 |
| `GET` | `/mock-admin/config` | 現在の設定（JSON） |
| `POST` | `/mock-admin/config/reload` | ホットリロードを手動トリガー |
| `GET` | `/mock-admin/endpoints` | 全エンドポイント一覧（ファイル定義 + ランタイム追加） |
| `POST` | `/mock-admin/endpoints` | ランタイムエンドポイントを追加（再起動不要） |
| `PUT` | `/mock-admin/endpoints/:id` | ランタイムエンドポイントを更新 |
| `DELETE` | `/mock-admin/endpoints/:id` | ランタイムエンドポイントを削除 |
| `GET` | `/mock-admin/requests` | リクエスト履歴（ページネーション: `?limit=50&offset=0`） |
| `DELETE` | `/mock-admin/requests` | リクエスト履歴をクリア |
| `GET` | `/mock-admin/requests/:id` | 特定リクエスト + レスポンスの詳細 |
| `GET` | `/mock-admin/metrics` | エンドポイント別集計統計 |
| `GET` | `/mock-admin/openapi.json` | OpenAPI 3.0 仕様（Swagger UI 用） |
| `GET` | `/mock-admin/ui/` | Web UI ダッシュボード |
| `GET` | `/mock-admin/ui/swagger.html` | Swagger UI |

### 例: ランタイムエンドポイントの追加

```bash
curl -X POST http://localhost:8080/mock-admin/endpoints \
  -H "Content-Type: application/json" \
  -d '{
    "path": "/api/v1/ping",
    "method": "GET",
    "default": {"status_code": 200, "inline_body": "{\"status\": \"ok\"}"}
  }'
```

### 例: リクエスト履歴の参照

```bash
curl "http://localhost:8080/mock-admin/requests?limit=10"
curl "http://localhost:8080/mock-admin/requests/req_42"
```

---

## Web UI ダッシュボード

`/mock-admin/ui/` でアクセス可能。バイナリに内包されているためビルドステップ不要。

| タブ | 内容 |
|-----|------|
| **Endpoints** | 登録済みエンドポイント一覧（メソッドバッジ・ルール数・詳細モーダル） |
| **Requests** | リクエスト履歴テーブル；クリックでヘッダー・ボディの詳細を確認 |
| **Metrics** | エンドポイント別リクエスト数・エラー率・平均 / 最小 / 最大レイテンシ |
| **Config** | 現在の設定を JSON ツリーで表示；リロードボタン付き |

Swagger UI（`/mock-admin/ui/swagger.html`）では、登録済みの全エンドポイントをブラウザから直接試すことができます。

---

## ヘルスチェック

```
GET /health
```

```json
{
  "status": "healthy",
  "uptime_sec": 3600.5,
  "config": {
    "loaded_at": "2026-02-27T10:00:00Z",
    "endpoints_count": 5
  }
}
```

---

## セレクタータイプ

| タイプ | 取得元 | キー形式 |
|-------|-------|---------|
| `body` | JSON リクエストボディ | [gjson パス](https://github.com/tidwall/gjson)（例: `data.user.id`） |
| `header` | HTTP リクエストヘッダー | ヘッダー名（例: `X-User-Type`） |
| `query` | URL クエリパラメータ | パラメータ名（例: `type`） |
| `path` | URL パスセグメント | `:param` パターンのパラメータ名 |

---

## マッチタイプ

| タイプ | 判定ロジック |
|-------|------------|
| `exact` | 完全一致 |
| `prefix` | 前方一致 |
| `suffix` | 後方一致 |
| `contains` | 部分一致 |
| `regex` | 正規表現マッチ |
| `range` | 数値範囲（例: `[1,100]` 閉区間、`(0,100)` 開区間） |

---

## 技術スタック

| コンポーネント | ライブラリ |
|-------------|----------|
| Web フレームワーク | `github.com/gin-gonic/gin` |
| 設定パース | `gopkg.in/yaml.v3` |
| JSON 抽出 | `github.com/tidwall/gjson` |
| ファイル監視 | `github.com/fsnotify/fsnotify` |
| ロギング | `go.uber.org/zap` |
| UUID 生成 | `github.com/google/uuid` |
| UI 内包 | Go `embed.FS`（標準ライブラリ） |
| プロキシ | `net/http/httputil`（標準ライブラリ） |
