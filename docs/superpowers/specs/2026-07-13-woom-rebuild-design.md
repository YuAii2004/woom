# WOOM 重构设计

## 目标

把 WOOM 演进成腾讯会议的轻量自托管替代品。一期的发布门槛是：2～10 人可以通过链接加入会议，完成摄像头和麦克风的开启与关闭、设备切换、屏幕共享、查看其他参会者并正常离会；会议服务能够在下一次真实会议中稳定运行。

录制、聊天、主持人权限、会议纪要和更大规模并发不属于一期门槛，但接口、状态模型和日志字段要为后续扩展保留空间。

## 当前情况

- 服务端是 Go 1.21 项目，使用 Chi、Redis 和 Live777；`server/api` 提供房间、用户、流以及 WHIP/WHEP 代理接口。
- 前端是 React 18、TypeScript、Vite 和 UnoCSS，状态由 Jotai 与 Zustand 分担，媒体控制封装在 `webapp/components/use/`。
- Redis 房间哈希表的字段值使用 Go Gob 编码，直接改成 Rust 会破坏已有房间数据的读取能力。
- 当前只有 Go 辅助模块测试；持续集成只在 macOS、Ubuntu、Windows 上做 npm 和 Go 构建，没有真实会议流程和浏览器测试。
- 当前 `npm run build` 依赖完整的 `node_modules`；原工作区缺少 `eslint` 可执行文件，因此前端基线尚未通过。

## 设计原则

1. 先建立可重复的 Demo 和回归基线，再做语言和框架迁移。
2. 迁移期间保持 HTTP 路由、JSON 字段和 WebRTC 代理地址不变，使每个 PR 都能独立验证和回滚。
3. 用带版本的 JSON 替代 Gob；Go 先兼容读取 Gob 并写出 JSON，Rust 只依赖 JSON。
4. 让编译器和静态检查提供清晰的 AI 修复反馈：Rust 使用严格的 Clippy 与编译检查，Vue 使用 TypeScript strict、`vue-tsc` 和 ESLint。
5. 日志只记录可诊断的结构化元数据，不记录 JWT、SDP、ICE candidate、音视频内容和未经处理的设备隐私信息。

## 目标架构

```text
浏览器（Vue 3 + TypeScript + daisyUI）
        |
        | HTTP JSON / X-Request-ID
        v
Rust 服务（Axum + Tokio）
        |                 \
        | Redis JSON        \ WHIP/WHEP 反向代理
        v                    v
      Redis              Live777
```

迁移期间保留 Go 服务作为可启动的回退实现。Rust 服务先在不同端口通过同一套接口测试，再切换默认端口；切换前不允许 Go 和 Rust 同时向同一个房间写入不同格式的数据。

目标 Rust 工程放在 `rust/woom-server`，负责环境变量、健康检查、JWT、房间/用户/流接口、Redis 访问、Live777 代理、静态文件和结构化日志。最终发布镜像只包含 Rust 二进制和 Vue 构建产物。

目标前端放在 `webapp/src`，按 `domain`、`api`、`features/meeting`、`features/device`、`components` 划分。Vue 只保留一个状态库 Pinia；旧 React 页面在迁移完成前继续作为对照实现。

## HTTP 和 Redis 契约

新增 `contracts/woom-v1.yaml`，定义以下稳定接口：

- `POST /user/` 返回 `streamId` 和 `token`。
- `POST /room/`、`GET /room/{roomId}` 返回 `roomId`、`owner`、`presenter`、`locked`、`streamId` 和 `streams`。
- `POST /room/{roomId}/stream`、`PATCH /room/{roomId}/stream/{streamId}`、`DELETE /room/{roomId}/stream/{streamId}` 管理媒体状态。
- `GET /healthz` 报告进程存活；`GET /readyz` 检查 Redis 和 Live777 配置是否可用。
- `POST /client-events` 接收限制大小的前端诊断事件。
- `/whip/{uuid}` 和 `/whep/{uuid}` 保持对 Live777 的透传行为。

