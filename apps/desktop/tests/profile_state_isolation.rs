use std::{
    io::{ErrorKind, Read, Write},
    net::{TcpListener, TcpStream},
    path::Path,
    sync::mpsc::{self, Sender, TryRecvError},
    thread::{self, JoinHandle},
    time::{Duration, Instant},
};

use browser_core::{BrowserHost, BrowserInstall, IdentityId, LaunchPlan};
use browser_persona::PersonaConfig;
use browser_platform::SystemBrowserHost;
use serde_json::{Value, json};
use tempfile::TempDir;
use tungstenite::{Message, WebSocket, stream::MaybeTlsStream};

const READY_TIMEOUT: Duration = Duration::from_secs(5);
const POLL_INTERVAL: Duration = Duration::from_millis(50);

#[test]
#[ignore = "requires packaged RealBrowser product Chromium and launches native processes"]
fn two_profiles_retain_only_their_own_site_state_after_restart() {
    let site = TestSite::start();
    let root = TempDir::new().unwrap();
    let profile_a = root.path().join("profile-a");
    let profile_b = root.path().join("profile-b");
    let identity_a = IdentityId::parse("00000000-0000-0000-0000-000000000161").unwrap();
    let identity_b = IdentityId::parse("00000000-0000-0000-0000-000000000162").unwrap();
    let host = SystemBrowserHost::new();
    let browser = host.detect_product_kernel().unwrap();
    let mut managed = ManagedBrowsers::new(&host);

    managed.start(identity_a, &profile_a, &browser, site.url());
    managed.start(identity_b, &profile_b, &browser, site.url());
    let mut page_a = PageClient::connect(&profile_a, site.origin());
    let mut page_b = PageClient::connect(&profile_b, site.origin());
    assert_eq!(page_a.write_account("A"), SiteState::new("account=A", "A"));
    assert_eq!(page_b.write_account("B"), SiteState::new("account=B", "B"));
    page_a.close();
    page_b.close();
    managed.stop_all();

    managed.start(identity_a, &profile_a, &browser, site.url());
    managed.start(identity_b, &profile_b, &browser, site.url());
    let mut page_a = PageClient::connect(&profile_a, site.origin());
    let mut page_b = PageClient::connect(&profile_b, site.origin());
    assert_eq!(page_a.read_account(), SiteState::new("account=A", "A"));
    assert_eq!(page_b.read_account(), SiteState::new("account=B", "B"));
    assert_ne!(page_a.read_account(), page_b.read_account());
    page_a.close();
    page_b.close();
    managed.stop_all();
}

struct ManagedBrowsers<'a> {
    host: &'a SystemBrowserHost,
    running: Vec<IdentityId>,
}

impl<'a> ManagedBrowsers<'a> {
    fn new(host: &'a SystemBrowserHost) -> Self {
        Self {
            host,
            running: Vec::new(),
        }
    }

    fn start(
        &mut self,
        identity_id: IdentityId,
        profile_root: &Path,
        browser: &BrowserInstall,
        startup_url: &str,
    ) {
        let _ = std::fs::remove_file(profile_root.join("DevToolsActivePort"));
        let persona_directory = profile_root.join(".realbrowser");
        std::fs::create_dir_all(&persona_directory).unwrap();
        let persona_path = persona_directory.join("persona.json");
        let persona = PersonaConfig::native(identity_id.persona_seed())
            .compile_kernel_persona(&identity_id.to_string(), browser.engine_major);
        std::fs::write(&persona_path, serde_json::to_vec_pretty(&persona).unwrap()).unwrap();
        let plan = LaunchPlan {
            identity_id,
            executable: browser.executable.clone(),
            profile_root: profile_root.to_path_buf(),
            startup_url: startup_url.to_owned(),
            browser_version: browser.version.clone(),
            arguments: vec![
                format!("--realbrowser-persona-file={}", persona_path.display()),
                "--headless=new".to_owned(),
                "--disable-gpu".to_owned(),
                "--remote-debugging-port=0".to_owned(),
            ],
            environment: Vec::new(),
        };
        self.host.start(&plan, 1_700_000_000_000).unwrap();
        self.running.push(identity_id);
    }

    fn stop_all(&mut self) {
        for identity_id in self.running.drain(..) {
            self.host.stop(identity_id).unwrap();
        }
    }
}

impl Drop for ManagedBrowsers<'_> {
    fn drop(&mut self) {
        for identity_id in self.running.drain(..) {
            let _ = self.host.stop(identity_id);
        }
    }
}

#[derive(Debug, Eq, PartialEq)]
struct SiteState {
    cookie: String,
    local_storage: String,
}

impl SiteState {
    fn new(cookie: &str, local_storage: &str) -> Self {
        Self {
            cookie: cookie.to_owned(),
            local_storage: local_storage.to_owned(),
        }
    }

