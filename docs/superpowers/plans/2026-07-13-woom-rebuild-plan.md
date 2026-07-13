# WOOM Rebuild Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在保持会议流程可用的前提下，将 WOOM 从 Go + React 逐步迁移到 Rust + Vue 3 + daisyUI，并建立跨平台、跨浏览器回归和结构化诊断能力。

**Architecture:** 采用渐进式替换。先固定 HTTP/JSON/Redis 契约并补测试，Go 和 React 在迁移期间作为可回退基线；Rust 通过同一端口契约接管服务，Vue 逐页替换 React，最后移除旧实现。

**Tech Stack:** Rust stable、Axum、Tokio、Serde、Redis、tracing、Vue 3、TypeScript strict、Pinia、Vite、Tailwind CSS、daisyUI、Playwright、GitHub Actions。

---

## Task 1: 建立可运行基线

**Files:**
- Modify: `README.md`
- Modify: `package.json`
- Modify: `server/api/api.go`
- Modify: `server/daemon/daemon.go`
- Test: `server/api/api_test.go`
- Test: `server/api/v1/room_test.go`

- [ ] **Step 1: Install the locked frontend dependencies**

Run `npm ci` from the repository root. Expected: the command completes successfully and `test -x node_modules/.bin/eslint` prints no error.

- [ ] **Step 2: Add health endpoints before starting E2E**

Register `GET /healthz` as a process liveness response with status 200 and body `{"status":"ok"}`. Register `GET /readyz` as a dependency readiness response that pings Redis and returns status 503 when Redis is unavailable. Keep both endpoints outside the JWT middleware group.

- [ ] **Step 3: Make API failures machine-readable**

Add one helper that writes `{"error":{"code":"...","message":"...","requestId":"..."}}` and use it for existing room, user, and stream error paths. Preserve HTTP status codes and do not expose Redis or JWT internals in the user-facing message.

- [ ] **Step 4: Add API tests for the baseline contract**

Use `httptest.NewServer` with a Redis test client to cover `POST /user/`, unauthorized `POST /room/`, authorized room creation, and `GET /healthz`. Assert status, JSON field names, and the presence of `X-Request-ID`.

- [ ] **Step 5: Document the reproducible Demo command**

Update `README.md` with Node 20+, Go 1.21+, Docker, `docker compose up -d redis live777`, `npm ci`, `npm run build`, and the application start command. Add a two-browser manual checklist for creating and joining one meeting.

- [ ] **Step 6: Verify the baseline**

Run `go test ./...`, `npm run lint`, `npm run build`, and `docker compose config`. Expected: all commands exit 0; the compose output contains Redis and Live777 services.

## Task 2: Freeze the API and storage contracts

**Files:**
- Create: `contracts/woom-v1.yaml`
- Create: `server/api/contract_test.go`
- Modify: `server/model/room.go`
- Modify: `server/model/model.go`
- Modify: `webapp/lib/api.ts`

- [ ] **Step 1: Write the OpenAPI contract**

Describe the exact request, response, auth, error, and status behavior for `/user/`, `/room/`, `/room/{roomId}`, all stream routes, `/healthz`, `/readyz`, `/client-events`, `/whip/{uuid}`, and `/whep/{uuid}`. Mark `streamId`, `token`, `roomId`, and `streams` with their actual current JSON casing.

- [ ] **Step 2: Align Go JSON models with the contract**

Make `Room`, `RoomAdmin`, `Stream`, `User`, and error payload types explicit. Remove frontend assumptions that `locked` is always `false`; model it as boolean and preserve optional `presenter` and `streamId` semantics.

- [ ] **Step 3: Add response compatibility tests**

Create table-driven tests that marshal representative room, stream, user, and error values and compare decoded JSON objects to the OpenAPI field set. The tests must fail if a field is renamed or silently removed.

- [ ] **Step 4: Verify contract fixtures**

Run `go test ./server/api/... ./server/model/...` and `npm run lint`. Expected: both pass with no JSON casing drift.

## Task 3: Migrate Redis storage from Gob to JSON

**Files:**
- Modify: `server/api/v1/helper.go`
- Modify: `server/helper/gob.go`
- Modify: `server/model/room.go`
- Create: `server/helper/redis_room.go`
- Test: `server/helper/redis_room_test.go`
- Test: `server/api/v1/helper_test.go`

