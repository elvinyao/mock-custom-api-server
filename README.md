# mock-custom-api-server

A general-purpose, file-driven API mock server written in Go. Zero code changes required — configure endpoints, rules, and responses entirely through YAML files.

## Features

- **Rule-based routing** — match on request body fields, headers, query params, or path segments with exact, prefix, suffix, regex, or range matching
- **OR condition logic** — conditions within a rule can be evaluated as AND (default) or OR; condition groups give fine-grained control
- **Response templates** — simple `{{.var}}` substitution or a full Go `text/template` engine with built-in helpers
- **Random responses** — weighted random selection across multiple response files for chaos/load testing
- **Hot reload** — config file changes take effect without restarting the server
- **CORS middleware** — configurable allowed origins, methods, headers, and preflight handling
- **Request recording** — circular buffer captures every request and response; browsable via Admin API or Web UI
- **Admin REST API** — manage endpoints, inspect requests, reset scenarios, and view metrics at runtime
- **Web UI dashboard** — single-page app (Alpine.js + Tailwind CDN) embedded in the binary
- **Stateful scenario simulation** — model multi-step workflows (e.g. checkout → confirmed → shipped)
- **Proxy mode** — forward requests to real upstreams; optionally record interactions as ready-to-use YAML stubs
- **Per-endpoint metrics** — in-memory request count, error rate, and latency (avg/min/max)
- **Content-Type control** — per-response `content_type` field; defaults to `application/json`

---

## Quick Start

```bash
git clone <repository-url>
cd mock-custom-api-server
go mod tidy
go run main.go                        # uses ./config.yaml
go run main.go -config custom.yaml   # custom config path
```

Test a mock endpoint:

```bash
curl -X POST http://localhost:8080/api/v1/payment/status \
  -H "Content-Type: application/json" \
  -H "X-User-Type: premium" \
  -d '{"order_id": "VIP_1001"}'
```

Open the Web UI: `http://localhost:8080/mock-admin/ui/`

---

## Directory Structure

```
mock-custom-api-server/
  main.go                      entry point; wires all subsystems
  config.yaml                  main configuration file
  config/
    config.go                  all data structures
    loader.go                  YAML loading, defaults, validation
    watcher.go                 hot-reload file watcher
  handler/
    handler.go                 request dispatch, scenario transitions
    selector.go                value extraction (body/header/query/path)
    matcher.go                 rule matching (AND/OR, condition groups)
    response.go                response building, random selection, delay
  middleware/
    logger.go                  zap access log
    recovery.go                panic recovery
    cors.go                    CORS header injection + preflight
    recorder.go                captures request/response bodies
    metrics.go                 per-endpoint duration/status counters
  admin/
    handler.go                 admin API router + auth
    config.go                  GET /config, POST /config/reload
    endpoints.go               CRUD /endpoints
    requests.go                GET/DELETE /requests
    scenarios.go               GET /scenarios, POST /scenarios/:name/reset
    metrics.go                 GET /metrics
  proxy/
    handler.go                 reverse proxy with fallback support
    stub_writer.go             saves recorded interactions as YAML stubs
  state/
    store.go                   scenario step store (thread-safe)
  recorder/
    recorder.go                circular buffer recorder
  metrics/
    metrics.go                 in-memory metrics store
  ui/
    ui.go                      embed.FS registration
    static/index.html          single-file SPA (Alpine.js + Tailwind CDN)
  pkg/
    template/
      template.go              simple and Go text/template engines
  mocks/                       JSON response files
  config/endpoints/            endpoint YAML definitions
```

---

## Configuration Reference

### Main `config.yaml`

```yaml
server:
  port: 8080
  hot_reload: true
  reload_interval_sec: 5

  logging:
    level: "debug"          # debug | info | warn | error
    access_log: true
    log_format: "text"      # text | json
    log_file: ""            # empty = stdout

  error_handling:
    show_details: true
    custom_error_responses:
      404: "./mocks/errors/not_found.json"
      500: "./mocks/errors/internal_error.json"

  cors:
    enabled: true
    allowed_origins: ["*"]
    allowed_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"]
    allowed_headers: ["Content-Type", "Authorization", "X-Request-ID"]
    allow_credentials: false
    max_age_seconds: 86400

  admin_api:
    enabled: true
    prefix: "/mock-admin"   # all admin routes mounted here
    auth:
      enabled: false        # set true to require HTTP Basic Auth
      username: "admin"
      password: "secret"

  recording:
    enabled: true
    max_entries: 1000       # circular buffer size
    record_body: true
    max_body_bytes: 65536   # per request/response body cap
    exclude_paths:
      - "/health"
      - "/mock-admin/"

health_check:
  enabled: true
  path: "/health"

endpoints:
  config_paths:
    - "./config/endpoints/holiday.yaml"
    - "./config/endpoints/business_apis.yaml"
```