    fn from_value(value: &Value) -> Self {
        Self {
            cookie: value["cookie"].as_str().unwrap().to_owned(),
            local_storage: value["localStorage"].as_str().unwrap().to_owned(),
        }
    }
}

struct PageClient {
    socket: WebSocket<MaybeTlsStream<TcpStream>>,
    session_id: String,
    next_id: u64,
}

impl PageClient {
    fn connect(profile_root: &Path, expected_origin: &str) -> Self {
        let endpoint = wait_for_browser_endpoint(profile_root);
        let (mut socket, _) = tungstenite::connect(endpoint).unwrap();
        if let MaybeTlsStream::Plain(stream) = socket.get_mut() {
            stream
                .set_read_timeout(Some(Duration::from_secs(3)))
                .unwrap();
        }
        let targets = raw_command(&mut socket, 1, "Target.getTargets", json!({}), None);
        let target_id = targets["targetInfos"]
            .as_array()
            .unwrap()
            .iter()
            .find(|target| target["type"] == "page")
            .and_then(|target| target["targetId"].as_str())
            .unwrap();
        let attached = raw_command(
            &mut socket,
            2,
            "Target.attachToTarget",
            json!({ "targetId": target_id, "flatten": true }),
            None,
        );
        let session_id = attached["sessionId"].as_str().unwrap().to_owned();
        let mut client = Self {
            socket,
            session_id,
            next_id: 3,
        };
        client.wait_for_origin(expected_origin);
        client
    }

    fn wait_for_origin(&mut self, expected_origin: &str) {
        let deadline = Instant::now() + READY_TIMEOUT;
        loop {
            let origin = self.evaluate("location.origin");
            if origin.as_str() == Some(expected_origin) {
                return;
            }
            assert!(
                Instant::now() < deadline,
                "page did not reach {expected_origin}"
            );
            thread::sleep(POLL_INTERVAL);
        }
    }

    fn write_account(&mut self, account: &str) -> SiteState {
        let account_json = serde_json::to_string(account).unwrap();
        let expression = format!(
            "document.cookie = `account=${{{account_json}}}; Max-Age=3600; Path=/; SameSite=Lax`; \
             localStorage.setItem('account', {account_json}); \
             ({{ cookie: document.cookie, localStorage: localStorage.getItem('account') }})"
        );
        SiteState::from_value(&self.evaluate(&expression))
    }

    fn read_account(&mut self) -> SiteState {
        SiteState::from_value(&self.evaluate(
            "({ cookie: document.cookie, localStorage: localStorage.getItem('account') })",
        ))
    }

    fn evaluate(&mut self, expression: &str) -> Value {
        let id = self.next_id;
        self.next_id = self.next_id.saturating_add(1);
        let result = raw_command(
            &mut self.socket,
            id,
            "Runtime.evaluate",
            json!({ "expression": expression, "returnByValue": true }),
            Some(&self.session_id),
        );
        result["result"]["value"].clone()
    }

    fn close(&mut self) {
        let _ = self.socket.close(None);
    }
}

fn raw_command(
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
        thread::sleep(POLL_INTERVAL);
    }
}

struct TestSite {
    origin: String,
    stop: Option<Sender<()>>,
    worker: Option<JoinHandle<()>>,
}

impl TestSite {
    fn start() -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        listener.set_nonblocking(true).unwrap();
        let origin = format!("http://{}", listener.local_addr().unwrap());
        let (stop, receiver) = mpsc::channel();
        let worker = thread::spawn(move || {
            loop {
                match receiver.try_recv() {
                    Ok(()) | Err(TryRecvError::Disconnected) => return,
                    Err(TryRecvError::Empty) => {}
                }
                match listener.accept() {
                    Ok((mut stream, _)) => serve_page(&mut stream),
                    Err(error) if error.kind() == ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(10));
                    }
                    Err(error) => panic!("test server failed: {error}"),
                }
            }
        });
        Self {
            origin,
            stop: Some(stop),
            worker: Some(worker),
        }
    }

    fn origin(&self) -> &str {
        &self.origin
    }

    fn url(&self) -> &str {
        &self.origin
    }
}

impl Drop for TestSite {
    fn drop(&mut self) {
        if let Some(stop) = self.stop.take() {
            let _ = stop.send(());
        }
        if let Some(worker) = self.worker.take() {
            let _ = worker.join();
        }
    }
}

fn serve_page(stream: &mut TcpStream) {
    let _ = stream.set_read_timeout(Some(Duration::from_millis(250)));
    let mut request = [0_u8; 2048];
    let _ = stream.read(&mut request);
    let body = b"<!doctype html><meta charset=utf-8><title>Profile State Test</title>";
    let response = format!(
        "HTTP/1.1 200 OK\r\nContent-Type: text/html; charset=utf-8\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
        body.len()
    );
    stream.write_all(response.as_bytes()).unwrap();
    stream.write_all(body).unwrap();
}
