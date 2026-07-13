# WOOM 重构实施计划

> 本计划以 Rust + Vue 为唯一实现。所有服务端、前端、测试和发布入口都必须遵循这一架构。

**目标：** 将 WOOM 建设为可自托管的轻量会议服务，支持 2～10 人通过链接加入会议，完成摄像头、麦克风、设备切换、屏幕共享、参会者查看和正常离会。

**技术栈：** Rust stable、Axum、Tokio、Serde、Redis、tracing、Vue 3、TypeScript strict、Vite、Tailwind CSS、daisyUI、Playwright、GitHub Actions。

## 任务 1：建立可运行基线

- [x] 使用 `docker compose up -d redis live777` 启动依赖服务。
- [x] 使用 `npm ci` 安装前端依赖。
- [x] 使用 Rust 服务提供 `/healthz` 和 `/readyz`。
- [x] 统一 API 错误格式，包含错误码、公开消息和请求 id。
- [x] 在 README 中记录 Rust 服务、Vue 前端和双浏览器 Demo 验收步骤。
- [x] 验证 `cargo test --manifest-path rust/Cargo.toml`、`npm run lint`、`npm run build` 和 `docker compose config`。

## 任务 2：冻结接口和存储契约

- [x] 使用 `contracts/woom-v1.yaml` 固定用户、房间、流、健康检查、诊断事件和 WHIP/WHEP 接口。
- [x] 统一 `roomId`、`streamId`、`token`、`streams` 等 JSON 字段名称。
- [x] 使用 Rust 契约测试验证房间、用户、流和错误响应。
- [x] Redis 房间统一使用带版本的 JSON：`{"version":1,"encoding":"json"}`。
- [x] Rust 只读写版本 1 的 JSON，不维护其他编码格式。

## 任务 3：实现 Rust 服务端

- [x] 创建 `rust/` 工程，启用 Rust stable、2024 版和 `forbid(unsafe_code)`。
- [x] 实现环境变量、JWT 校验、请求 id、健康检查和就绪检查。
- [x] 实现 Redis 房间、用户和流接口。
- [x] 实现 Live777 的 WHIP/WHEP 代理。
- [x] 在发布模式下由 Rust 二进制提供 Vue 静态文件。
- [x] 使用 `cargo fmt`、`cargo clippy` 和 `cargo test` 作为服务端质量门禁。

## 任务 4：实现结构化诊断

- [x] Rust 使用 JSON 格式输出一行一条结构化日志。
- [x] 统一记录 `timestamp`、`level`、`service`、`event`、`operation`、`request_id`、`session_id`、`room_id`、`stream_id` 和 `context`。
- [x] 前端日志器在开发环境输出 JSON，在生产环境上报必要的警告和错误。
- [x] `/client-events` 限制请求体大小，并过滤 token、SDP、ICE candidate、设备名称和媒体数据。
- [x] 为权限请求、设备枚举、WHIP、WHEP、房间刷新和离会清理记录生命周期事件。

## 任务 5：迁移到 Vue 和 daisyUI

- [x] 删除 React 入口、React 组件、React 配置和 React 专用依赖。
- [x] 使用 Vue 3、TypeScript、Vite、Tailwind CSS 和 daisyUI 重建会议页面。
- [x] 实现首页、准备页、会议页、设备选择、摄像头、麦克风、屏幕共享、复制链接和离会。
- [x] 保持 API、WHIP/WHEP 和媒体生命周期行为与契约一致。
- [x] 通过 `vue-tsc`、ESLint、单元测试和 Vue Playwright 流程验证。

## 任务 6：建立 Playwright 回归矩阵

- [x] 配置 Chromium、Firefox、Chrome 和 Edge 项目。
- [x] 覆盖创建会议、复制链接、第二个浏览器上下文加入、媒体控制和离会清理。
- [x] 在 GitHub Actions 中配置 Ubuntu、macOS、Windows 与 Chrome、Firefox、Edge 矩阵。
- [x] 失败时上传 Playwright 跟踪文件、HTML 报告和测试结果。
- [ ] 在远程 CI 中取得三种操作系统和三种浏览器的实际通过结果。

## 任务 7：发布和远程验证

- [x] 使用多阶段 Dockerfile 构建 Vue 静态文件和 Rust 发布二进制。
- [x] Compose 提供 Redis、Live777 和 Rust/Vue 应用服务。
- [x] Makefile 默认只执行 Vue 构建和 Rust 构建。
- [x] CI 默认只执行 Vue、Rust 和 Docker 检查。
- [ ] 完成可用 Docker 环境中的镜像构建验证。
- [ ] 完成 GitHub Actions 的远程 CI 验证。
- [ ] 在 macOS、Windows、Linux 上用真实浏览器完成人工会议验收。

## 当前结论

- Rust 是唯一服务端实现。
- Vue + daisyUI 是唯一前端实现。
- 仓库中不保留 Go 服务、Go 模块、Go 测试、Gob 编码或 Go 回退启动方案。
- 默认运行命令为 `cargo run --manifest-path rust/Cargo.toml` 和 `npm run dev`。
- 发布前剩余工作是 Docker 环境、远程 CI 和真实设备会议验收。