---

### Endpoint YAML — single file

```yaml
path: "/api/v1/payment/status"
method: "POST"
description: "Payment status mock"

selectors:
  - name: "order_id"
    type: "body"      # body | header | query | path
    key: "order_id"   # gjson path for body, key name otherwise
  - name: "user_type"
    type: "header"
    key: "X-User-Type"

rules:
  - conditions:
      - selector: "order_id"
        match_type: "prefix"   # exact | prefix | suffix | contains | regex | range
        value: "ERR_"
    response_file: "./mocks/payment/error_order.json"
    status_code: 400

  - conditions:
      - selector: "order_id"
        match_type: "prefix"
        value: "VIP_"
      - selector: "user_type"
        match_type: "exact"
        value: "premium"
    response_file: "./mocks/payment/vip_success.json"
    status_code: 200
    delay_ms: 50

default:
  response_file: "./mocks/payment/default.json"
  status_code: 200
```

---

### Multi-endpoint file

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
        response_file: "./mocks/user/admin.json"
        status_code: 200
    default:
      response_file: "./mocks/user/guest.json"
      status_code: 200
```

---

### OR Condition Logic

```yaml
rules:
  - condition_logic: "or"        # any single condition is enough to match
    conditions:
      - selector: "status"
        match_type: "exact"
        value: "pending"
      - selector: "status"
        match_type: "exact"
        value: "processing"
    response_file: "./mocks/order/in_progress.json"
    status_code: 200
```

### Condition Groups

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
    response_file: "./mocks/vip_regional.json"
    status_code: 200
```

---

### Response Templates

**Simple engine (default, backward-compatible):**

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
  response_file: "./mocks/order/detail_template.json"
  status_code: 200
  template:
    enabled: true
    # engine: "simple"   # default
```

**Go template engine — richer expressions:**

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
  response_file: "./mocks/order/dynamic.json"
  status_code: 200
  template:
    enabled: true
    engine: "go"
```

**Available Go template functions:**

| Function | Signature | Description |
|----------|-----------|-------------|
| `randomInt` | `(min, max int) int` | Random integer in [min, max) |
| `randomFloat` | `(min, max float64) float64` | Random float in [min, max) |
| `randomChoice` | `(items ...string) string` | Pick a random string |
| `timestampMs` | `() int64` | Current Unix timestamp in ms |
| `timestamp` | `() string` | Current RFC3339 timestamp |
| `uuid` | `() string` | Random UUID v4 |
| `base64Encode` | `(s string) string` | Base64 encode |
| `jsonEscape` | `(s string) string` | JSON-escape a string |
| `add/sub/mul/div` | `(a, b int) int` | Integer arithmetic |
| `env` | `(key string) string` | Read environment variable |
| `upper/lower/trim` | `(s string) string` | String transforms |

---

### Random Responses

```yaml
default:
  random_responses:
    enabled: true
    files:
      - file: "./mocks/random/success.json"
        weight: 70
        status_code: 200
      - file: "./mocks/random/error.json"
        weight: 20
        status_code: 500
      - file: "./mocks/random/timeout.json"
        weight: 10
        status_code: 200
        delay_ms: 3000
```

---

### Content-Type Override

```yaml
rules:
  - conditions: []
    response_file: "./mocks/data.xml"
    status_code: 200
    content_type: "application/xml"
```

---

### Stateful Scenario Simulation

Model multi-step workflows where each request advances state.

```yaml
path: "/api/v1/checkout/confirm"
method: "POST"
scenario: "checkout_flow"
scenario_key: "session_id"    # selector name that partitions state
selectors:
  - name: "session_id"
    type: "header"
    key: "X-Session-ID"

rules:
  - scenario_step: "idle"      # matches only when step == "idle"
    conditions: []
    next_step: "initiated"     # transitions to this step on match
    response_file: "./mocks/checkout/initiated.json"
    status_code: 200

  - scenario_step: "initiated"
    conditions: []
    next_step: "confirmed"
    response_file: "./mocks/checkout/confirmed.json"
    status_code: 200

  - scenario_step: "any"       # matches regardless of current step
    conditions: []
    response_file: "./mocks/checkout/invalid_state.json"
    status_code: 400
```

