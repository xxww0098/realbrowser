use std::{net::TcpStream, path::Path, thread, time::Duration};

use browser_core::{BrowserHost, IdentityId, LaunchPlan};
use browser_persona::{DisplayMetrics, PersonaConfig, RegionPreset, TimezonePolicy};
use browser_persona_runtime::{
    CdpPersonaRuntime, PersonaRuntime, PersonaRuntimeNavigation, PersonaRuntimeRequest,
};
use browser_platform::SystemBrowserHost;
use serde_json::{Value, json};
use tempfile::TempDir;
use tungstenite::{Message, WebSocket, stream::MaybeTlsStream};

#[test]
#[ignore = "requires packaged RealBrowser product Chromium and launches a native process"]
fn realbrowser_applies_observes_and_replays_the_managed_persona() {
    let root = TempDir::new().unwrap();
    let profile_root = root.path().join("profile");
    std::fs::create_dir_all(&profile_root).unwrap();
    let identity_id = IdentityId::parse("00000000-0000-0000-0000-000000000153").unwrap();
    let host = SystemBrowserHost::new();
    let browser = host.detect_product_kernel().unwrap();
    let persona = PersonaConfig {
        region_preset: RegionPreset::Custom,
        timezone: TimezonePolicy::iana("Asia/Tokyo").unwrap(),
        display_metrics: Some(DisplayMetrics::desktop(1366, 768, 1920, 1080, 125)),
        ..PersonaConfig::native(identity_id.persona_seed())
    };
    let mut compiled = persona.compile().unwrap();
    compiled.runtime.observe_canvas = true;
    let persona_path = profile_root.join("persona.json");
    let kernel_persona =
        persona.compile_kernel_persona(&identity_id.to_string(), browser.engine_major);
    std::fs::write(
        &persona_path,
        serde_json::to_vec_pretty(&kernel_persona).unwrap(),
    )
    .unwrap();
    let mut arguments = compiled.rendered_arguments();
    arguments.push(format!(
        "--realbrowser-persona-file={}",
        persona_path.display()
    ));
    let runtime = CdpPersonaRuntime::new();
    let request = PersonaRuntimeRequest {
        profile_root: &profile_root,
        plan: compiled.runtime,
        requested_startup_url: "about:blank",
        navigation: PersonaRuntimeNavigation::Navigate,
    };
    let contribution = runtime.prepare(&request).unwrap();
    arguments.extend(contribution.chrome_arguments().iter().cloned());
    arguments.extend(["--headless=new".to_owned(), "--disable-gpu".to_owned()]);
    let plan = LaunchPlan {
        identity_id,
        executable: browser.executable,
        profile_root: profile_root.clone(),
        startup_url: contribution.browser_startup_url().to_owned(),
        browser_version: browser.version,
        arguments,
        environment: Vec::new(),
    };

    host.start(&plan, 1_700_000_000_000).unwrap();
    let mut active = runtime.attach(&request).unwrap().unwrap();
    let timezone = active
        .observation()
        .fields
        .iter()
        .find(|field| field.field == "region.timezone")
        .unwrap();
    assert_eq!(timezone.expected, "Asia/Tokyo");
    assert_eq!(timezone.observed, "Asia/Tokyo");
    assert!(timezone.matches);
    let viewport = active
        .observation()
        .fields
        .iter()
        .find(|field| field.field == "display.viewport")
        .unwrap();
    assert_eq!(viewport.observed, "1366x768");
    let new_page = observe_persona_in_new_page(&profile_root);
    if new_page["timezone"] != "Asia/Tokyo"
        || new_page["viewportWidth"] != 1366
        || new_page["viewportHeight"] != 768
        || new_page["screenWidth"] != 1920
        || new_page["screenHeight"] != 1080
        || new_page["deviceScaleFactor"] != 1.25
    {
        let runtime_result = active.shutdown();
        panic!("new page Persona mismatch: {new_page}; runtime shutdown: {runtime_result:?}");
    }

    active.shutdown().unwrap();
    host.stop(identity_id).unwrap();
}

fn observe_persona_in_new_page(profile_root: &Path) -> Value {
    let endpoint = browser_endpoint(profile_root);
    let (mut socket, _) = tungstenite::connect(endpoint).unwrap();
    if let MaybeTlsStream::Plain(stream) = socket.get_mut() {
        stream
            .set_read_timeout(Some(Duration::from_secs(3)))
            .unwrap();
    }
    let before = command(&mut socket, 1, "Target.getTargets", json!({}), None);
    let existing_pages = before["targetInfos"]
        .as_array()
        .unwrap()
        .iter()
        .filter(|target| target["type"] == "page")
        .filter_map(|target| target["targetId"].as_str())
        .map(str::to_owned)
        .collect::<Vec<_>>();
    let created = command(
        &mut socket,
        2,
        "Target.createTarget",
        json!({ "url": "about:blank", "forTab": true }),
        None,
    );
    let tab_target_id = created["targetId"].as_str().unwrap();
    thread::sleep(Duration::from_millis(250));
    let targets = command(&mut socket, 3, "Target.getTargets", json!({}), None);
    let target_id = targets["targetInfos"]
        .as_array()
        .unwrap()
        .iter()
        .find(|target| {
            target["type"] == "page"
                && target["targetId"]
                    .as_str()
                    .is_some_and(|id| !existing_pages.iter().any(|existing| existing == id))
        })
        .and_then(|target| target["targetId"].as_str())
        .unwrap_or_else(|| panic!("new page target missing: {targets}"));
    let attached = command(
        &mut socket,
        4,
        "Target.attachToTarget",
        json!({ "targetId": target_id, "flatten": true }),
        None,
    );
    let session_id = attached["sessionId"].as_str().unwrap();
    let mut persona = Value::Null;
    for id in 5..25 {
        let observed = command(
            &mut socket,
            id,
            "Runtime.evaluate",
            json!({
                "expression": "({timezone: Intl.DateTimeFormat().resolvedOptions().timeZone, viewportWidth: innerWidth, viewportHeight: innerHeight, screenWidth: screen.width, screenHeight: screen.height, deviceScaleFactor: devicePixelRatio})",
                "returnByValue": true
            }),
            Some(session_id),
        );
        persona = observed["result"]["value"].clone();
        if persona["timezone"] == "Asia/Tokyo"
            && persona["viewportWidth"] == 1366
            && persona["screenWidth"] == 1920
            && persona["deviceScaleFactor"] == 1.25
        {
            break;
        }
        thread::sleep(Duration::from_millis(100));
    }
    command(
        &mut socket,
        25,
        "Target.closeTarget",
        json!({ "targetId": tab_target_id }),
        None,
    );
    persona
}

fn browser_endpoint(profile_root: &Path) -> String {
    let contents = std::fs::read_to_string(profile_root.join("DevToolsActivePort")).unwrap();
    let mut lines = contents.lines();
    let port = lines.next().unwrap();
    let path = lines.next().unwrap();
    format!("ws://127.0.0.1:{port}{path}")
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