- [ ] **Step 1: Define the versioned Redis representation**

Use a `__schema` hash field with JSON value `{"version":1,"encoding":"json"}`. Store the `admin` and stream fields as JSON values using the existing hash key and field names.

- [ ] **Step 2: Implement JSON-first, Gob-fallback decoding**

Decode JSON when `__schema.version` is 1. If the schema field is absent, decode the existing Gob value. Return a typed error for malformed JSON and malformed Gob instead of silently returning an empty room.

- [ ] **Step 3: Implement lazy migration**

When a legacy room is successfully read, write its admin and stream values back as JSON and add `__schema`. Do not delete the original hash or change the room id. Make the migration idempotent.

- [ ] **Step 4: Test both formats**

Cover a new JSON room, a legacy Gob room, a mixed room, malformed values, and a second read after migration. Assert that the second read does not produce another write.

- [ ] **Step 5: Verify the storage migration**

Run `go test ./server/helper ./server/api/v1 ./server/model` and inspect a temporary Redis hash with `redis-cli HGETALL`. Expected: a migrated room contains `__schema` and JSON values, and its API response is unchanged.

## Task 4: Add AI-oriented structured diagnostics

**Files:**
- Create: `server/observability/log.go`
- Create: `server/api/middleware/request_id.go`
- Create: `webapp/lib/logger.ts`
- Modify: `server/api/api.go`
- Modify: `webapp/lib/api.ts`
- Create: `server/api/client_events.go`
- Test: `server/observability/log_test.go`
- Test: `webapp/lib/logger.test.ts`

- [ ] **Step 1: Define the shared event fields**

Use the fields `timestamp`, `level`, `service`, `event`, `component`, `operation`, `request_id`, `session_id`, `room_id`, `stream_id`, `browser`, `os`, `duration_ms`, `error`, and `context`. Define an explicit allowlist for context keys.

- [ ] **Step 2: Add request correlation to Go**

Generate a request id when `X-Request-ID` is absent, store it in the request context, echo it in the response, and log method, path, status, duration, and request id as one JSON event using `log/slog`.

- [ ] **Step 3: Add the client event endpoint**

Accept only JSON bodies up to 16 KiB at `POST /client-events`. Drop credentials, SDP, ICE candidates, device labels, and media payloads before writing the event. Return 202 for accepted events and 400 for invalid event shapes.

- [ ] **Step 4: Add the frontend logger and API error type**

Generate one `session_id` per browser session. Make every API request send `X-Request-ID`; convert non-2xx responses into an error containing status, code, message, and request id. Log warnings/errors as JSON and send only errors plus warnings to `/client-events`.

- [ ] **Step 5: Instrument media lifecycle events**

Emit events for permission request, device enumeration, WHIP start/stop/restart/failure, WHEP start/stop/restart/failure, room refresh, and leave cleanup. Include room and stream ids; never include token or SDP values.

- [ ] **Step 6: Verify diagnostic output**

Run the logger unit tests and start the app with one meeting. Expected: terminal output is line-delimited JSON and a failed media/API operation can be filtered by `session_id` and `request_id`.

## Task 5: Introduce Playwright regression tests

**Files:**
- Modify: `package.json`
- Create: `playwright.config.ts`
- Create: `tests/e2e/meeting.spec.ts`
- Create: `tests/e2e/fixtures/app.ts`
- Create: `tests/e2e/assets/fake-camera.y4m`
- Create: `.github/workflows/e2e.yml`
- Modify: `README.md`

- [ ] **Step 1: Pin Playwright and add scripts**

Add `@playwright/test` to dev dependencies and scripts named `test:e2e`, `test:e2e:ui`, and `test:e2e:report`. Keep the version in `package-lock.json` so all runners use the same browser driver.

- [ ] **Step 2: Define deterministic application startup**

Configure the fixture to start or verify Redis and Live777, start the Go app on port 4000, wait for `/readyz`, and start Vite on port 5173. Use `reuseExistingServer` locally and isolated processes in CI.

- [ ] **Step 3: Write the first meeting flow**

