# AGENTS.md

## Overview

Go middleware server for a vision-based smart incubator. Connects MCU devices via MQTT to Alibaba Cloud OSS (object storage) and Tablestore (time-series DB). Two independent binaries: an MQTT listener and a web server.

## Build & Run

```bash
# Build both binaries
go build -o bin/listener ./cmd/server/
go build -o bin/web ./cmd/web/

# Run MQTT listener (requires env vars)
./bin/listener

# Run web server on :8080 (requires env vars)
./bin/web
```

There is no Makefile, no test suite, no linting, no CI, and no Dockerfile. `go vet` works; `go build` is the primary verification step.

## Environment Variables

All config via env vars. Copy `.env.example` to `.env` and fill in credentials. The app uses `godotenv` to load `.env` automatically at startup.

Critical vars: `OSS_ACCESS_KEY_ID`, `OSS_ACCESS_KEY_SECRET`, `REGION`, `BUCKET_NAME`, `TABLESTORE_ACCESS_KEY_ID`, `TABLESTORE_ACCESS_KEY_SECRET`, `TABLE_INSTANCE_NAME`, `TABLE_ENDPOINT`, `ENV_TABLE_NAME`, `ENV_MEASURE_NAME`, `COLONY_TABLE_NAME`, `COLONY_MEASURE_NAME`, `USERNAME`, `PASSWORD`, `PORT` (MQTT broker address), `DASHSCOPE_API_KEY`, `MODEL_NAME`.

Email alert vars: `SMTP_HOST`, `SMTP_PORT`, `SRC_EMAIL`, `DEST_EMAIL`, `AUTHCODE`. SMTP host/port default to `smtp.qq.com:465` when omitted.

Use `ENV_MEASURE_NAME` / `COLONY_MEASURE_NAME` for Tablestore measurement names; the code reads those exact names.

## Architecture

Two separate `main` packages — not a single entrypoint:

- `cmd/server/listener.go` — MQTT subscriber. Connects to broker via `PORT` env var. Subscribes to `device/#`. Dispatches to `utils.OnDataReceived` (env data → Tablestore), `utils.OnUploadRequest` (upload → OSS presign + MQTT reply + colony record), and `utils.SendAlert` for warn messages.

- `cmd/web/web.go` — HTTP server on `:8080`. Serves static files from `static/` and exposes `/api/env` and `/api/colony` JSON endpoints.

- `utils/` — Business logic: OSS presigning, Tablestore read/write, Bailian (Alibaba Cloud AI) inference.

- `web/` — Query functions `GetEnv()` and `GetColony()` called by the web handler.

- `static/` — Frontend HTML (Chart.js for env data, colony image viewer).

## Quirks & Gotchas

- **No tests exist.** If you add code, there's nothing to run to verify correctness beyond `go build`.
- **Filename typo:** `web/clonony.go` should be `colony.go`. The function inside is correctly named `GetColony`.
- **JSON field typo:** `EnvResponse.Sucess` is missing a `c` (should be `Success`). `ColonyResponse` uses correct `json:"success"` tag. These are baked into API responses — changing them would break clients.
- **Unused code:** `utils/bailian_utils.go` contains AI inference and `UploadSucess` function that are not called from anywhere.
- **Timestamp format:** MQTT messages use `"20060102-150405"` format, `Asia/Shanghai` timezone. Web API returns RFC3339 UTC. Tablestore truncates to whole seconds.
- **Presign expiry:** 10 minutes hardcoded in multiple places.
- **Presigned URLs:** Use OSS presigned URLs exactly as generated. Adding query params on the frontend can invalidate the signature.
- **OSS CORS:** `static/colony.html` fetches record text from OSS in the browser, so the bucket must allow cross-origin GET from the web origin.
- **Web static dir:** Served relative to CWD (`http.Dir("static")`), so web binary must be run from repo root.
- **Logging inconsistency:** `cmd/web/web.go` uses `log` (stdlib), while `utils/` and `cmd/server/` use `log/slog`. Don't introduce a third pattern.
- **`.env` is gitignored.** Never commit it. `.env.example` is the committed template.

## Conventions

- Chinese comments throughout (project is in Chinese).
- No error wrapping patterns — errors are logged and returned as JSON strings.
- OSS SDK v2 (`alibabacloud-oss-go-sdk-v2`), Tablestore SDK v1 (`aliyun-tablestore-go-sdk`).