Reset a scenario via Admin API:

```bash
curl -X POST http://localhost:8080/mock-admin/scenarios/checkout_flow/reset
```

---

### Proxy Mode

Forward requests to a real upstream and optionally record interactions as YAML stubs.

```yaml
path: "/api/v1/external/*"
method: "ANY"
mode: "proxy"
proxy:
  target: "https://api.example.com"
  strip_prefix: "/api/v1/external"
  timeout_ms: 5000
  record: true
  record_dir: "./recorded"      # saved as YAML + JSON pairs
  fallback_on_error: true       # falls back to mock rules if upstream fails
  headers:
    X-Internal-Token: "secret"
```

---

## Admin REST API

All endpoints are mounted under the configured prefix (default `/mock-admin`).

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/mock-admin/health` | Server health + uptime |
| `GET` | `/mock-admin/config` | Current config as JSON |
| `POST` | `/mock-admin/config/reload` | Trigger hot-reload |
| `GET` | `/mock-admin/endpoints` | List all endpoints (file + runtime) |
| `POST` | `/mock-admin/endpoints` | Add runtime endpoint (in-memory, no restart) |
| `PUT` | `/mock-admin/endpoints/:id` | Update runtime endpoint by index |
| `DELETE` | `/mock-admin/endpoints/:id` | Remove runtime endpoint by index |
| `GET` | `/mock-admin/requests` | Paginated request history (`?limit=50&offset=0`) |
| `DELETE` | `/mock-admin/requests` | Clear request history |
| `GET` | `/mock-admin/requests/:id` | Inspect a single request + response |
| `GET` | `/mock-admin/scenarios` | List active scenarios and current steps |
| `POST` | `/mock-admin/scenarios/:name/reset` | Reset scenario state |
| `GET` | `/mock-admin/metrics` | Aggregate stats per endpoint |
| `GET` | `/mock-admin/ui/` | Web UI dashboard |

### Example: add a runtime endpoint

```bash
curl -X POST http://localhost:8080/mock-admin/endpoints \
  -H "Content-Type: application/json" \
  -d '{
    "path": "/api/v1/ping",
    "method": "GET",
    "default": {"status_code": 200, "response_file": "./mocks/ping.json"}
  }'
```

### Example: inspect request history

```bash
curl "http://localhost:8080/mock-admin/requests?limit=10"
curl "http://localhost:8080/mock-admin/requests/req_42"
```

---

## Web UI Dashboard

Available at `/mock-admin/ui/` — embedded in the binary, no build step required.

| Tab | Content |
|-----|---------|
| **Endpoints** | All registered endpoints (file-based + runtime), method badges, rule counts |
| **Requests** | Live request table; click Inspect to see full headers + body diff |
| **Scenarios** | Active scenarios with current step; Reset button per scenario |
| **Metrics** | Request count, error rate, avg/min/max latency per endpoint |
| **Config** | Current config as JSON tree; Trigger Reload button |

---

## Health Check

```
GET /health
```

```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z",
  "config": {
    "loaded_at": "2024-01-15T10:00:00Z",
    "endpoints_count": 5,
    "hot_reload": true
  }
}
```

---

## Selector Types

| Type | Source | Key format |
|------|--------|-----------|
| `body` | JSON request body | [gjson path](https://github.com/tidwall/gjson) e.g. `data.user.id` |
| `header` | HTTP request header | Header name e.g. `X-User-Type` |
| `query` | URL query string | Parameter name e.g. `type` |
| `path` | URL path segment | Parameter name from `:param` pattern |

---

## Match Types

| Type | Logic |
|------|-------|
| `exact` | `targetValue == value` |
| `prefix` | `strings.HasPrefix(targetValue, value)` |
| `suffix` | `strings.HasSuffix(targetValue, value)` |
| `contains` | `strings.Contains(targetValue, value)` |
| `regex` | `regexp.MatchString(value, targetValue)` |
| `range` | Numeric range; e.g. `[1,100]` (inclusive) or `(0,100)` (exclusive) |

---

## Tech Stack

| Component | Library |
|-----------|---------|
| Web framework | `github.com/gin-gonic/gin` |
| Config parsing | `gopkg.in/yaml.v3` |
| JSON extraction | `github.com/tidwall/gjson` |
| File watching | `github.com/fsnotify/fsnotify` |
| Logging | `go.uber.org/zap` |
| UUID generation | `github.com/google/uuid` |
| UI embedding | Go `embed.FS` (stdlib) |
| Proxy | `net/http/httputil` (stdlib) |