Create one browser context to create a meeting and a second context to join through the copied meeting id. Assert the prepare screen, join transition, two participant tiles, an audio/video toggle state change, and stream deletion after leave.

- [ ] **Step 4: Add media and screen-share fixtures**

Grant camera and microphone permissions. Chromium-based projects use fake media flags and a fixed Y4M file. Add a screen-share test for Chrome and Edge with the display-capture flag; Firefox runs the core flow and records unsupported display-capture behavior as a named test result.

- [ ] **Step 5: Add the OS/browser matrix**

Define GitHub Actions matrix entries for Ubuntu, macOS, Windows and Playwright projects for Chrome, Firefox, and Edge. Install the exact browser channels, upload traces and HTML reports on failure, and keep a job summary with OS/browser/test result.

- [ ] **Step 6: Verify the baseline matrix locally**

Run `npm run test:e2e -- --project=chromium` and `npm run test:e2e -- --project=firefox` after `docker compose up -d redis live777`. Expected: the meeting flow passes and a trace is available for any failure.

## Task 6: Build the Rust service beside Go

**Files:**
- Create: `rust/Cargo.toml`
- Create: `rust/woom-server/Cargo.toml`
- Create: `rust/woom-server/src/main.rs`
- Create: `rust/woom-server/src/config.rs`
- Create: `rust/woom-server/src/error.rs`
- Create: `rust/woom-server/src/http.rs`
- Create: `rust/woom-server/src/auth.rs`
- Create: `rust/woom-server/src/rooms.rs`
- Create: `rust/woom-server/src/streams.rs`
- Create: `rust/woom-server/src/live777.rs`
- Create: `rust/woom-server/src/observability.rs`
- Create: `rust/woom-server/tests/api_contract.rs`
- Create: `rust/rust-toolchain.toml`
- Modify: `.github/workflows/build.yml`

- [ ] **Step 1: Create the strict Rust workspace**

Use a Rust stable toolchain, edition 2024, `forbid(unsafe_code)`, `cargo fmt --check`, `cargo check`, `cargo clippy --all-targets --all-features -- -D warnings`, and `cargo test` in CI. Pin all direct dependencies in `rust/Cargo.toml` and commit `rust/Cargo.lock`.

- [ ] **Step 2: Implement configuration and errors**

Parse `SECRET`, `PORT`, `REDIS_URL`, `LIVE777_URL`, and `LIVE777_TOKEN` with typed defaults matching Go. Define one serializable error envelope with code, public message, and request id.

- [ ] **Step 3: Implement JWT and request middleware**

Validate the signing algorithm, signature, expiration, issued-at, and not-before claims. Do not accept an arbitrary algorithm. Reuse the shared request id and JSON log fields.

- [ ] **Step 4: Implement Redis JSON room and stream operations**

Read and write only version 1 JSON values. Match Go’s room id, stream id, status, and HTTP response behavior. Use cancellation-aware Tokio operations and map Redis errors to stable 503/500 responses.

- [ ] **Step 5: Implement Live777 proxy and static serving**

Forward `/whip/{uuid}` and `/whep/{uuid}`, set the optional bearer token, preserve request method and body, and serve the Vue build directory from the same binary in release mode.

- [ ] **Step 6: Run Go/Rust contract comparison**

Run the same fixture requests against Go port 4000 and Rust port 4001, compare status codes and decoded JSON fields, then run `cargo fmt --check && cargo clippy --all-targets --all-features -- -D warnings && cargo test`.

## Task 7: Replace React with Vue and daisyUI

**Files:**
- Modify: `package.json`
- Create: `webapp/src/main.ts`
- Create: `webapp/src/App.vue`
- Create: `webapp/src/router.ts`
- Create: `webapp/src/stores/session.ts`
- Create: `webapp/src/stores/meeting.ts`
- Create: `webapp/src/lib/api.ts`
- Create: `webapp/src/lib/logger.ts`
- Create: `webapp/src/features/meeting/WelcomePage.vue`
- Create: `webapp/src/features/meeting/PreparePage.vue`
- Create: `webapp/src/features/meeting/MeetingPage.vue`
- Create: `webapp/src/features/device/DeviceBar.vue`
- Create: `webapp/src/components/MediaTile.vue`
- Modify: `webapp/vite.config.ts`
- Create: `webapp/tailwind.config.ts`
- Create: `webapp/postcss.config.cjs`
- Create: `webapp/src/style.css`
- Delete after parity: `webapp/app.tsx`, `webapp/main.tsx`, `webapp/pages/`, `webapp/components/`, `webapp/store/`, and React-only dependencies.

