use std::{
    net::TcpStream,
    path::Path,
    thread,
    time::{Duration, Instant},
};

use browser_core::{BrowserHost, BrowserInstall, BrowserSession, IdentityId, LaunchPlan};
use browser_persona::PersonaConfig;
use browser_persona_runtime::CANVAS_K1_OBSERVATION_SCRIPT;
use browser_platform::SystemBrowserHost;
use serde_json::{Value, json};
use tempfile::TempDir;
use tungstenite::{Message, WebSocket, stream::MaybeTlsStream};

const READY_TIMEOUT: Duration = Duration::from_secs(10);

#[test]
#[ignore = "requires packaged RealBrowser product Chromium and launches two native processes"]
fn two_realbrowser_identities_have_stable_distinct_canvas_across_contexts() {
    let root = TempDir::new().unwrap();
    let identity_a = IdentityId::parse("00000000-0000-0000-0000-000000000171").unwrap();
    let identity_b = IdentityId::parse("00000000-0000-0000-0000-000000000172").unwrap();
    let profile_a = root.path().join("profile-a");
    let profile_b = root.path().join("profile-b");
    let host = SystemBrowserHost::new();
    let browser = host.detect_product_kernel().unwrap();

    let session_a = start_identity(&host, &browser, identity_a, &profile_a);
    let first_a = observe_canvas(&profile_a);
    let session_b = start_identity(&host, &browser, identity_b, &profile_b);
    let first_b = observe_canvas(&profile_b);

    assert!(host.is_running(identity_a));
    assert!(host.is_running(identity_b));
    assert_product_process(&session_a, &browser);
    assert_product_process(&session_b, &browser);
    assert_product_processes_for_profiles(&browser, &[&profile_a, &profile_b]);
    assert_canvas_matrix(&first_a);
    assert_canvas_matrix(&first_b);
    assert_ne!(first_a["getImageDataHash"], first_b["getImageDataHash"]);
    assert_ne!(first_a["dataUrlHash"], first_b["dataUrlHash"]);
    assert_ne!(first_a["blobHash"], first_b["blobHash"]);

    host.stop(identity_a).unwrap();
    host.stop(identity_b).unwrap();
    let restarted_a = start_identity(&host, &browser, identity_a, &profile_a);
    let second_a = observe_canvas(&profile_a);
    assert_product_process(&restarted_a, &browser);
    assert_product_processes_for_profiles(&browser, &[&profile_a]);
    assert_canvas_matrix(&second_a);
    assert_eq!(first_a["getImageDataHash"], second_a["getImageDataHash"]);
    assert_eq!(first_a["dataUrlHash"], second_a["dataUrlHash"]);
    assert_eq!(first_a["blobHash"], second_a["blobHash"]);
    host.stop(identity_a).unwrap();
}

fn start_identity(
    host: &SystemBrowserHost,
    browser: &BrowserInstall,
    identity_id: IdentityId,
    profile_root: &Path,
) -> BrowserSession {
    std::fs::create_dir_all(profile_root).unwrap();
    let _ = std::fs::remove_file(profile_root.join("DevToolsActivePort"));
    let persona_path = profile_root.join("persona.json");
    let persona = PersonaConfig::native(identity_id.persona_seed())
        .compile_kernel_persona(&identity_id.to_string(), browser.engine_major);
    std::fs::write(&persona_path, serde_json::to_vec_pretty(&persona).unwrap()).unwrap();
    host.start(
        &LaunchPlan {
            identity_id,
            executable: browser.executable.clone(),
            profile_root: profile_root.to_path_buf(),
            startup_url: "about:blank".to_owned(),
            browser_version: browser.version.clone(),
            arguments: vec![
                format!("--realbrowser-persona-file={}", persona_path.display()),
                "--remote-debugging-port=0".to_owned(),
                "--headless=new".to_owned(),
                "--disable-gpu".to_owned(),
            ],
            environment: Vec::new(),
        },
        1_700_000_000_000,
    )
    .unwrap()
}

