# 基于视觉的智能培养箱 — 后端服务器

连接物联网设备（MCU）、阿里云 OSS 对象存储、阿里云 Tablestore 时序数据库的中间件服务器，提供 MQTT 消息处理、数据持久化与 Web 可视化查询能力。

## 系统架构

```
                    ┌──────────────────────────────────────────────┐
                    │           智能培养箱后端服务器                 │
                    │                                              │
  MCU               │   ┌─────────────────┐      ┌──────────────┐  │
  设备  ──MQTT──▶  │   │  listener.go    │      │  web.go      │   │
  (发布)            │   │  (MQTT 订阅者)   │      │  (:8080)     │  │
                    │   └────────┬────────┘      └──────┬───────┘  │
                    │            │                      │          │
                    │            ▼                      ▼          │
                    │   ┌──────────────────┐  ┌──────────────────┐ │
                    │   │     OSS          │  │   Tablestore     │ │
                    │   │  对象存储（图片/  │  │  时序数据库       │ │
                    │   │    记录文件）     │  │  (env / colony)  │ │
                    │   └──────────────────┘  └──────────────────┘ │
                    │                                              │
                    │           Web 前端 (static/)                 │
                    │   env.html ──── /api/env                     │
                    │   colony.html ── /api/colony                 │
                    └──────────────────────────────────────────────┘
```

## 核心功能

- **环境数据采集** — 订阅 `device/{uuid}/data` 主题，将温度/湿度写入 Tablestore `env` 表
- **上传 URL 签发** — 订阅 `device/{uuid}/upload` 主题，为图片和记录文件生成 OSS 预签名上传 URL，通过 MQTT 回复给设备
- **菌落数据记录** — 在文件上传请求时同步将文件路径、菌落数写入 Tablestore `colony` 表
- **Web 数据查询** — 提供 `/api/env` 和 `/api/colony` 两个 JSON API，前端使用 Chart.js 展示温度/湿度曲线和菌落图像

## 快速开始

### 前置要求

- Go 1.26+
- 阿里云 OSS 和 Tablestore 服务（需提前创建好实例和数据表）
- MQTT Broker（如 Mosquitto），监听地址通过 `PORT` 环境变量配置

### 配置

复制环境变量模板并填入真实凭据：

```bash
cp .env.example .env
# 编辑 .env 填入密钥
```

### 构建

```bash
go build -o bin/listener ./cmd/server/
go build -o bin/web ./cmd/web/
```

### 运行

终端 1 — MQTT 订阅者（生产服务）：

```bash
./bin/listener
```

终端 2 — Web 服务器（`:8080`）：

```bash
./bin/web
```

浏览器访问 `http://localhost:8080` 查看仪表板。

## 环境变量

所有配置通过环境变量注入。

### 阿里云 OSS

| 变量 | 说明 |
|---|---|
| `OSS_ACCESS_KEY_ID` | AccessKey ID |
| `OSS_ACCESS_KEY_SECRET` | AccessKey Secret |
| `REGION` | OSS 地域 |
| `BUCKET_NAME` | 存储桶名称 |

### 阿里云 Tablestore

| 变量 | 说明 |
|---|---|
| `TABLESTORE_ACCESS_KEY_ID` | AccessKey ID |
| `TABLESTORE_ACCESS_KEY_SECRET` | AccessKey Secret |
| `TABLE_INSTANCE_NAME` | 实例名称 |
| `TABLE_ENDPOINT` | 实例访问地址 |
| `ENV_TABLE_NAME` | 环境数据表名 |
| `ENV_MEASURE_NAME` | 环境数据度量名 |
| `COLONY_TABLE_NAME` | 菌落数据表名 |
| `COLONY_MEASURE_NAME` | 菌落数据度量名 |

### MQTT

| 变量 | 说明 |
|---|---|
| `USERNAME` | Broker 用户名 |
| `PASSWORD` | Broker 密码 |
| `PORT` | Broker 连接地址，如 `tcp://localhost:1883` |

### 邮件告警（可选）

| 变量 | 说明 |
|---|---|
| `SMTP_HOST` | SMTP 服务器地址，默认 `smtp.qq.com` |
| `SMTP_PORT` | SMTP 端口，默认 `465` |
| `SRC_EMAIL` | 发件人邮箱 |
| `DEST_EMAIL` | 收件人邮箱 |
| `AUTHCODE` | SMTP 授权码 |