- [ ] **Step 1: Add Vue tooling without deleting React**

Add Vue, Pinia, Vue compiler, `vue-tsc`, Tailwind CSS, daisyUI, ESLint Vue support, and the Vite Vue plugin. Add `typecheck`, `lint:vue`, and `build:vue` scripts. Keep the React scripts available until parity is verified.

- [ ] **Step 2: Port domain types and API client**

Move the room, stream, user, error, and request-id types into typed Vue modules. Keep the exact `/user/`, `/room/`, stream, WHIP, and WHEP paths from `contracts/woom-v1.yaml`.

- [ ] **Step 3: Port session and meeting state**

Use Pinia stores for persisted user/session data, current meeting id, participant streams, device state, and screen-share state. Each store exposes typed actions and has no direct DOM access.

- [ ] **Step 4: Port the prepare and meeting flows**

Implement `WelcomePage.vue`, `PreparePage.vue`, and `MeetingPage.vue` with typed props/emits and lifecycle cleanup. Preserve the existing WHIP/WHEP behavior and `beforeunload`/`unload` cleanup until the browser matrix proves the replacement.

- [ ] **Step 5: Apply daisyUI components to the meeting controls**

Use daisyUI buttons, selects, alerts, loading indicators, modal settings, and responsive layout for join, device, share-screen, copy-link, and leave actions. Keep control dimensions stable and ensure labels do not overlap at mobile width.

- [ ] **Step 6: Run Vue static checks and E2E parity**

Run `npm run lint:vue`, `npm run typecheck`, `npm run build:vue`, and the full Playwright meeting flow. Expected: every React baseline scenario passes against Vue before removing React.

## Task 8: Cut over and publish the first usable release

**Files:**
- Modify: `Dockerfile`
- Modify: `compose.yml`
- Modify: `Makefile`
- Modify: `.github/workflows/build.yml`
- Modify: `.github/workflows/e2e.yml`
- Modify: `README.md`
- Delete after verification: Go runtime and React-only build paths

- [ ] **Step 1: Build the Rust/Vue release image**

Build the Vue static assets, compile the Rust release binary, copy only the binary and static assets into the runtime image, expose port 4000, and keep Redis/Live777 as external compose dependencies.

- [ ] **Step 2: Make CI run all required gates**

Run frontend lint/typecheck/build, Go tests while fallback exists, Rust format/check/clippy/tests, contract tests, Docker build, and the OS/browser Playwright matrix. Upload logs, traces, and reports for failed jobs.

- [ ] **Step 3: Execute the manual meeting checklist**

On one macOS, one Windows, and one Linux machine, open two supported browsers, grant camera/microphone permissions, create and join a meeting, toggle devices, share the screen, leave, and verify no stale stream remains in Redis.

- [ ] **Step 4: Verify rollback**

Keep the prior Go image and React build artifact available. Confirm that changing the service command back to Go and the frontend entry back to React restores the same meeting flow and can read JSON-migrated rooms.

- [ ] **Step 5: Publish the PR sequence**

Open the PRs in the order `PR-0` through `PR-5`, with each description containing scope, test commands, screenshots or traces where relevant, known browser limitations, and the exact rollback command. Do not merge a later PR when its predecessor’s required CI gate is red.

## Verification checklist

- [ ] `npm ci && npm run lint && npm run build` passes on macOS, Ubuntu, and Windows.
- [ ] `go test ./...` passes while Go fallback exists.
- [ ] `cargo fmt --check && cargo clippy --all-targets --all-features -- -D warnings && cargo test` passes.
- [ ] Playwright meeting flow passes for Chrome, Firefox, and Edge on all three OS runners.
- [ ] Redis Gob rooms are readable and lazily converted to JSON.
- [ ] Rust and Go return matching contract fixtures.
- [ ] Every API/media error has correlated structured logs.
- [ ] Manual audio/video/screen-share checklist passes before the next meeting.
