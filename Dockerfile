FROM node:20-alpine AS frontend-builder

WORKDIR /app

COPY package.json package-lock.json ./

RUN npm ci

COPY . .

RUN npm run build

FROM rust:1.91-alpine AS rust-builder

WORKDIR /app

RUN apk add --no-cache build-base

COPY rust/Cargo.toml rust/Cargo.lock rust/
COPY rust/src rust/src

RUN cargo build --release --manifest-path rust/Cargo.toml

FROM alpine:3.21 AS runtime

WORKDIR /app

COPY --from=rust-builder /app/rust/target/release/woom-server /usr/bin/woom-server
COPY --from=frontend-builder /app/static/dist /app/static/dist

EXPOSE 4000/tcp

ENTRYPOINT ["/usr/bin/woom-server"]