fn observe_canvas(profile_root: &Path) -> Value {
    let endpoint = wait_for_browser_endpoint(profile_root);
    let (mut socket, _) = tungstenite::connect(endpoint).unwrap();
    if let MaybeTlsStream::Plain(stream) = socket.get_mut() {
        stream
            .set_read_timeout(Some(Duration::from_secs(5)))
            .unwrap();
    }
    let targets = command(&mut socket, 1, "Target.getTargets", json!({}), None);
    let target_id = targets["targetInfos"]
        .as_array()
        .unwrap()
        .iter()
        .find(|target| target["type"] == "page")
        .and_then(|target| target["targetId"].as_str())
        .unwrap();
    let attached = command(
        &mut socket,
        2,
        "Target.attachToTarget",
        json!({ "targetId": target_id, "flatten": true }),
        None,
    );
    let session_id = attached["sessionId"].as_str().unwrap();
    let evaluated = command(
        &mut socket,
        3,
        "Runtime.evaluate",
        json!({
            "expression": CANVAS_K1_OBSERVATION_SCRIPT,
            "awaitPromise": true,
            "returnByValue": true
        }),
        Some(session_id),
    );
    assert!(evaluated.get("exceptionDetails").is_none(), "{evaluated}");
    evaluated["result"]["value"].clone()
}

fn assert_canvas_matrix(observed: &Value) {
    assert_eq!(
        observed["status"],
        "seeded_idempotent_copy_all_contexts_native"
    );
    assert_eq!(observed["nativeFunctions"], true);
    assert_eq!(observed["getImageDataHash"], observed["iframeHash"]);
    assert_eq!(observed["getImageDataHash"], observed["workerHash"]);
}

fn wait_for_browser_endpoint(profile_root: &Path) -> String {
    let path = profile_root.join("DevToolsActivePort");
    let deadline = Instant::now() + READY_TIMEOUT;
    loop {
        if let Ok(contents) = std::fs::read_to_string(&path) {
            let mut lines = contents.lines();
            if let (Some(port), Some(endpoint_path)) = (lines.next(), lines.next()) {
                return format!("ws://127.0.0.1:{port}{endpoint_path}");
            }
        }
        assert!(
            Instant::now() < deadline,
            "RealBrowser CDP endpoint unavailable"
        );
        thread::sleep(Duration::from_millis(25));
    }
}

fn command(
    socket: &mut WebSocket<MaybeTlsStream<TcpStream>>,
    id: u64,
    method: &str,
    params: Value,
    session_id: Option<&str>,
) -> Value {
    let mut payload = json!({ "id": id, "method": method, "params": params });
    if let Some(session_id) = session_id {
        payload["sessionId"] = Value::String(session_id.to_owned());
    }
    socket
        .send(Message::Text(payload.to_string().into()))
        .unwrap();
    loop {
        let Message::Text(text) = socket.read().unwrap() else {
            continue;
        };
        let response = serde_json::from_str::<Value>(&text).unwrap();
        if response["id"].as_u64() == Some(id) {
            assert!(response.get("error").is_none(), "{response}");
            return response["result"].clone();
        }
    }
}

#[cfg(target_os = "macos")]
fn assert_product_process(session: &BrowserSession, browser: &BrowserInstall) {
    let output = std::process::Command::new("ps")
        .args(["-o", "command=", "-p", session.pid.to_string().as_str()])
        .output()
        .unwrap();
    let command = String::from_utf8_lossy(&output.stdout);
    assert!(command.contains(browser.executable.to_string_lossy().as_ref()));
    assert!(!command.contains("Google Chrome"));
}

#[cfg(target_os = "macos")]
fn assert_product_processes_for_profiles(browser: &BrowserInstall, profile_roots: &[&Path]) {
    let bundle_root = browser
        .executable
        .ancestors()
        .find(|path| path.extension().is_some_and(|extension| extension == "app"))
        .expect("product executable must be inside RealBrowser.app");
    let output = std::process::Command::new("ps")
        .args(["-axo", "command="])
        .output()
        .unwrap();
    let process_table = String::from_utf8_lossy(&output.stdout);

    for profile_root in profile_roots {
        let profile = profile_root.to_string_lossy();
        let related = process_table
            .lines()
            .filter(|command| command.contains(profile.as_ref()))
            .collect::<Vec<_>>();
        assert!(
            !related.is_empty(),
            "no process found for RealBrowser profile {profile}"
        );
        for command in related {
            assert!(
                command.contains(bundle_root.to_string_lossy().as_ref()),
                "profile process escaped the product bundle: {command}"
            );
            assert!(
                !command.contains("Google Chrome"),
                "profile process used Google Chrome: {command}"
            );
        }
    }
}

#[cfg(not(target_os = "macos"))]
fn assert_product_process(_session: &BrowserSession, _browser: &BrowserInstall) {}

#[cfg(not(target_os = "macos"))]
fn assert_product_processes_for_profiles(_browser: &BrowserInstall, _profile_roots: &[&Path]) {}
