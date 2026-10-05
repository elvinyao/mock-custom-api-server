# Local Development Improvements Implementation Plan

> **For agentic workers:** Use superpowers:subagent-driven-development and superpowers:test-driven-development. Work through the assigned task, record failing and passing test evidence, and request specification review before code-quality review.

**Goal:** Make file-driven local mock development accurate, explainable, and easy to edit and reuse, implementing the six recommendations approved in this chat.

**Architecture:** Keep Go/Gin, YAML endpoint definitions, embedded Alpine.js, and the in-memory recorder. Use a single matching implementation for initial and runtime endpoints, strict configuration checks before activation, fresh fixture reads, directory-based reload watching, and additive request diagnostics. Keep endpoint export an explicit browser download rather than writing user files implicitly.

**Tech Stack:** Go 1.26, Gin, fsnotify, yaml.v3, Alpine.js, Node 22 built-in test runner for dashboard behavior. All executable commands go through `bash .agent/run.sh`; use `sh -c` for compound commands so the container retains its runtime PATH.

## Task 1: Configuration, Reload, and Templates

**Files:** `config/`, `pkg/template/`, `handler/response.go`, `main.go`, their tests, and `admin/config.go`.

- [x] Add regressions for fixture edits after an initial read; config atomic replacement and a second replacement; invalid configuration preserving the previous valid state; invalid templates; negative weights; shared template variables across response body and headers.
- [x] Run the focused tests and record expected failures before implementation.
- [x] Remove stale fixture caching for this local development server; preserve the public cache-clear hook for compatibility.
- [x] Watch parent directories, filter actual configuration paths, refresh the watch set, serialize/debounce reloads, stop pending work on shutdown, and honor the polling interval.
- [x] Introduce `config.LoadValidatedConfig(path string) (*Config, error)` and `config.ValidateEndpoint(ep Endpoint) error`; preserve `ValidateConfig`'s diagnostics interface for existing callers. Check condition groups, match types including contains, response status/delay/weights, path syntax, selector references, and Go templates. Include source file plus field paths in errors. Check syntax/structure without executing templates during validation.
- [x] Startup, watcher, and manual reload must activate only valid configurations. Add `--check-config` that exits successfully without starting the server for a valid file and nonzero with actionable diagnostics otherwise.
- [x] Add strict template rendering that returns errors and one rendering context shared by body/headers; retain legacy wrappers where needed for compatibility.
- [x] Run focused tests and submit evidence for review.

## Task 2: Routing and Request Explanations

**Files:** `handler/handler.go`, `handler/matcher.go`, new matching/diagnostic helpers and tests, `recorder/recorder.go`, `middleware/recorder.go` and tests.

- [x] First add failing tests: `/api/*rest` excludes `/apix`; `/org/:id/*rest` accepts `/org/1/a` with both captures; runtime endpoints follow the same semantics; specific static routes take precedence over parameter and wildcard routes; same request produces recorded explanations.
- [x] Keep admin/UI/health routes registered in Gin, dispatch mock routes through a single live matcher, and preserve existing method behavior and documented trailing-slash behavior.
- [x] Evaluate conditions and groups once, recording values and outcomes without changing AND/OR semantics. Record final response source after random selection or proxy fallback.
- [x] Store immutable structured data in Gin key `match_trace`, and add `MatchTrace any` with JSON key `match_trace` to recorded requests. Contract: `{endpoint, selectors, rules, selected_rule, response_source}`; rules contain `{index, matched, conditions, groups}` and conditions contain `{selector, actual, expected, match_type, matched}`.
- [x] Mark request bodies with additive `request_body_truncated` and `request_body_recorded` metadata so replay never silently sends missing/truncated bodies. Preserve full downstream bodies while bounding recording capture and skip body capture when disabled.
- [x] Run focused route, matcher, recorder, and middleware tests and submit for review.

## Task 3: Dashboard, Endpoint Editing, and Swagger

**Files:** `ui/static/index.html`, extracted `ui/static/app.js`, `ui/*test*`, `ui/package.json`, `admin/endpoints.go`, `admin/openapi.go`, `admin/requests.go`, new admin tests, and Swagger assets.

