# WOOM 重构实施计划

> **给执行人员：** 必须按任务逐项执行，并使用子任务驱动开发或执行计划规范；所有步骤使用复选框跟踪。每完成一个任务，都要运行该任务列出的验证命令。

**目标：** 在保持会议流程可用的前提下，将 WOOM 从 Go + React 逐步迁移到 Rust + Vue 3 + daisyUI，并建立跨平台、跨浏览器回归测试和结构化诊断能力。

**架构：** 采用渐进式替换。先固定 HTTP/JSON/Redis 契约并补测试，Go 和 React 在迁移期间作为可回退基线；Rust 通过同一接口接管服务，Vue 逐页替换 React，最后移除旧实现。

**技术栈：** Rust stable、Axum、Tokio、Serde、Redis、tracing、Vue 3、TypeScript strict、Pinia、Vite、Tailwind CSS、daisyUI、Playwright、GitHub Actions。

---

## 任务 1：建立可运行基线

**涉及文件：**

- 修改：`README.md`
- 修改：`package.json`
- 修改：`server/api/api.go`
- 修改：`server/daemon/daemon.go`
- 新增测试：`server/api/api_test.go`
- 新增测试：`server/api/v1/room_test.go`

- [x] **步骤 1：安装锁定版本的前端依赖**

在仓库根目录执行 `npm ci`。预期结果：命令成功结束，并且 `test -x node_modules/.bin/eslint` 不报错。

- [x] **步骤 2：在自动化测试前增加健康检查接口**

注册 `GET /healthz` 作为进程存活接口，返回状态码 200 和 `{"status":"ok"}`。注册 `GET /readyz` 作为依赖就绪接口，检查 Redis；Redis 不可用时返回状态码 503。两个接口都放在 JWT 中间件之外。

- [x] **步骤 3：统一 API 错误格式**

增加一个错误响应辅助函数，写出 `{"error":{"code":"...","message":"...","requestId":"..."}}`，并用于现有房间、用户和流的错误路径。保留原有 HTTP 状态码，不把 Redis 或 JWT 内部错误直接暴露给用户。

- [x] **步骤 4：为基线契约增加 API 测试**

使用 `httptest.NewServer` 和 Redis 测试客户端，覆盖 `POST /user/`、未授权的 `POST /room/`、已授权的新建房间以及 `GET /healthz`。断言状态码、JSON 字段名和 `X-Request-ID` 是否存在。

- [x] **步骤 5：记录可重复的 Demo 启动命令**

在 `README.md` 写明 Node 20+、Go 1.21+、Docker 的要求，以及 `docker compose up -d redis live777`、`npm ci`、`npm run build` 和应用启动命令。增加双浏览器创建会议和加入会议的手工检查表。

- [x] **步骤 6：验证基线**

执行 `go test ./...`、`npm run lint`、`npm run build` 和 `docker compose config`。预期结果：全部命令退出码为 0，Compose 输出包含 Redis 和 Live777 服务。

## 任务 2：冻结接口和存储契约

**涉及文件：**

- 新增：`contracts/woom-v1.yaml`
- 新增：`server/api/contract_test.go`
- 修改：`server/model/room.go`
- 修改：`server/model/model.go`
- 修改：`webapp/lib/api.ts`

- [x] **步骤 1：编写 OpenAPI 契约**

描述 `/user/`、`/room/`、`/room/{roomId}`、全部流接口、`/healthz`、`/readyz`、`/client-events`、`/whip/{uuid}` 和 `/whep/{uuid}` 的请求、响应、鉴权、错误和状态码行为。明确 `streamId`、`token`、`roomId` 和 `streams` 的实际 JSON 大小写。

- [x] **步骤 2：让 Go JSON 模型与契约一致**

明确 `Room`、`RoomAdmin`、`Stream`、`User` 和错误响应的类型。删除前端把 `locked` 固定当成 `false` 的假设，并保留 `presenter` 和 `streamId` 的可选语义。

- [x] **步骤 3：增加响应兼容性测试**

使用表格驱动测试序列化代表性的房间、流、用户和错误对象，再与契约中的字段集合比较。字段重命名或静默删除时测试必须失败。

- [x] **步骤 4：验证接口契约**

执行 `go test ./server/api/... ./server/model/...` 和 `npm run lint`。预期结果：两个命令都通过，JSON 字段大小写没有漂移。

## 任务 3：把 Redis 存储从 Gob 迁移到 JSON

**涉及文件：**

- 修改：`server/api/v1/helper.go`
- 修改：`server/helper/gob.go`
- 修改：`server/model/room.go`
- 新增：`server/helper/redis_room.go`
- 新增测试：`server/helper/redis_room_test.go`
- 新增测试：`server/api/v1/helper_test.go`

