use std::{env, net::SocketAddr};
use woom_server::{AppState, app};

#[tokio::main]
async fn main() {
    let state = AppState::from_env().expect("Rust 服务配置无效");
    let port = env::var("PORT").unwrap_or_else(|_| "4000".into());
    let address: SocketAddr = format!("0.0.0.0:{port}").parse().expect("服务端口无效");
    let listener = tokio::net::TcpListener::bind(address)
        .await
        .expect("无法监听 Rust 服务端口");
    println!("Rust 服务已启动: http://{address}");
    axum::serve(listener, app::router(state))
        .await
        .expect("Rust 服务异常退出");
}
