#![forbid(unsafe_code)]

use axum::{
    Json, Router,
    body::Body,
    extract::{Path, Request, State},
    http::{HeaderMap, HeaderValue, StatusCode, header},
    middleware::{self, Next},
    response::{IntoResponse, Response},
    routing::{get, patch, post},
};
use jsonwebtoken::{Algorithm, DecodingKey, EncodingKey, Header, Validation, decode, encode};
use redis::AsyncCommands;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::{
    collections::HashMap,
    env,
    sync::Arc,
    time::Instant,
    time::{SystemTime, UNIX_EPOCH},
};
use tower_http::services::{ServeDir, ServeFile};
use uuid::Uuid;

const ROOM_SCHEMA_FIELD: &str = "__schema";
const ROOM_SCHEMA_VALUE: &str = r#"{"version":1,"encoding":"json"}"#;
const ADMIN_FIELD: &str = "admin";

#[derive(Clone)]
pub struct AppState {
    pub redis: redis::Client,
    pub secret: Arc<String>,
    pub live777_url: Arc<String>,
    pub live777_token: Arc<String>,
    pub http: reqwest::Client,
}

impl AppState {
    pub fn from_env() -> Result<Self, String> {
        let redis_url = env::var("REDIS_URL").unwrap_or_else(|_| "redis://127.0.0.1:6379/0".into());
        Ok(Self {
            redis: redis::Client::open(redis_url).map_err(|err| err.to_string())?,
            secret: Arc::new(env::var("SECRET").unwrap_or_else(|_| "woom".into())),
            live777_url: Arc::new(
                env::var("LIVE777_URL").unwrap_or_else(|_| "http://127.0.0.1:7777".into()),
            ),
            live777_token: Arc::new(env::var("LIVE777_TOKEN").unwrap_or_default()),
            http: reqwest::Client::new(),
        })
    }
}

pub mod model {
    use super::*;

    #[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
    pub struct User {
        #[serde(rename = "streamId")]
        pub stream_id: String,
        pub token: String,
    }

    #[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
    #[serde(rename_all = "lowercase")]
    pub enum StreamState {
        New,
        Signaled,
        Connecting,
        Connected,
        Disconnected,
        Failed,
        Closed,
    }

    #[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
    pub struct Stream {
        pub name: String,
        pub state: StreamState,
        pub audio: bool,
        pub video: bool,
        pub screen: bool,
    }

    impl Default for Stream {
        fn default() -> Self {
            Self {
                name: String::new(),
                state: StreamState::New,
                audio: false,
                video: false,
                screen: false,
            }
        }
    }

    #[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
    pub struct Room {
        #[serde(rename = "roomId")]
        pub room_id: String,
        pub owner: String,
        #[serde(skip_serializing_if = "String::is_empty")]
        pub presenter: String,
        pub locked: bool,
        #[serde(rename = "streamId", skip_serializing_if = "String::is_empty")]
        pub stream_id: String,
        #[serde(skip_serializing_if = "HashMap::is_empty")]
        pub streams: HashMap<String, Stream>,
    }

    impl Room {
        pub fn new(room_id: String, owner: String) -> Self {
            Self {
                room_id,
                owner: owner.clone(),
                presenter: String::new(),
                locked: false,
                stream_id: owner,
                streams: HashMap::new(),
            }
        }
    }