- [x] **步骤 1：定义带版本的 Redis 表示**

使用 `__schema` 哈希字段，其 JSON 值为 `{"version":1,"encoding":"json"}`。继续使用原有房间哈希和字段名，把 `admin` 与流字段的值改为 JSON。

- [x] **步骤 2：实现 JSON 优先、Gob 回退的解码**

当 `__schema.version` 为 1 时解码 JSON；没有该字段时解码现有 Gob。JSON 和 Gob 都损坏时返回明确的类型化错误，不要静默返回空房间。

- [x] **步骤 3：实现惰性迁移**

成功读取旧房间后，把管理员和流值写回 JSON，并增加 `__schema`。不删除原哈希，不改变房间 id，重复读取不得重复迁移。

- [x] **步骤 4：测试两种存储格式**

覆盖全新 JSON 房间、旧 Gob 房间、混合格式房间、损坏值和迁移后的第二次读取。断言第二次读取不会再次写 Redis。

- [x] **步骤 5：验证存储迁移**

执行 `go test ./server/helper ./server/api/v1 ./server/model`，并使用 `redis-cli HGETALL` 查看临时房间。预期结果：迁移后的房间含有 `__schema` 和 JSON 值，接口响应保持不变。

## 任务 4：增加面向 AI 排障的结构化诊断

**涉及文件：**

- 新增：`server/observability/log.go`
- 新增：`server/api/middleware/request_id.go`
- 新增：`webapp/lib/logger.ts`
- 修改：`server/api/api.go`
- 修改：`webapp/lib/api.ts`
- 新增：`server/api/client_events.go`
- 新增测试：`server/observability/log_test.go`
- 新增测试：`webapp/lib/logger.test.ts`

- [x] **步骤 1：定义统一事件字段**

统一使用 `timestamp`、`level`、`service`、`event`、`component`、`operation`、`request_id`、`session_id`、`room_id`、`stream_id`、`browser`、`os`、`duration_ms`、`error` 和 `context`，并为 `context` 定义允许使用的键名列表。

- [x] **步骤 2：在 Go 中增加请求关联**

当请求没有 `X-Request-ID` 时生成一个 id，将它放入请求上下文并写回响应头；使用 `log/slog` 将方法、路径、状态码、耗时和请求 id 输出为一条 JSON 日志。

- [x] **步骤 3：增加前端事件接口**

在 `POST /client-events` 接收最大 16 KiB 的 JSON 请求。写入日志前丢弃凭据、SDP、ICE candidate、设备 label 和媒体数据。合法事件返回 202，格式不合法返回 400。

- [x] **步骤 4：增加前端日志器和 API 错误类型**

每个浏览器会话生成一个 `session_id`。每次 API 请求发送 `X-Request-ID`；非 2xx 响应转换成包含状态码、错误码、消息和请求 id 的错误。开发环境以 JSON 输出警告和错误，生产环境只向 `/client-events` 上报警告和错误。

- [x] **步骤 5：记录媒体生命周期事件**

为权限请求、设备枚举、WHIP 启动/停止/重启/失败、WHEP 启动/停止/重启/失败、房间刷新和离会清理增加事件。事件带房间 id 和流 id，不得带 token 或 SDP。

- [x] **步骤 6：验证诊断输出**

执行日志单元测试并启动一个会议。预期结果：终端输出为每行一个 JSON；发生媒体或 API 失败时，可以用 `session_id` 和 `request_id` 筛选完整链路。

## 任务 5：引入 Playwright 回归测试

**涉及文件：**

- 修改：`package.json`
- 新增：`playwright.config.ts`
- 新增：`tests/e2e/meeting.spec.ts`
- 新增：`tests/e2e/fixtures/app.ts`
- 新增：`tests/e2e/assets/fake-camera.y4m`
- 新增：`.github/workflows/e2e.yml`
- 修改：`README.md`

- [ ] **步骤 1：固定 Playwright 并增加脚本**

把 `@playwright/test` 加入开发依赖，并增加 `test:e2e`、`test:e2e:ui` 和 `test:e2e:report` 脚本。版本写入 `package-lock.json`，保证所有运行器使用同一浏览器驱动。

- [ ] **步骤 2：定义确定性的应用启动方式**

让测试夹具启动或检查 Redis 与 Live777，启动 4000 端口的 Go 应用，等待 `/readyz`，再启动 5173 端口的 Vite。本地允许复用已有服务，持续集成使用隔离进程。

- [ ] **步骤 3：编写第一条会议流程**

