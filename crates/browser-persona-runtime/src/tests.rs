use std::{fs, net::TcpListener, sync::mpsc, thread, time::Duration};

use browser_persona::{DisplayMetrics, PersonaRuntimePlan};
use serde_json::{Value, json};
use tempfile::TempDir;
use tungstenite::{Message, accept};

use super::{
    CdpPersonaRuntime, PersonaRuntime, PersonaRuntimeError, PersonaRuntimeNavigation,
    PersonaRuntimeRequest, parse_endpoint,
};

#[test]
fn rejects_a_non_loopback_or_malformed_devtools_endpoint() {
    assert!(matches!(
        parse_endpoint("9222\nws://remote.example/devtools/browser/x"),
        Err(PersonaRuntimeError::InvalidEndpoint)
    ));
    assert!(matches!(
        parse_endpoint("0\n/devtools/browser/x"),
        Err(PersonaRuntimeError::InvalidEndpoint)
    ));
}

#[test]
fn native_persona_does_not_enable_remote_debugging() {
    let root = TempDir::new().unwrap();
    let runtime = CdpPersonaRuntime::new();
    let request = PersonaRuntimeRequest {
        profile_root: root.path(),
        plan: PersonaRuntimePlan::default(),
        requested_startup_url: "https://seller.example",
        navigation: PersonaRuntimeNavigation::Navigate,
    };

    let launch = runtime.prepare(&request).unwrap();
    assert!(launch.chrome_arguments().is_empty());
    assert_eq!(launch.browser_startup_url(), "https://seller.example");
    assert!(runtime.attach(&request).unwrap().is_none());
}

#[test]
fn applies_observes_and_replays_the_managed_runtime_plan() {
    let root = TempDir::new().unwrap();
    fs::write(root.path().join("DevToolsActivePort"), "stale").unwrap();
    let runtime = CdpPersonaRuntime::with_ready_timeout(Duration::from_secs(1));
    let plan = PersonaRuntimePlan {
        timezone_id: Some("Europe/Berlin".to_owned()),
        display_metrics: Some(DisplayMetrics::desktop(1366, 768, 1920, 1080, 125)),
        ..PersonaRuntimePlan::default()
    };
    let request = PersonaRuntimeRequest {
        profile_root: root.path(),
        plan,
        requested_startup_url: "https://seller.example/dashboard",
        navigation: PersonaRuntimeNavigation::Navigate,
    };

    let launch = runtime.prepare(&request).unwrap();
    assert_eq!(launch.chrome_arguments(), ["--remote-debugging-port=0"]);
    assert_eq!(launch.browser_startup_url(), "about:blank");
    assert!(!root.path().join("DevToolsActivePort").exists());

    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let port = listener.local_addr().unwrap().port();
    fs::write(
        root.path().join("DevToolsActivePort"),
        format!("{port}\n/devtools/browser/runtime-test\n"),
    )
    .unwrap();
    let (commands_tx, commands_rx) = mpsc::channel();
    let server = thread::spawn(move || {
        let (stream, _) = listener.accept().unwrap();
        let mut socket = accept(stream).unwrap();
        for command_index in 0..12 {
            let Message::Text(text) = socket.read().unwrap() else {
                panic!("expected text command");
            };
            let command = serde_json::from_str::<Value>(&text).unwrap();
            commands_tx.send(command.clone()).unwrap();
            let id = command["id"].as_u64().unwrap();
            let result = match command["method"].as_str().unwrap() {
                "Target.getTargets" => json!({
                    "targetInfos": [{
                        "targetId": "page-1",
                        "type": "page",
                        "title": "",
                        "url": "about:blank",
                        "attached": false,
                        "canAccessOpener": false
                    }]
                }),
                "Target.attachToTarget" => json!({ "sessionId": "session-1" }),
                "Emulation.setTimezoneOverride" => json!({}),
                "Emulation.setDeviceMetricsOverride" => json!({}),
                "Target.setAutoAttach" => json!({}),
                "Target.setDiscoverTargets" => json!({}),
                "Runtime.runIfWaitingForDebugger" => json!({}),
                "Runtime.evaluate"
                    if command["params"]["expression"]
                        .as_str()
                        .is_some_and(|expression| expression.contains("Intl.DateTimeFormat")) =>
                {
                    json!({
                        "result": { "type": "string", "value": "Europe/Berlin" }
                    })
                }
                "Runtime.evaluate" => json!({
                    "result": {
                        "type": "object",
                        "value": {
                            "viewportWidth": 1366,
                            "viewportHeight": 768,
                            "screenWidth": 1920,
                            "screenHeight": 1080,
                            "deviceScaleFactor": 1.25
                        }
                    }
                }),
                "Page.navigate" => json!({ "frameId": "frame-1" }),
                method => panic!("unexpected method: {method}"),
            };
            socket
                .send(Message::Text(
                    json!({ "id": id, "result": result }).to_string().into(),
                ))
                .unwrap();
            if command_index == 5 {
                socket
                    .send(Message::Text(
                        json!({
                            "method": "Target.attachedToTarget",
                            "params": {
                                "sessionId": "session-2",
                                "targetInfo": {
                                    "targetId": "page-2",
                                    "type": "page",
                                    "title": "",
                                    "url": "about:blank",
                                    "attached": true,
                                    "canAccessOpener": false
                                },
                                "waitingForDebugger": true
                            }
                        })
                        .to_string()
                        .into(),
                    ))
                    .unwrap();
            }
        }
        while let Ok(message) = socket.read() {
            if matches!(message, Message::Close(_)) {
                return;
            }
        }
    });

    let mut active = runtime.attach(&request).unwrap().unwrap();
    assert_eq!(active.observation().fields.len(), 4);
    assert_eq!(active.observation().fields[0].observed, "Europe/Berlin");
    assert_eq!(active.observation().fields[1].observed, "1366x768");
    assert_eq!(active.observation().fields[2].observed, "1920x1080");
    assert_eq!(active.observation().fields[3].observed, "1.25");
    let commands = (0..12)
        .map(|_| commands_rx.recv_timeout(Duration::from_secs(1)).unwrap())
        .collect::<Vec<_>>();
    assert_eq!(
        commands[2]["params"]["timezoneId"],
        Value::String("Europe/Berlin".to_owned())
    );
    assert_eq!(
        commands[7]["params"]["url"],
        Value::String("https://seller.example/dashboard".to_owned())
    );
    assert_eq!(commands[8]["method"], "Emulation.setTimezoneOverride");
    assert_eq!(commands[8]["sessionId"], "session-2");
    assert_eq!(commands[9]["method"], "Emulation.setDeviceMetricsOverride");
    assert_eq!(commands[9]["sessionId"], "session-2");
    assert_eq!(commands[10]["method"], "Target.setAutoAttach");
    assert_eq!(commands[10]["sessionId"], "session-2");
    assert_eq!(commands[11]["method"], "Runtime.runIfWaitingForDebugger");
    assert_eq!(commands[11]["sessionId"], "session-2");

    active.shutdown().unwrap();
    server.join().unwrap();
}
