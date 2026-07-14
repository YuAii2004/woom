use axum::{body::Body, http::Request};
use serde_json::json;
use tower::ServiceExt;
use woom_server::{
    app,
    model::{Room, Stream, StreamState, User},
};

#[test]
fn model_json_matches_the_v1_contract() {
    let user = User {
        stream_id: "stream-1".into(),
        token: "token".into(),
    };
    assert_eq!(
        serde_json::to_value(user).unwrap(),
        json!({
            "streamId": "stream-1",
            "token": "token"
        })
    );

    let stream = Stream {
        name: "Alice".into(),
        state: StreamState::Connected,
        audio: true,
        video: false,
        screen: false,
    };
    assert_eq!(
        serde_json::to_value(stream).unwrap(),
        json!({
            "name": "Alice",
            "state": "connected",
            "audio": true,
            "video": false,
            "screen": false
        })
    );

    let room = Room::new("123-456-789".into(), "owner".into());
    assert_eq!(
        serde_json::to_value(room).unwrap(),
        json!({
            "roomId": "123-456-789",
            "owner": "owner",
            "locked": false,
            "streamId": "owner"
        })
    );
}

#[tokio::test]
async fn health_endpoint_returns_contract_payload() {
    let service = app::router_for_test();
    let response = service
        .oneshot(
            Request::builder()
                .uri("/healthz")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();

    assert_eq!(response.status(), 200);
    let body = axum::body::to_bytes(response.into_body(), 1024)
        .await
        .unwrap();
    assert_eq!(body.as_ref(), br#"{"status":"ok"}"#);
}

#[tokio::test]
async fn unauthorized_error_contains_request_id() {
    let service = app::router_for_test();
    let response = service
        .oneshot(
            Request::builder()
                .method("POST")
                .uri("/room/")
                .header("x-request-id", "rust-test-request")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();

    assert_eq!(response.status(), 401);
    let body = axum::body::to_bytes(response.into_body(), 1024)
        .await
        .unwrap();
    let payload: serde_json::Value = serde_json::from_slice(&body).unwrap();
    assert_eq!(payload["error"]["requestId"], "rust-test-request");
}

#[tokio::test]
async fn client_events_accept_safe_diagnostics_and_reject_bad_levels() {
    let service = app::router_for_test();
    let accepted = service
        .oneshot(
            Request::builder()
                .method("POST")
                .uri("/client-events")
                .header("content-type", "application/json")
                .body(Body::from(
                    r#"{"level":"error","service":"woom-web","event":"api_failed","error":{"message":"失败","token":"不要记录"}}"#,
                ))
                .unwrap(),
        )
        .await
        .unwrap();
    assert_eq!(accepted.status(), 202);

    let service = app::router_for_test();
    let rejected = service
        .oneshot(
            Request::builder()
                .method("POST")
                .uri("/client-events")
                .header("content-type", "application/json")
                .header("x-request-id", "bad-event-request")
                .body(Body::from(
                    r#"{"level":"trace","service":"woom-web","event":"bad"}"#,
                ))
                .unwrap(),
        )
        .await
        .unwrap();
    assert_eq!(rejected.status(), 400);
    let body = axum::body::to_bytes(rejected.into_body(), 1024)
        .await
        .unwrap();
    let payload: serde_json::Value = serde_json::from_slice(&body).unwrap();
    assert_eq!(payload["error"]["requestId"], "bad-event-request");
}