用第一个浏览器上下文创建会议，用第二个上下文通过会议 id 加入。断言准备页面、加入后的页面、两个参会者卡片、音频/视频开关状态变化，以及离会后的流删除。

- [ ] **步骤 4：增加媒体和屏幕共享测试夹具**

授予摄像头和麦克风权限。基于 Chromium 的项目使用虚拟媒体参数和固定 Y4M 文件；Chrome 与 Edge 增加屏幕共享测试；Firefox 执行核心流程，并将屏幕共享能力记录为有名字的测试结果。

- [ ] **步骤 5：增加操作系统和浏览器矩阵**

在 GitHub Actions 中定义 Ubuntu、macOS、Windows 与 Chrome、Firefox、Edge 的组合。安装指定浏览器通道，失败时上传跟踪文件和 HTML 报告，并在任务摘要中列出操作系统、浏览器和测试结果。

- [ ] **步骤 6：在本地验证回归基线**

执行 `docker compose up -d redis live777`，再运行 `npm run test:e2e -- --project=chromium` 和 `npm run test:e2e -- --project=firefox`。预期结果：会议流程通过，失败时可以取得跟踪文件。

## 任务 6：在 Go 旁边建立 Rust 服务

**涉及文件：**

- 新增：`rust/Cargo.toml`
- 新增：`rust/woom-server/Cargo.toml`
- 新增：`rust/woom-server/src/main.rs`
- 新增：`rust/woom-server/src/config.rs`
- 新增：`rust/woom-server/src/error.rs`
- 新增：`rust/woom-server/src/http.rs`
- 新增：`rust/woom-server/src/auth.rs`
- 新增：`rust/woom-server/src/rooms.rs`
- 新增：`rust/woom-server/src/streams.rs`
- 新增：`rust/woom-server/src/live777.rs`
- 新增：`rust/woom-server/src/observability.rs`
- 新增测试：`rust/woom-server/tests/api_contract.rs`
- 新增：`rust/rust-toolchain.toml`
- 修改：`.github/workflows/build.yml`

- [ ] **步骤 1：建立严格的 Rust 工作区**

使用 Rust stable、2024 版、`forbid(unsafe_code)`，并在持续集成中执行 `cargo fmt --check`、`cargo check`、`cargo clippy --all-targets --all-features -- -D warnings` 和 `cargo test`。锁定直接依赖并提交 `rust/Cargo.lock`。

- [ ] **步骤 2：实现配置和错误类型**

解析 `SECRET`、`PORT`、`REDIS_URL`、`LIVE777_URL` 和 `LIVE777_TOKEN`，默认值与 Go 保持一致。定义可序列化的错误响应，包含错误码、公开消息和请求 id。

- [ ] **步骤 3：实现 JWT 和请求中间件**

校验签名算法、签名、过期时间、签发时间和生效时间，不接受任意算法。复用统一请求 id 和 JSON 日志字段。

- [ ] **步骤 4：实现 Redis JSON 房间和流操作**

只读写版本 1 的 JSON 值。匹配 Go 的房间 id、流 id、状态和 HTTP 响应行为。使用可取消的 Tokio 操作，并把 Redis 错误映射为稳定的 503/500 响应。

- [ ] **步骤 5：实现 Live777 代理和静态文件服务**

转发 `/whip/{uuid}` 和 `/whep/{uuid}`，设置可选的 Bearer token，保留请求方法和请求体，并在发布模式下由同一个二进制提供 Vue 构建目录。

- [ ] **步骤 6：执行 Go/Rust 契约对照**

把同一组固定请求分别发送到 Go 的 4000 端口和 Rust 的 4001 端口，比较状态码和解码后的 JSON 字段，然后执行 `cargo fmt --check && cargo clippy --all-targets --all-features -- -D warnings && cargo test`。

## 任务 7：用 Vue 和 daisyUI 替换 React

**涉及文件：**

- 修改：`package.json`
- 新增：`webapp/src/main.ts`
- 新增：`webapp/src/App.vue`
- 新增：`webapp/src/router.ts`
- 新增：`webapp/src/stores/session.ts`
- 新增：`webapp/src/stores/meeting.ts`
- 新增：`webapp/src/lib/api.ts`
- 新增：`webapp/src/lib/logger.ts`
- 新增：`webapp/src/features/meeting/WelcomePage.vue`
- 新增：`webapp/src/features/meeting/PreparePage.vue`
- 新增：`webapp/src/features/meeting/MeetingPage.vue`
- 新增：`webapp/src/features/device/DeviceBar.vue`
- 新增：`webapp/src/components/MediaTile.vue`
- 修改：`webapp/vite.config.ts`
- 新增：`webapp/tailwind.config.ts`
- 新增：`webapp/postcss.config.cjs`
- 新增：`webapp/src/style.css`
- 对照验证完成后删除：`webapp/app.tsx`、`webapp/main.tsx`、`webapp/pages/`、`webapp/components/`、`webapp/store/` 和 React 专用依赖。

