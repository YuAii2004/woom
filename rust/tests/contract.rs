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
