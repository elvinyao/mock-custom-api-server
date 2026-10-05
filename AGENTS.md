# Repository Guidelines

## Project Structure & Module Organization

This Go 1.26/Gin mock API server starts in `main.go`. Packages separate configuration and reloads (`config/`), routing and responses (`handler/`), middleware, admin APIs, proxying, recording, and metrics. Template rendering lives in `pkg/template/`. Tests sit beside source as `*_test.go`. `ui/static/` contains embedded dashboard and Swagger assets; no frontend build is required. Runtime settings, endpoint YAML, and JSON fixtures live in `conf/config.yaml`, `conf/endpoints/`, and `conf/mocks/`. Deployment manifests are in `k8s/`.

## Docker Execution Policy

Use the host only for file inspection, read-only Git operations, and edits through file-editing tools. Do not execute project code or use shell redirection on the host. Run all builds, tests, dependency operations, formatting, scripts, and servers through `bash .agent/run.sh <command> [args...]`; never invoke `docker run` directly. The runner uses Go 1.26 or Node 22, mounts the repository at `/workspace`, and removes containers after execution. Put compound commands inside the container using `sh -c`. If Docker fails, report the problem without a host fallback.

## Build, Test, and Development Commands

Run from the repository root:

- `bash .agent/run.sh go mod download` — fetch dependencies.
- `bash .agent/run.sh go build -o mock-api-server .` — build the binary.
- `bash .agent/run.sh go test ./...` — run all tests, matching release CI.
- `bash .agent/run.sh go test -cover ./...` — inspect coverage; no minimum is enforced.
- `bash .agent/run.sh node --test ui/app.test.cjs` — test dashboard behavior.
- `bash .agent/run.sh gofmt -w handler/handler.go` — format changed Go files.
- `AGENT_HTTP_PORT=8080 bash .agent/run.sh go run .` — start with `conf/config.yaml`; browse `http://localhost:8080/mock-admin/ui/`.

## Coding Style & Naming Conventions

Use `gofmt`, tab indentation, lowercase package names, exported `CamelCase`, and private `lowerCamelCase` identifiers. Keep YAML indentation at two spaces and configuration keys in `snake_case`. No dedicated lint configuration exists.

## Testing Guidelines

Go tests use `testing`, `httptest`, and `t.TempDir()` with test functions named `TestFunction_Scenario` and table-driven subtests. Dashboard tests use `node:test` in `ui/app.test.cjs`. Cover successful responses, invalid configuration, and changed error paths; run the suite before submitting.

## Commit & Pull Request Guidelines

Follow the prevalent `type: imperative summary` style: `feat:`, `fix:`, `refactor:`, `docs:`, `chore:`, or `ui:`. Keep commits focused. PRs should explain behavior changes, link relevant issues, report validation, and include screenshots for UI changes.

## Configuration Tips

Endpoint paths resolve relative to the main configuration file; response-file paths resolve from the working directory. Keep credentials and sensitive recorded payloads out of committed fixtures.