- [ ] **步骤 1：增加 Vue 工具链但暂不删除 React**

加入 Vue、Pinia、Vue 编译器、`vue-tsc`、Tailwind CSS、daisyUI、Vue ESLint 支持和 Vite Vue 插件。增加 `typecheck`、`lint:vue` 和 `build:vue` 脚本；行为一致性验证完成前保留 React 脚本。

- [ ] **步骤 2：迁移领域类型和 API 客户端**

把房间、流、用户、错误和请求 id 类型迁移到带类型的 Vue 模块。保持 `contracts/woom-v1.yaml` 中的 `/user/`、`/room/`、流、WHIP 和 WHEP 路径完全一致。

- [ ] **步骤 3：迁移会话和会议状态**

使用 Pinia 管理持久化用户/会话数据、当前会议 id、参会者流、设备状态和屏幕共享状态。每个仓库只暴露带类型的动作，不直接访问 DOM。

- [ ] **步骤 4：迁移准备和会议流程**

使用带类型的属性、事件和生命周期清理，实现 `WelcomePage.vue`、`PreparePage.vue` 和 `MeetingPage.vue`。保留现有 WHIP/WHEP 行为以及 `beforeunload`/`unload` 清理，直到浏览器矩阵通过。

- [ ] **步骤 5：使用 daisyUI 重做会议控制区**

使用 daisyUI 的按钮、选择框、提示、加载状态、设置弹窗和响应式布局实现加入、设备、屏幕共享、复制链接和离会操作。固定控件尺寸，确保移动端文字不重叠。

- [ ] **步骤 6：执行 Vue 静态检查和行为一致性测试**

执行 `npm run lint:vue`、`npm run typecheck`、`npm run build:vue` 和完整 Playwright 会议流程。预期结果：React 基线中的每个场景都在 Vue 上通过后，才能删除 React。

## 任务 8：切换默认实现并发布一期版本

**涉及文件：**

- 修改：`Dockerfile`
- 修改：`compose.yml`
- 修改：`Makefile`
- 修改：`.github/workflows/build.yml`
- 修改：`.github/workflows/e2e.yml`
- 修改：`README.md`
- 全部验证通过后删除：Go 运行路径和 React 专用构建路径。

- [ ] **步骤 1：构建 Rust/Vue 发布镜像**

先构建 Vue 静态资源，再编译 Rust 发布二进制；运行镜像只复制二进制和静态资源，暴露 4000 端口，Redis 与 Live777 继续由外部 Compose 服务提供。

- [ ] **步骤 2：让持续集成执行全部门禁**

执行前端 lint、类型检查和构建；Go 回退实现存在时执行 Go 测试；执行 Rust 格式检查、编译、Clippy、测试、契约测试、Docker 构建以及操作系统/浏览器 Playwright 矩阵。失败任务上传日志、跟踪文件和报告。

- [ ] **步骤 3：执行手工会议检查表**

在一台 macOS、一台 Windows 和一台 Linux 设备上，使用两个支持的浏览器，授权摄像头和麦克风，创建并加入会议，切换设备、共享屏幕、离会，并确认 Redis 中没有残留流。

- [ ] **步骤 4：验证回滚**

保留上一版 Go 镜像和 React 构建产物。确认把服务命令切回 Go、把前端入口切回 React 后，会议流程仍然恢复，并且可以读取迁移后的 JSON 房间。

- [ ] **步骤 5：按顺序提交 PR**

按 `PR-0` 到 `PR-5` 的顺序提交，每个 PR 描述都包含范围、测试命令、必要的截图或跟踪文件、已知浏览器限制和准确回滚命令。前一个 PR 的门禁失败时，不合并后续 PR。

## 总体验收清单

- [ ] `npm ci && npm run lint && npm run build` 在 macOS、Ubuntu、Windows 上通过。
- [ ] Go 回退实现存在期间，`go test ./...` 通过。
- [ ] `cargo fmt --check && cargo clippy --all-targets --all-features -- -D warnings && cargo test` 通过。
- [ ] Playwright 会议流程在三种操作系统和 Chrome、Firefox、Edge 上通过。
- [ ] 旧 Gob 房间可以读取，并在访问时转换为 JSON。
- [ ] Rust 与 Go 返回相同的契约测试结果。
- [ ] 每个 API/媒体错误都能关联到结构化日志。
- [ ] 下次会议前，真实音频、视频和屏幕共享手工检查表通过。
