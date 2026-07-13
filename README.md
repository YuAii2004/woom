<h1 align="center">
  <img src="./webapp/public/logo.svg" alt="WOOM" width="200">
  <br>WOOM<br>
</h1>

<div align="center">
    <a href="https://discord.gg/mtSpDNwCAz"><img src="https://img.shields.io/badge/-Discord-424549?style=social&logo=discord" height=25></a>
    &nbsp;
    <a href="https://t.me/binbatlib"><img src="https://img.shields.io/badge/-Telegram-red?style=social&logo=telegram" height=25></a>
    &nbsp;
    <a href="https://twitter.com/binbatlab"><img src="https://img.shields.io/badge/-Twitter-red?style=social&logo=x" height=25></a>
</div>

---

WOOM 是一个轻量、可自托管的会议服务，媒体能力使用 [Live777](https://github.com/binbat/live777) 提供。

## 当前状态

当前版本使用 Go 服务端和 React 前端。重构计划是逐步迁移到 Rust、Vue 3、TypeScript 和 daisyUI；迁移期间会保留可回退实现，确保会议流程持续可用。

一期目标是支持 2～10 人通过链接加入会议，并完成摄像头、麦克风、设备切换、屏幕共享和离会。

## 环境要求

- Node.js 20 或更高版本
- Go 1.21 或更高版本
- Docker Desktop 或其他支持 Docker Compose 的 Docker 环境

## 本地运行 Demo

### 1. 启动依赖服务

在项目根目录执行：

```bash
docker compose up -d redis live777
```

Redis 默认监听 `localhost:6379`，Live777 默认监听 `localhost:7777`。

### 2. 安装前端依赖

```bash
npm ci
```

### 3. 启动 Go 服务端

在一个终端执行：

```bash
go run .
```

服务端默认监听 `http://localhost:4000`。

### 4. 启动前端开发服务

在另一个终端执行：

```bash
npm run dev
```

打开终端输出的地址，通常是 `http://localhost:5173`。

## Demo 验收

1. 在第一个浏览器窗口新建会议。
2. 复制会议 id 或会议链接。
3. 在第二个浏览器窗口加入会议。
4. 分别检查摄像头、麦克风、设备切换和屏幕共享。
5. 从一个窗口离会，再确认另一个窗口可以看到参会者离开。
6. 关闭页面后，确认 Redis 中不会长期残留已经离开的流。

## 发布构建

构建前端并编译 Go 服务：

```bash
make
```

也可以分开执行：

```bash
npm run build
go build -tags release -trimpath -o woom
```

构建 Docker 镜像：

```bash
docker build -t woom .
```

如果使用 Compose 创建的 `woom` 网络运行镜像：

```bash
docker run --rm --name woom --network woom \
  -p 4000:4000 \
  -e REDIS_URL=redis://redis:6379/0 \
  -e LIVE777_URL=http://live777:7777 \
  woom
```

## 常用检查命令

```bash
go test ./...
npm run lint
npm run build
docker compose config
```

Playwright 回归测试覆盖 Chromium、Firefox、Chrome 和 Edge。使用以下命令运行全部本地项目：

```bash
npm run test:e2e
```

只运行某一个浏览器项目：

```bash
npx playwright test --project=chromium
npx playwright test --project=firefox
```

GitHub Actions 会在 Windows、macOS、Linux 上分别运行 Chrome、Firefox 和 Edge，并在失败时上传测试报告。

## 相关文档

- [重构设计](./docs/superpowers/specs/2026-07-13-woom-rebuild-design.md)
- [重构实施计划](./docs/superpowers/plans/2026-07-13-woom-rebuild-plan.md)
