<h1 align="center">
  <img src="./vueapp/public/logo.svg" alt="WOOM" width="200">
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

WOOM is a lightweight, self-hostable meeting service powered by [Live777](https://github.com/binbat/live777) for media transport.

## Current Status

The default application now uses a Rust backend and a Vue 3, TypeScript, and daisyUI frontend. Rust is the only backend implementation, and the frontend has been unified on Vue.

The first release target is a 2 to 10 person meeting flow where participants can join by link, use camera and microphone controls, switch devices, share the screen, and leave cleanly.

## Requirements

- Node.js 20 or newer
- Rust stable
- Docker Desktop or another Docker environment with Docker Compose support

## Run the Demo Locally

### 1. Start the dependencies

From the project root:

```bash
docker compose up -d redis live777
```

Redis listens on `localhost:6379` by default. Live777 listens on `localhost:7777` by default.

### 2. Install frontend dependencies

```bash
npm ci
```

### 3. Start the Rust backend

In one terminal:

```bash
cargo run --manifest-path rust/Cargo.toml
```

The Rust backend listens on `http://localhost:4000` by default.

### 4. Start the Vue frontend dev server

In another terminal:

```bash
npm run dev
```

Open the URL printed by the dev server, usually `http://localhost:5173`.

## Demo Acceptance Checklist

1. Create a meeting in the first browser window.
2. Copy the meeting id or meeting link.
3. Join the meeting from a second browser window.
4. Check camera, microphone, device switching, and screen sharing.
5. Leave from one window and confirm the other window sees the participant leave.
6. After closing the pages, confirm Redis does not retain stale departed streams.

## Production Build

Build the Vue frontend and compile the Rust service:

```bash
make
```

You can also run the steps separately:

```bash
npm run build
cargo build --release --manifest-path rust/Cargo.toml
```

The default entrypoint is Rust, Vue, and daisyUI. The Vue dev server uses port 5173 by default. `npm run dev:vue` and `npm run build:vue` are explicit aliases for the same Vue workflow.

Build the Docker image:

```bash
docker build -t woom .
```

Compose starts the Rust/Vue app, Redis, and Live777 together:

```bash
docker compose up -d
```

To run the image on the Compose-created `woom` network:

```bash
docker run --rm --name woom --network woom \
  -p 4000:4000 \
  -e REDIS_URL=redis://redis:6379/0 \
  -e LIVE777_URL=http://live777:7777 \
  woom
```

## Common Verification Commands

```bash
npm run test:unit
npm run lint
npm run build
docker compose config
cargo fmt --manifest-path rust/Cargo.toml -- --check
cargo test --manifest-path rust/Cargo.toml
```

Playwright regression tests cover Chromium, Firefox, Chrome, and Edge. Run all local projects with:

```bash
npm run test:e2e
```

Run one browser project:

```bash
npx playwright test --project=chromium
npx playwright test --project=firefox
```

GitHub Actions runs Chrome, Firefox, and Edge on Windows, macOS, and Linux, and uploads test reports on failure.