Redis 中每个房间继续使用一个哈希表，字段名继续使用 `admin` 和流 id，字段值改为 JSON，并增加 `__schema` 字段：

```json
{"version":1,"encoding":"json"}
```

Go 的读取顺序为 JSON、旧 Gob；读取旧 Gob 后立即以 JSON 回写。Go 的写入只写 JSON。Rust 只接受 `__schema.version == 1` 的 JSON。迁移期间不删除旧字段，不改变房间 id 生成规则；旧房间在第一次访问时完成惰性迁移。

## 日志和诊断协议

所有后端和前端事件使用同一组字段：`timestamp`、`level`、`service`、`event`、`component`、`operation`、`request_id`、`session_id`、`room_id`、`stream_id`、`browser`、`os`、`duration_ms`、`error` 和 `context`。

- Go 使用 `log/slog`，Rust 使用 `tracing` 加 JSON 输出；两者都做到一行一个 JSON 事件。
- HTTP 中间件生成或透传 `X-Request-ID`，响应头回传该 id。
- 前端日志器在开发控制台输出 JSON，在生产环境只上报 `error` 和 `warn`；`/client-events` 限制请求体为 16 KiB，并丢弃 token、SDP、ICE candidate、设备 label 和媒体数据。
- API 错误统一包含 `status`、`code`、`message`、`requestId`，用户界面显示可读信息，日志保存完整诊断上下文。
- 每个会议会话生成 `session_id`，房间和流相关操作必须带 `room_id`/`stream_id`，使 AI 可以按一次会议导出完整事件序列。

## 测试策略

测试分三层：

1. Go/Rust 单元和集成测试验证 JWT、Redis 编解码、房间状态转换、错误响应和代理配置。
2. API 契约测试验证 Go 与 Rust 对同一 OpenAPI 响应的兼容性。
3. Playwright 验证浏览器真实行为：创建会议、通过链接加入、授权设备、切换设备、两名参会者看到彼此、屏幕共享和离会清理。

GitHub Actions 新增 `e2e.yml`，矩阵为 `ubuntu-latest`、`macos-latest`、`windows-latest` 与 `chrome`、`firefox`、`msedge` 的组合。浏览器安装使用 Playwright 固定版本；Chrome 和 Edge 使用各自浏览器通道，Firefox 使用 Playwright Firefox。媒体测试使用虚拟媒体参数和固定测试视频，另保留真实摄像头与麦克风手工验收清单，因为持续集成无法证明真实硬件驱动质量。

## PR 和发布顺序

1. `PR-0`：依赖、启动、健康检查、API 错误格式和 Demo 验收。
2. `PR-1`：契约文件、Redis JSON 双读单写、JWT 校验加固、结构化日志。
3. `PR-2`：Playwright 基线和三种操作系统、三种浏览器的持续集成矩阵，先覆盖当前 React。
4. `PR-3`：Rust 服务实现并通过 Go/Rust 契约对照测试，保留 Go 回退实现。
5. `PR-4`：Vue + daisyUI 逐页替换 React，所有 Playwright 用例在 Vue 上通过。
6. `PR-5`：Rust/Vue 默认发布镜像、删除旧前端和 Go 运行路径，完成发布检查单。

每个 PR 都必须包含变更说明、验证命令、日志样例和回滚步骤；只有 `PR-2` 之后才允许大规模迁移界面，只有 Rust 契约测试和 E2E 全部通过才允许切换默认服务。

## 一期验收

- 执行 `docker compose up -d redis live777` 后，单条命令可以启动应用并打开会议页面。
- 新建会议、复制链接、第二个浏览器上下文加入成功。
- 两个参会者的音视频状态和离会清理在 Chrome、Firefox、Edge 的持续集成组合中通过。
- macOS、Windows、Linux 的构建和 E2E 任务均通过。
- 任意失败事件都可以用 `session_id`、`room_id`、`stream_id` 和 `request_id` 关联到后端日志。
- Rust 和 Vue 的 PR 可以独立回滚，不影响已经验证的会议流程。
