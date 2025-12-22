# 需求规格说明书：基于配置的动态 Mock Server

## 1. 项目概述 (Project Overview)

本项目旨在开发一个基于 **Golang** 的轻量级 Mock API Server。
**核心目标**是解耦“代码逻辑”与“模拟数据”。开发者无需修改代码，仅通过修改 `config.yaml` 配置文件，即可定义 API 路由，并根据请求内容（Request Body/Header/Query）中的特定字段值，动态返回不同的 JSON 文件。

**关键特性：**

* **零代码变更**：新增接口或修改返回逻辑只需改配置。
* **动态分流**：支持根据请求参数值（如 `order_id` 或 `type`）返回不同结果。
* **强制兜底**：每个接口必须配置默认响应（Default Response），确保未命中规则时服务的高可用性。

---

## 2. 系统架构流程 (Logic Flow)

```mermaid
graph TD
    A[Client 发起请求] --> B(Server 接收请求);
    B --> C{匹配 URL & Method?};
    C -- No --> D[返回 404 Not Found];
    C -- Yes --> E[解析 Selector];
    E --> F[提取目标字段值];
    F --> G{遍历 Rules 匹配值?};
    G -- 命中规则 --> H[读取 Rule 指定的 JSON];
    G -- 未命中 --> I[读取 Default 指定的 JSON];
    H --> J[构建 Response (Status Code + Body)];
    I --> J;
    J --> K[返回给 Client];

```

---

## 3. 配置文件设计 (Configuration Schema)

配置文件采用 YAML 格式。核心在于 `endpoints` 的定义，每个 Endpoint 必须包含 `selector`（取值逻辑）、`rules`（匹配逻辑）和 `default`（兜底逻辑）。

### 3.1 示例配置文件 (`config.yaml`)

```yaml
server:
  port: 8080

endpoints:
  # 示例 1: 支付状态查询接口
  - path: "/api/v1/payment/status"
    method: "POST"
    description: "根据订单ID返回不同的支付结果"
    
    # 1. 取值选择器：定义依据哪个字段进行判断
    selector:
      type: "body"        # 可选: body (json), header, query
      key: "order_id"     # json path 或 header key
    
    # 2. 匹配规则：特殊情况处理
    rules:
      - match: "1001"     # 当 order_id == "1001"
        response_file: "./mocks/payment/success.json"
        status_code: 200
        
      - match: "1002"     # 当 order_id == "1002"
        response_file: "./mocks/payment/failed.json"
        status_code: 400

    # 3. 默认兜底策略 (必填)：所有未命中规则的请求都走这里
    default:
      response_file: "./mocks/payment/default.json"
      status_code: 200
      delay_ms: 0         # 可选：模拟延迟

  # 示例 2: 用户信息 (演示 Query 参数匹配)
  - path: "/api/v1/user"
    method: "GET"
    selector:
      type: "query"
      key: "user_type"    # URL?user_type=admin
    rules:
      - match: "admin"
        response_file: "./mocks/user/admin.json"
        status_code: 200
    default:
      response_file: "./mocks/user/guest.json"
      status_code: 200

```

---

## 4. 数据结构定义 (Go Structs)

为了准确解析上述 YAML，建议使用以下 Go 结构体定义。

```go
package main

type Config struct {
    Server    ServerConfig `yaml:"server"`
    Endpoints []Endpoint   `yaml:"endpoints"`
}

type ServerConfig struct {
    Port int `yaml:"port"`
}

type Endpoint struct {
    Path        string         `yaml:"path"`
    Method      string         `yaml:"method"`
    Description string         `yaml:"description"`
    Selector    Selector       `yaml:"selector"`
    Rules       []Rule         `yaml:"rules"`
    Default     ResponseConfig `yaml:"default"` // 强制兜底配置
}

type Selector struct {
    Type string `yaml:"type"` // "body", "header", "query"
    Key  string `yaml:"key"`  // e.g. "order_id", "data.user.id"
}

type Rule struct {
    Match          string `yaml:"match"`         // 匹配的关键值 (字符串形式)
    ResponseConfig `yaml:",inline"`              // 复用响应配置
}

type ResponseConfig struct {
    ResponseFile string `yaml:"response_file"`
    StatusCode   int    `yaml:"status_code"`
    DelayMs      int    `yaml:"delay_ms,omitempty"` // 模拟网络延迟 (毫秒)
}

```

---

## 5. 详细功能逻辑 (Functional Requirements)

### 5.1 启动阶段

1. 读取启动参数指定的 `config.yaml` 文件。
2. 解析 YAML 到内存结构体。
3. 遍历 `endpoints`，注册 HTTP 路由。
4. **预检建议**：启动时检查所有 `response_file` 路径是否存在，若不存在打印 Warning 日志。

### 5.2 请求处理阶段 (Request Handling)

**Step 1: 路由匹配**

* 拦截所有请求。
* 根据 Request URL 和 Method 查找对应的 `Endpoint` 配置。
* 若无匹配，返回 HTTP 404。

**Step 2: 特征值提取 (Selector)**
根据 `selector.type` 提取比较值 (`targetValue`)：

* **body**: 读取 Request Body，解析 JSON，使用 `selector.key` (支持嵌套，如 `data.id`) 提取值。
* *技术建议*: 使用 `github.com/tidwall/gjson` 实现高性能 JSON 路径提取。


* **header**: 读取 Request Header 中的 `key`。
* **query**: 读取 URL Query String 中的 `key`。
* *异常处理*: 若提取失败（如 JSON 格式错误或字段不存在），视作空字符串或特定标识，直接进入 Default 流程。

**Step 3: 规则匹配 (Rule Matching)**

* 遍历 `Endpoint.Rules` 列表。
* 比较：`if rule.Match == targetValue`。
* **命中**: 使用该 Rule 的 `ResponseConfig`。
* **未命中**: 循环结束后，使用 `Endpoint.Default` 的 `ResponseConfig`。

**Step 4: 响应构建**

1. **延迟模拟**: 如果配置了 `delay_ms`，线程休眠对应时间。
2. **读取文件**: 读取 `ResponseConfig.ResponseFile` 的内容。
3. **设置 Header**: 默认设置 `Content-Type: application/json`。
4. **发送响应**: 写入 Status Code 和文件内容。

---

## 6. 技术栈与依赖 (Tech Stack)

* **Language**: Go (Golang) >= 1.20
* **Web Framework**: `net/http` (标准库) 或 `github.com/gin-gonic/gin` (推荐，路由处理更方便)。
* **Config Parsing**: `gopkg.in/yaml.v3`
* **JSON Extraction**: `github.com/tidwall/gjson` (核心依赖，用于从 JSON Body 中动态取值)。

## 7. 目录结构建议 (Directory Structure)

```text
.
├── main.go               # 入口文件
├── config.go             # 结构体定义与解析逻辑
├── handler.go            # 核心处理逻辑 (提取与匹配)
├── config.yaml           # 配置文件
└── mocks/                # 存放 JSON 响应文件的目录
    ├── payment/
    │   ├── success.json
    │   ├── failed.json
    │   └── default.json
    └── user/
        └── ...

```