> 不启用邮件告警时，这一组变量可以不填。

## API 文档

### 查询环境数据

```
GET /api/env?uuid=<设备UUID>&start=<起始微秒>&end=<结束微秒>
```

响应示例：

```json
{
  "sucess": true,
  "env": [
    {"timestamp": "2026-06-27 12:00:00", "temp": 37.5, "hum": 65.2}
  ]
}
```

> `/api/env` currently returns the legacy field name `sucess` for frontend compatibility.

### 查询菌落数据

```
GET /api/colony?uuid=<设备UUID>&plateid=<盘位号>&start=<起始微秒>&end=<结束微秒>
```

响应示例：

```json
{
  "success": true,
  "colony": [
    {
      "timestamp": "2026-06-27 12:00:00",
      "number": 15,
      "image": {"success": true, "url": "https://...oss预签名地址..."},
      "record": {"success": true, "url": "https://...oss预签名地址..."}
    }
  ]
}
```

> start/end 为 Unix 微秒时间戳（整数），由 Web 前端自动转换。

## 项目结构

```
├── cmd/
│   ├── server/listener.go    # 生产 MQTT 订阅者（main 入口）
│   └── web/web.go            # Web 服务器（main 入口）
├── static/
│   ├── env.html              # 环境数据仪表板（Chart.js 折线图）
│   └── colony.html           # 菌落图像查看器（缩略图 + 边界框叠加）
├── utils/
│   ├── oss_utils.go          # OSS 预签名 URL 生成 + MQTT 回复
│   ├── tablestorage_utils.go # Tablestore 读写（env / colony 表）
│   ├── bailian_utils.go      # Bailian AI 推理
│   └── mail_utils.go         # 邮件告警
├── web/
│   ├── env.go                # /api/env 查询逻辑
│   └── clonony.go            # /api/colony 查询逻辑
├── go.mod / go.sum           # Go 模块依赖
├── .env.example              # 环境变量模板（可提交）
└── .env                      # 环境变量（已 gitignore，请勿提交！）
```

## MQTT 消息协议

### 设备上报环境数据

主题：`device/{uuid}/data`

```json
{"timestamp": "20060102-150405", "temp": 37.5, "hum": 65.2}
```

> timestamp 格式为 `YYYYMMDD-HHMMSS`，时区为 `Asia/Shanghai`。

服务端收到后写入 Tablestore `env` 表，时间戳截断到整秒。

### 设备请求上传

主题：`device/{uuid}/upload`

```json
{"timestamp": "20060102-150405", "plateid": 1, "imgpath": "/local/photo.jpg", "txtpath": "/local/result.txt", "number": 15}
```

服务端生成 OSS 预签名 PutObject URL，通过以下主题回复：

主题：`server/{uuid}/upload`

```json
{"timestamp": "20060102-150405", "success": true, "path": "/local/photo.jpg", "url": "https://oss预签名地址..."}
```

同时将图片路径、记录文件路径、菌落数写入 Tablestore `colony` 表。

### 服务端时间同步

主题：`server/{uuid}/time`

回复当前服务器 Unix 时间戳（秒）。

## 技术栈

| 组件 | 技术 |
|---|---|
| 语言 | Go 1.26 |
| MQTT 客户端 | Eclipse Paho (`eclipse/paho.mqtt.golang`) |
| 对象存储 | 阿里云 OSS SDK v2 |
| 时序数据库 | 阿里云 Tablestore SDK |
| Web 前端 | 原生 HTML/JS + Chart.js |
| Web 服务器 | 标准库 `net/http` |

## 注意事项

- **时区**：所有 MQTT 时间戳使用 `Asia/Shanghai`，格式 `"20060102-150405"`，解析失败时回退为当前时间
- **时间精度**：Tablestore 写入时时间戳截断到整秒（微秒值向下取整）
- **预签名 URL 有效期**：均为 10 分钟
- **OSS CORS**：`colony.html` 会在浏览器中 `fetch` 识别记录文本，OSS Bucket 需要允许 Web 页面来源的跨域 GET 请求
- **MQTT Broker 地址**：通过 `PORT` 环境变量配置，如 `tcp://localhost:1883`