    #[derive(Debug, Clone, Serialize, Deserialize)]
    pub struct RoomAdmin {
        pub owner: String,
        pub presenter: String,
        pub locked: bool,
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
struct Claims {
    id: String,
    exp: usize,
    iat: usize,
    nbf: usize,
}

#[derive(Debug, Deserialize)]
struct ClientEvent {
    timestamp: Option<String>,
    level: String,
    service: String,
    event: String,
    component: Option<String>,
    operation: Option<String>,
    request_id: Option<String>,
    session_id: Option<String>,
    room_id: Option<String>,
    stream_id: Option<String>,
    browser: Option<String>,
    os: Option<String>,
    duration_ms: Option<i64>,
    #[serde(default)]
    error: HashMap<String, Value>,
    #[serde(default)]
    context: HashMap<String, Value>,
}

#[derive(Debug, Serialize)]
struct ErrorBody {
    error: ErrorDetail,
}

#[derive(Debug, Serialize)]
struct ErrorDetail {
    code: String,
    message: String,
    #[serde(rename = "requestId")]
    request_id: String,
}

#[derive(Debug)]
struct AppError {
    status: StatusCode,
    code: &'static str,
    message: &'static str,
}

impl AppError {
    fn new(status: StatusCode, code: &'static str, message: &'static str) -> Self {
        Self {
            status,
            code,
            message,
        }
    }
}

impl IntoResponse for AppError {
    fn into_response(self) -> Response {
        (
            self.status,
            Json(ErrorBody {
                error: ErrorDetail {
                    code: self.code.into(),
                    message: self.message.into(),
                    request_id: String::new(),
                },
            }),
        )
            .into_response()
    }
}

async fn request_id(mut request: Request, next: Next) -> Response {
    let started = Instant::now();
    let method = request.method().to_string();
    let path = request.uri().path().to_string();
    let id = request
        .headers()
        .get("x-request-id")
        .cloned()
        .unwrap_or_else(|| {
            HeaderValue::from_str(&Uuid::new_v4().to_string()).expect("uuid is a valid header")
        });
    request.headers_mut().insert("x-request-id", id.clone());
    let mut response = next.run(request).await;
    if response.status().is_client_error() || response.status().is_server_error() {
        let status = response.status();
        let headers = response.headers().clone();
        let body = axum::body::to_bytes(response.into_body(), 64 * 1024)
            .await
            .unwrap_or_default();
        let request_id = id.to_str().unwrap_or_default().to_owned();
        let response_body = if let Ok(mut payload) = serde_json::from_slice::<Value>(&body) {
            if let Some(error) = payload.get_mut("error").and_then(Value::as_object_mut) {
                error.insert("requestId".into(), Value::String(request_id));
                serde_json::to_vec(&payload).unwrap_or_else(|_| body.to_vec())
            } else {
                body.to_vec()
            }
        } else {
            body.to_vec()
        };
        let mut restored = Response::new(Body::from(response_body));
        *restored.status_mut() = status;
        for (name, value) in &headers {
            if name != header::CONTENT_LENGTH {
                restored.headers_mut().insert(name, value.clone());
            }
        }
        response = restored;
    }
    response.headers_mut().insert("x-request-id", id);
    println!(
        "{}",
        serde_json::json!({
            "logged_at": SystemTime::now().duration_since(UNIX_EPOCH).map(|value| value.as_millis()).unwrap_or_default(),
            "level": "info",
            "service": "woom-rust",
            "event": "http_request",
            "component": "http",
            "operation": format!("{method} {path}"),
            "request_id": response.headers().get("x-request-id").and_then(|value| value.to_str().ok()).unwrap_or_default(),
            "status": response.status().as_u16(),
            "duration_ms": started.elapsed().as_millis()
        })
    );
    response
}

fn authenticate(headers: &axum::http::HeaderMap, secret: &str) -> Result<String, AppError> {
    let value = headers
        .get(header::AUTHORIZATION)
        .and_then(|value| value.to_str().ok())
        .unwrap_or_default();
    let token = value
        .strip_prefix("Bearer ")
        .ok_or_else(|| AppError::new(StatusCode::UNAUTHORIZED, "unauthorized", "未提供有效身份"))?;
    let validation = Validation::new(Algorithm::HS512);
    decode::<Claims>(
        token,
        &DecodingKey::from_secret(secret.as_bytes()),
        &validation,
    )
    .map(|data| data.claims.id)
    .map_err(|_| AppError::new(StatusCode::UNAUTHORIZED, "unauthorized", "身份验证失败"))
}

fn room_id() -> String {
    let digits: String = (0..9)
        .map(|_| char::from(b'0' + rand::random_range(0..10)))
        .collect();
    format!("{}-{}-{}", &digits[0..3], &digits[3..6], &digits[6..9])
}

fn jwt_for(stream_id: &str, secret: &str) -> Result<String, AppError> {
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map_err(|_| {
            AppError::new(
                StatusCode::INTERNAL_SERVER_ERROR,
                "token_generation_failed",
                "无法创建用户身份",
            )
        })?
        .as_secs() as usize;
    encode(
        &Header::new(Algorithm::HS512),
        &Claims {
            id: stream_id.into(),
            exp: now + 10 * 365 * 24 * 60 * 60,
            iat: now,
            nbf: now,
        },
        &EncodingKey::from_secret(secret.as_bytes()),
    )
    .map_err(|_| {
        AppError::new(
            StatusCode::INTERNAL_SERVER_ERROR,
            "token_generation_failed",
            "无法创建用户身份",
        )
    })
}

async fn redis_hash(state: &AppState, room_id: &str) -> Result<HashMap<String, String>, AppError> {
    let mut connection = state
        .redis
        .get_multiplexed_async_connection()
        .await
        .map_err(|err| {
            eprintln!("Redis connection failed while reading room: {err}");
            AppError::new(
                StatusCode::SERVICE_UNAVAILABLE,
                "dependencies_unavailable",
                "会议依赖服务尚未就绪",
            )
        })?;
    connection.hgetall(room_id).await.map_err(|_| {
        AppError::new(
            StatusCode::INTERNAL_SERVER_ERROR,
            "room_read_failed",
            "无法读取会议",
        )
    })
}

async fn write_hash(
    state: &AppState,
    room_id: &str,
    values: &[(&str, String)],
) -> Result<(), AppError> {
    let mut connection = state
        .redis
        .get_multiplexed_async_connection()
        .await
        .map_err(|err| {
            eprintln!("Redis connection failed while writing room: {err}");
            AppError::new(
                StatusCode::SERVICE_UNAVAILABLE,
                "dependencies_unavailable",
                "会议依赖服务尚未就绪",
            )
        })?;
    connection
        .hset_multiple(room_id, values)
        .await
        .map_err(|_| {
            AppError::new(
                StatusCode::INTERNAL_SERVER_ERROR,
                "room_persistence_failed",
                "无法保存会议状态",
            )
        })
}

async fn read_room(state: &AppState, room_id: &str) -> Result<model::Room, AppError> {
    let values = redis_hash(state, room_id).await?;
    let admin: model::RoomAdmin = serde_json::from_str(
        values
            .get(ADMIN_FIELD)
            .ok_or_else(|| AppError::new(StatusCode::NOT_FOUND, "room_not_found", "会议不存在"))?,
    )
    .map_err(|_| {
        AppError::new(
            StatusCode::INTERNAL_SERVER_ERROR,
            "room_read_failed",
            "无法读取会议",
        )
    })?;
    let mut room = model::Room {
        room_id: room_id.into(),
        owner: admin.owner,
        presenter: admin.presenter,
        locked: admin.locked,
        stream_id: String::new(),
        streams: HashMap::new(),
    };
    for (key, value) in values {
        if key == ADMIN_FIELD || key == ROOM_SCHEMA_FIELD {
            continue;
        }
        let stream = serde_json::from_str::<model::Stream>(&value).map_err(|_| {
            AppError::new(
                StatusCode::INTERNAL_SERVER_ERROR,
                "room_read_failed",
                "无法读取会议",
            )
        })?;
        room.streams.insert(key, stream);
    }
    Ok(room)
}

async fn healthz() -> Json<Value> {
    Json(serde_json::json!({"status": "ok"}))
}

async fn readyz(State(state): State<AppState>) -> Result<Json<Value>, AppError> {
    let mut connection = state
        .redis
        .get_multiplexed_async_connection()
        .await
        .map_err(|_| {
            AppError::new(
                StatusCode::SERVICE_UNAVAILABLE,
                "dependencies_unavailable",
                "会议依赖服务尚未就绪",
            )
        })?;
    redis::cmd("PING")
        .query_async::<String>(&mut connection)
        .await
        .map_err(|_| {
            AppError::new(
                StatusCode::SERVICE_UNAVAILABLE,
                "dependencies_unavailable",
                "会议依赖服务尚未就绪",
            )
        })?;
    Ok(Json(serde_json::json!({"status": "ready"})))
}

async fn create_user(State(state): State<AppState>) -> Result<Json<model::User>, AppError> {
    let stream_id = Uuid::new_v4().to_string();
    Ok(Json(model::User {
        token: jwt_for(&stream_id, &state.secret)?,
        stream_id,
    }))
}

async fn create_room(
    State(state): State<AppState>,
    request: Request,
) -> Result<Json<model::Room>, AppError> {
    let owner = authenticate(request.headers(), &state.secret)?;
    let room = model::Room::new(room_id(), owner);
    let admin = serde_json::json!({"owner": room.owner, "presenter": room.presenter, "locked": room.locked}).to_string();
    write_hash(
        &state,
        &room.room_id,
        &[
            (ADMIN_FIELD, admin),
            (ROOM_SCHEMA_FIELD, ROOM_SCHEMA_VALUE.into()),
        ],
    )
    .await?;
    Ok(Json(room))
}

async fn show_room(
    State(state): State<AppState>,
    Path(room_id): Path<String>,
    request: Request,
) -> Result<Json<model::Room>, AppError> {
    authenticate(request.headers(), &state.secret)?;
    Ok(Json(read_room(&state, &room_id).await?))
}

async fn create_stream(
    State(state): State<AppState>,
    Path(room_id): Path<String>,
    request: Request,
) -> Result<Json<model::Room>, AppError> {
    authenticate(request.headers(), &state.secret)?;
    let mut room = read_room(&state, &room_id).await?;
    let stream_id = Uuid::new_v4().to_string();
    write_hash(
        &state,
        &room_id,
        &[(
            stream_id.as_str(),
            serde_json::to_string(&model::Stream::default()).unwrap(),
        )],
    )
    .await?;
    room.stream_id = stream_id;
    Ok(Json(room))
}

async fn update_stream(
    State(state): State<AppState>,
    Path((room_id, stream_id)): Path<(String, String)>,
    headers: HeaderMap,
    Json(stream): Json<model::Stream>,
) -> Result<Json<model::Room>, AppError> {
    authenticate(&headers, &state.secret)?;
    let mut room = read_room(&state, &room_id).await?;
    let value = serde_json::to_string(&stream)
        .map_err(|_| AppError::new(StatusCode::BAD_REQUEST, "invalid_stream", "会议流数据无效"))?;
    write_hash(&state, &room_id, &[(stream_id.as_str(), value)]).await?;
    room.streams.insert(stream_id, stream);
    Ok(Json(room))
}

async fn delete_stream(
    State(state): State<AppState>,
    Path((room_id, stream_id)): Path<(String, String)>,
    request: Request,
) -> Result<StatusCode, AppError> {
    authenticate(request.headers(), &state.secret)?;
    let mut connection = state
        .redis
        .get_multiplexed_async_connection()
        .await
        .map_err(|_| {
            AppError::new(
                StatusCode::SERVICE_UNAVAILABLE,
                "dependencies_unavailable",
                "会议依赖服务尚未就绪",
            )
        })?;
    let _: usize = connection.hdel(room_id, stream_id).await.map_err(|_| {
        AppError::new(
            StatusCode::INTERNAL_SERVER_ERROR,
            "room_persistence_failed",
            "无法保存会议状态",
        )
    })?;
    Ok(StatusCode::NO_CONTENT)
}

fn sanitize_client_fields(
    fields: &HashMap<String, Value>,
    allowed: &[&str],
) -> serde_json::Map<String, Value> {
    fields
        .iter()
        .filter(|(key, value)| {
            allowed.contains(&key.as_str())
                && (value.is_string() || value.is_boolean() || value.is_number())
        })
        .map(|(key, value)| (key.clone(), value.clone()))
        .collect()
}

async fn client_events(Json(event): Json<ClientEvent>) -> Result<StatusCode, AppError> {
    if !matches!(event.level.as_str(), "debug" | "info" | "warn" | "error") {
        return Err(AppError::new(
            StatusCode::BAD_REQUEST,
            "invalid_request",
            "诊断事件级别不正确",
        ));
    }
    if event.service.trim().is_empty() || event.event.trim().is_empty() {
        return Err(AppError::new(
            StatusCode::BAD_REQUEST,
            "invalid_request",
            "诊断事件缺少服务或事件名称",
        ));
    }
    let payload = serde_json::json!({
        "logged_at": event.timestamp.unwrap_or_else(|| SystemTime::now().duration_since(UNIX_EPOCH).map(|value| value.as_millis().to_string()).unwrap_or_default()),
        "level": event.level,
        "service": event.service,
        "event": event.event,
        "component": event.component,
        "operation": event.operation,
        "request_id": event.request_id,
        "session_id": event.session_id,
        "room_id": event.room_id,
        "stream_id": event.stream_id,
        "browser": event.browser,
        "os": event.os,
        "duration_ms": event.duration_ms,
        "error": sanitize_client_fields(&event.error, &["code", "kind", "message", "name", "status"]),
        "context": sanitize_client_fields(&event.context, &["browser", "connection_state", "device_kind", "feature", "http_status", "os", "permission_state", "retry_count"])
    });
    println!("{}", payload);
    Ok(StatusCode::ACCEPTED)
}

async fn proxy_media(
    State(state): State<AppState>,
    Path(uuid): Path<String>,
    request: Request,
    kind: &'static str,
) -> Result<Response, AppError> {
    let method = request.method().clone();
    let request_headers = request.headers().clone();
    let body = axum::body::to_bytes(request.into_body(), 16 * 1024 * 1024)
        .await
        .map_err(|_| {
            AppError::new(
                StatusCode::BAD_REQUEST,
                "media_request_invalid",
                "媒体请求无效",
            )
        })?;
    let mut url = state.live777_url.trim_end_matches('/').to_string();
    url.push_str(&format!("/{kind}/{uuid}"));
    let mut builder = state.http.request(method, url).body(body);
    for name in [header::CONTENT_TYPE, header::ACCEPT] {
        if let Some(value) = request_headers.get(&name) {
            builder = builder.header(name.as_str(), value.as_bytes());
        }
    }
    if !state.live777_token.is_empty() {
        builder = builder.bearer_auth(state.live777_token.as_str());
    }
    let response = builder.send().await.map_err(|_| {
        AppError::new(
            StatusCode::BAD_GATEWAY,
            "media_proxy_failed",
            "媒体服务不可用",
        )
    })?;
    let status =
        StatusCode::from_u16(response.status().as_u16()).unwrap_or(StatusCode::BAD_GATEWAY);
    let headers = response.headers().clone();
    let body = response.bytes().await.map_err(|_| {
        AppError::new(
            StatusCode::BAD_GATEWAY,
            "media_proxy_failed",
            "媒体服务不可用",
        )
    })?;
    let mut result = Response::builder()
        .status(status)
        .body(Body::from(body))
        .unwrap();
    for (name, value) in &headers {
        result.headers_mut().insert(name, value.clone());
    }
    Ok(result)
}

async fn whip(
    State(state): State<AppState>,
    Path(uuid): Path<String>,
    request: Request,
) -> Result<Response, AppError> {
    proxy_media(State(state), Path(uuid), request, "whip").await
}

async fn whep(
    State(state): State<AppState>,
    Path(uuid): Path<String>,
    request: Request,
) -> Result<Response, AppError> {
    proxy_media(State(state), Path(uuid), request, "whep").await
}

pub mod app {
    use super::*;

    pub fn router(state: AppState) -> Router {
        Router::new()
            .route("/healthz", get(super::healthz))
            .route("/readyz", get(super::readyz))
            .route("/client-events", post(super::client_events))
            .route("/user/", post(super::create_user))
            .route("/room/", post(super::create_room))
            .route("/room/{room_id}", get(super::show_room))
            .route("/room/{room_id}/stream", post(super::create_stream))
            .route(
                "/room/{room_id}/stream/{stream_id}",
                patch(super::update_stream).delete(super::delete_stream),
            )
            .route("/whip/{uuid}", post(super::whip))
            .route("/whep/{uuid}", post(super::whep))
            .layer(middleware::from_fn(super::request_id))
            .fallback_service(
                ServeDir::new("static/dist").fallback(ServeFile::new("static/dist/index.html")),
            )
            .with_state(state)
    }

    pub fn router_for_test() -> Router {
        router(AppState {
            redis: redis::Client::open("redis://127.0.0.1:6379/0")
                .expect("test redis URL is valid"),
            secret: Arc::new("woom".into()),
            live777_url: Arc::new("http://127.0.0.1:7777".into()),
            live777_token: Arc::new(String::new()),
            http: reqwest::Client::new(),
        })
    }
}