- [x] Add executable JavaScript behavior tests using Node's `node:test` before changing reload/history logic. Extract the Alpine app factory into a file that also exports it for tests.
- [x] Check HTTP status for all mutations; show reload errors accurately and retain the prior configuration. Add pause/resume refresh and preserve pagination while browsing older requests. Filter all recorded requests through the API rather than only the currently loaded page.
- [x] Add cURL copy and same-origin replay with method, query, and body preserved. Exclude browser-controlled transport headers; refuse missing/truncated request bodies and admin paths. Show the replay outcome without changing unrelated UI state.
- [x] Show recorded match explanations using the Task 2 schema.
- [x] Add endpoint JSON editor supporting create, duplicate, edit/delete runtime entries and JSON-to-YAML browser export. File entries are duplicated rather than overwritten. Validate create/update through `config.ValidateEndpoint`; show errors inline. Display the in-memory/restart lifecycle explicitly. YAML export must round-trip a complete endpoint, not just common fields.
- [x] Use a same-origin relative OpenAPI server URL; serve pinned Swagger UI JS/CSS locally with licenses. No CDN dependency for Swagger startup.
- [x] Run dashboard JavaScript tests, admin/UI Go tests, and browser smoke verification if supported in Docker.

## Task 4: Proxy Fidelity

**Files:** `proxy/handler.go`, `proxy/stub_writer.go`, new proxy regression tests.

- [x] Add failing integration tests with actual httptest upstreams: multiple Set-Cookie headers, truncated response bodies, fallback on body errors, request-body read errors, repeated/concurrent recording names.
- [x] Preserve repeated end-to-end headers, remove hop-by-hop headers, and handle read errors before forwarding/recording. Keep existing timeout and fallback behavior.
- [x] Use unique recording names and atomic file writes; return recording errors to the caller for logging without losing a successful upstream response.
- [x] Run `bash .agent/run.sh go test -race ./proxy` and submit for review.

## Task 5: Integration and Documentation

**Files:** `README.md`, `.agent/run.sh`, `.github/workflows/ci.yml`, plan progress.

- [x] Add Node 22 selection and persistent dependency caches to the Docker runner while preserving the existing command interface.
- [x] Add ordinary push/PR verification for Go race tests and dashboard behavior tests; retain release workflow.
- [x] Document strict checks, fresh response fixtures, routing precedence, reload behavior, replay limitations, endpoint export/import and Docker-only commands, preserving README's Japanese language.
- [x] Specification review: compare each delivered behavior with this plan and resolve omissions.
- [x] Code-quality review: review integration boundaries, concurrency, tests, escaping, and failure paths; fix actionable findings.
- [x] Verify `bash .agent/run.sh go test -race -cover ./...`, `bash .agent/run.sh go vet ./...`, `bash .agent/run.sh node --test ui/app.test.cjs`, and `bash .agent/run.sh go run . --check-config -config conf/config.yaml`. Confirm no unexpected file changes.

## Decisions

- The user's implementation request approves the prior recommendations; no additional design approval gate is needed.
- Work in the current checkout on a `codex/` branch, preserving the existing untracked contributor guide and runner. Leave changes reviewable rather than pushing or publishing.
- Independent domains have explicit file ownership. Dependent interface integration and final reviews run sequentially.

## Completion and Verification — 2026-10-05

All five tasks are complete on `codex/local-mock-improvements`.

- Regression tests were observed failing before the corresponding fixes for configuration/reload, routing/recording, proxy fidelity, dashboard behavior, and template compatibility.
- Independent specification reviews approved the configuration task, routing/dashboard/proxy tasks, and integration/documentation task.
- Independent code-quality reviews approved the final changes after fixes for body-read error preservation, proxy error traces, cURL globbing, stale asynchronous UI responses, runtime endpoint mutation ordering, literal selector names, and nested JSON template braces.
- `bash .agent/run.sh go test -race -cover ./...` passed after the final template fix across all ten packages.
- `bash .agent/run.sh go vet ./...` passed.
- `bash .agent/run.sh node --test ui/app.test.cjs` passed all 26 dashboard tests.
- `bash .agent/run.sh go run . --check-config -config conf/config.yaml` passed for all 13 example endpoints.
- `bash .agent/run.sh go build -o .agent/mock-api-server-linux .` passed.
- Actual Chromium smoke checks passed after the dashboard review fixes: fresh fixtures, invalid endpoint diagnostics, create/edit/export/delete, retained pagination, filtering, trace inspection, cURL copy, replay, failed reload preservation and recovery, and offline Swagger. Screenshots were inspected; no browser JavaScript errors or external asset requests occurred.
- The optional browser driver passed `node --check`; its installation and smoke execution steps were also exercised inside Docker. GitHub CI was added but has not been run remotely.
- Final diff whitespace check passed. The test server and temporary browser container were shut down.

The existing startup lifecycle remains: port, TLS, and middleware settings require a restart. Runtime endpoint changes stay in memory until explicitly exported and added to the configuration files.
