#![forbid(unsafe_code)]

use std::{
    collections::VecDeque,
    fs,
    io::ErrorKind,
    net::TcpStream,
    path::{Path, PathBuf},
    sync::mpsc::{self, Sender, TryRecvError},
    thread::{self, JoinHandle},
    time::{Duration, Instant},
};

use browser_persona::{DisplayMetrics, ObservedField, PersonaObservation, PersonaRuntimePlan};
use serde_json::{Value, json};
use thiserror::Error;
use tungstenite::{Message, WebSocket, stream::MaybeTlsStream};

const DEVTOOLS_ACTIVE_PORT: &str = "DevToolsActivePort";
const REMOTE_DEBUGGING_ARGUMENT: &str = "--remote-debugging-port=0";
const RUNTIME_STARTUP_URL: &str = "about:blank";
const DEFAULT_READY_TIMEOUT: Duration = Duration::from_secs(5);
const COMMAND_TIMEOUT: Duration = Duration::from_secs(3);
const KEEPALIVE_POLL: Duration = Duration::from_millis(200);
const CANVAS_OBSERVATION: &str = "seeded_idempotent_copy_all_contexts_native";
pub const CANVAS_K1_OBSERVATION_SCRIPT: &str = r#"(async () => {
  const hashBytes = (bytes) => {
    let hash = 2166136261;
    for (const byte of bytes) hash = Math.imul(hash ^ byte, 16777619) >>> 0;
    return hash.toString(16).padStart(8, '0');
  };
  const hashString = (value) => hashBytes(new TextEncoder().encode(value));
  const draw = (canvas) => {
    canvas.width = 32;
    canvas.height = 20;
    const context = canvas.getContext('2d');
    context.fillStyle = '#17324d';
    context.fillRect(0, 0, 32, 20);
    context.fillStyle = '#d4813b';
    context.fillRect(3, 2, 17, 11);
    context.fillStyle = 'rgba(41, 191, 126, 0.73)';
    context.fillRect(11, 7, 18, 10);
    return context;
  };
  const blobHash = async (blob) => hashBytes(new Uint8Array(await blob.arrayBuffer()));
  const canvasResult = async (doc) => {
    const canvas = doc.createElement('canvas');
    const context = draw(canvas);
    const sourceHash = () => {
      const probe = doc.createElement('canvas');
      probe.width = 32;
      probe.height = 20;
      const probeContext = probe.getContext('2d');
      probeContext.drawImage(canvas, 0, 0);
      return hashBytes(probeContext.getImageData(0, 0, 32, 20).data);
    };
    const sourceBefore = sourceHash();
    const getA = hashBytes(context.getImageData(0, 0, 32, 20).data);
    const getB = hashBytes(context.getImageData(0, 0, 32, 20).data);
    const urlA = hashString(canvas.toDataURL());
    const urlB = hashString(canvas.toDataURL());
    const toBlob = () => new Promise((resolve, reject) => canvas.toBlob((blob) => blob ? resolve(blob) : reject(new Error('toBlob returned null'))));
    const blobA = await toBlob();
    const blobB = await toBlob();
    const sourceAfter = sourceHash();
    return {getA, getB, urlA, urlB, blobA: await blobHash(blobA), blobB: await blobHash(blobB), sourceBefore, sourceAfter};
  };
  const top = await canvasResult(document);
  const iframe = document.createElement('iframe');
  iframe.srcdoc = '<!doctype html><meta charset="utf-8">';
  await new Promise((resolve, reject) => {
    iframe.addEventListener('load', resolve, {once: true});
    setTimeout(() => reject(new Error('iframe timeout')), 2000);
    document.documentElement.appendChild(iframe);
  });
  const frame = await canvasResult(iframe.contentDocument);
  iframe.remove();
  const workerSource = `
    const hashBytes = ${hashBytes.toString()};
    const draw = ${draw.toString()};
    const blobHash = ${blobHash.toString()};
    onmessage = async () => {
      const canvas = new OffscreenCanvas(32, 20);
      const context = draw(canvas);
      const sourceHash = () => {
        const probe = new OffscreenCanvas(32, 20);
        const probeContext = probe.getContext('2d');
        probeContext.drawImage(canvas, 0, 0);
        return hashBytes(probeContext.getImageData(0, 0, 32, 20).data);
      };
      const sourceBefore = sourceHash();
      const getA = hashBytes(context.getImageData(0, 0, 32, 20).data);
      const getB = hashBytes(context.getImageData(0, 0, 32, 20).data);
      const blobA = await blobHash(await canvas.convertToBlob());
      const blobB = await blobHash(await canvas.convertToBlob());
      const sourceAfter = sourceHash();
      postMessage({getA, getB, blobA, blobB, sourceBefore, sourceAfter});
    };
  `;
  const workerUrl = URL.createObjectURL(new Blob([workerSource], {type: 'text/javascript'}));
  const worker = new Worker(workerUrl);
  const workerResult = await new Promise((resolve, reject) => {
    worker.onmessage = (event) => resolve(event.data);
    worker.onerror = (event) => reject(new Error(event.message));
    worker.postMessage(null);
    setTimeout(() => reject(new Error('worker timeout')), 3000);
  });
  worker.terminate();
  URL.revokeObjectURL(workerUrl);
  const nativeFunctions = [
    CanvasRenderingContext2D.prototype.getImageData,
    HTMLCanvasElement.prototype.toDataURL,
    HTMLCanvasElement.prototype.toBlob,
    OffscreenCanvas.prototype.convertToBlob,
  ].every((fn) => Function.prototype.toString.call(fn).includes('[native code]'));
  const stable = top.getA === top.getB && top.urlA === top.urlB && top.blobA === top.blobB && top.sourceBefore === top.sourceAfter;
  const frameMatches = top.getA === frame.getA && top.urlA === frame.urlA && top.blobA === frame.blobA;
  const workerMatches = top.getA === workerResult.getA && top.blobA === workerResult.blobA;
  const workerStable = workerResult.getA === workerResult.getB && workerResult.blobA === workerResult.blobB && workerResult.sourceBefore === workerResult.sourceAfter;
  if (!stable || !frameMatches || !workerMatches || !workerStable || !nativeFunctions) {
    throw new Error(JSON.stringify({stable, frameMatches, workerMatches, workerStable, nativeFunctions, top, frame, workerResult}));
  }
  return {
    status: 'seeded_idempotent_copy_all_contexts_native',
    getImageDataHash: top.getA,
    dataUrlHash: top.urlA,
    blobHash: top.blobA,
    iframeHash: frame.getA,
    workerHash: workerResult.getA,
    nativeFunctions,
  };
})()"#;

pub struct PersonaRuntimeRequest<'a> {
    pub profile_root: &'a Path,
    pub plan: PersonaRuntimePlan,
    pub requested_startup_url: &'a str,
    pub navigation: PersonaRuntimeNavigation,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum PersonaRuntimeNavigation {
    Navigate,
    Preserve,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PersonaRuntimeLaunch {
    chrome_arguments: Vec<String>,
    browser_startup_url: String,
}

impl PersonaRuntimeLaunch {
    #[must_use]
    pub fn native(requested_startup_url: &str) -> Self {
        Self {
            chrome_arguments: Vec::new(),
            browser_startup_url: requested_startup_url.to_owned(),
        }
    }

    #[must_use]
    pub fn managed(chrome_arguments: Vec<String>, browser_startup_url: &str) -> Self {
        Self {
            chrome_arguments,
            browser_startup_url: browser_startup_url.to_owned(),
        }
    }

    #[must_use]
    pub fn chrome_arguments(&self) -> &[String] {
        &self.chrome_arguments
    }

    #[must_use]
    pub fn browser_startup_url(&self) -> &str {
        &self.browser_startup_url
    }
}

pub trait ActivePersonaRuntime: Send {
    fn observation(&self) -> &PersonaObservation;
    fn shutdown(&mut self) -> Result<(), PersonaRuntimeError>;
}

pub trait PersonaRuntime: Send + Sync {
    fn prepare(
        &self,
        request: &PersonaRuntimeRequest<'_>,
    ) -> Result<PersonaRuntimeLaunch, PersonaRuntimeError>;

    fn attach(
        &self,
        request: &PersonaRuntimeRequest<'_>,
    ) -> Result<Option<Box<dyn ActivePersonaRuntime>>, PersonaRuntimeError>;
}

pub struct CdpPersonaRuntime {
    ready_timeout: Duration,
}

impl CdpPersonaRuntime {
    #[must_use]
    pub fn new() -> Self {
        Self {
            ready_timeout: DEFAULT_READY_TIMEOUT,
        }
    }

    #[cfg(test)]
    fn with_ready_timeout(ready_timeout: Duration) -> Self {
        Self { ready_timeout }
    }
}

impl Default for CdpPersonaRuntime {
    fn default() -> Self {
        Self::new()
    }
}

impl PersonaRuntime for CdpPersonaRuntime {
    fn prepare(
        &self,
        request: &PersonaRuntimeRequest<'_>,
    ) -> Result<PersonaRuntimeLaunch, PersonaRuntimeError> {
        if !request.plan.is_required() {
            return Ok(PersonaRuntimeLaunch::native(request.requested_startup_url));
        }

        let endpoint_file = endpoint_file(request.profile_root);
        match fs::remove_file(&endpoint_file) {
            Ok(()) => {}
            Err(error) if error.kind() == ErrorKind::NotFound => {}
            Err(error) => {
                return Err(PersonaRuntimeError::Io {
                    operation: "清理旧 CDP 端点",
                    detail: error.to_string(),
                });
            }
        }

        Ok(PersonaRuntimeLaunch::managed(
            vec![REMOTE_DEBUGGING_ARGUMENT.to_owned()],
            RUNTIME_STARTUP_URL,
        ))
    }

    fn attach(
        &self,
        request: &PersonaRuntimeRequest<'_>,
    ) -> Result<Option<Box<dyn ActivePersonaRuntime>>, PersonaRuntimeError> {
        if !request.plan.is_required() {
            return Ok(None);
        }
        let endpoint = wait_for_endpoint(request.profile_root, self.ready_timeout)?;
        let (mut socket, _) = tungstenite::connect(endpoint.as_str()).map_err(|error| {
            PersonaRuntimeError::Protocol {
                stage: "连接 CDP",
                detail: error.to_string(),
            }
        })?;
        set_read_timeout(&mut socket, KEEPALIVE_POLL)?;

        let mut protocol = CdpProtocol::new(&mut socket);
        let target_id = protocol.first_page_target()?;
        let session_id = protocol.attach_to_target(&target_id)?;
        apply_plan_to_session(&mut protocol, &session_id, &request.plan)?;
        let fields = observe_plan(&mut protocol, &session_id, &request.plan)?;
        protocol.discover_targets()?;
        if request.navigation == PersonaRuntimeNavigation::Navigate
            && request.requested_startup_url != RUNTIME_STARTUP_URL
        {
            protocol.navigate(&session_id, request.requested_startup_url)?;
        }
        let pending_events = protocol.take_pending_events();
        drop(protocol);

        let observation = PersonaObservation { fields };
        let (stop, stop_receiver) = mpsc::channel();
        let plan = request.plan.clone();
        let worker = thread::Builder::new()
            .name("realbrowser-persona-cdp".to_owned())
            .spawn(move || keep_connection(socket, stop_receiver, pending_events, &plan))
            .map_err(|error| PersonaRuntimeError::Io {
                operation: "启动 CDP 保持线程",
                detail: error.to_string(),
            })?;

        Ok(Some(Box::new(CdpPersonaSession {
            observation,
            stop: Some(stop),
            worker: Some(worker),
        })))
    }
}

struct CdpPersonaSession {
    observation: PersonaObservation,
    stop: Option<Sender<()>>,
    worker: Option<JoinHandle<Result<(), PersonaRuntimeError>>>,
}

impl ActivePersonaRuntime for CdpPersonaSession {
    fn observation(&self) -> &PersonaObservation {
        &self.observation
    }

    fn shutdown(&mut self) -> Result<(), PersonaRuntimeError> {
        if let Some(stop) = self.stop.take() {
            let _ = stop.send(());
        }
        if let Some(worker) = self.worker.take() {
            worker.join().map_err(|_| PersonaRuntimeError::Protocol {
                stage: "关闭 CDP 会话",
                detail: "保持线程异常退出".to_owned(),
            })??;
        }
        Ok(())
    }
}

impl Drop for CdpPersonaSession {
    fn drop(&mut self) {
        let _ = self.shutdown();
    }
}

struct CdpProtocol<'a> {
    socket: &'a mut WebSocket<MaybeTlsStream<TcpStream>>,
    next_id: u64,
    pending_events: VecDeque<Value>,
}

impl<'a> CdpProtocol<'a> {
    fn new(socket: &'a mut WebSocket<MaybeTlsStream<TcpStream>>) -> Self {
        Self {
            socket,
            next_id: 1,
            pending_events: VecDeque::new(),
        }
    }

    fn with_pending_events(
        socket: &'a mut WebSocket<MaybeTlsStream<TcpStream>>,
        pending_events: VecDeque<Value>,
    ) -> Self {
        Self {
            socket,
            next_id: 1,
            pending_events,
        }
    }

    fn take_pending_events(&mut self) -> VecDeque<Value> {
        std::mem::take(&mut self.pending_events)
    }

    fn first_page_target(&mut self) -> Result<String, PersonaRuntimeError> {
        let result = self.command("Target.getTargets", json!({}), None)?;
        result["targetInfos"]
            .as_array()
            .and_then(|targets| {
                targets.iter().find_map(|target| {
                    (target["type"] == "page")
                        .then(|| target["targetId"].as_str().map(str::to_owned))
                        .flatten()
                })
            })
            .ok_or_else(|| PersonaRuntimeError::Protocol {
                stage: "定位初始页面",
                detail: "RealBrowser 未返回 page target".to_owned(),
            })
    }

    fn attach_to_target(&mut self, target_id: &str) -> Result<String, PersonaRuntimeError> {
        let result = self.command(
            "Target.attachToTarget",
            json!({ "targetId": target_id, "flatten": true }),
            None,
        )?;
        result["sessionId"]
            .as_str()
            .map(str::to_owned)
            .ok_or_else(|| PersonaRuntimeError::Protocol {
                stage: "附着初始页面",
                detail: "RealBrowser 未返回 sessionId".to_owned(),
            })
    }

    fn set_timezone(
        &mut self,
        session_id: &str,
        timezone_id: &str,
    ) -> Result<(), PersonaRuntimeError> {
        self.command(
            "Emulation.setTimezoneOverride",
            json!({ "timezoneId": timezone_id }),
            Some(session_id),
        )?;
        Ok(())
    }

    fn set_display_metrics(
        &mut self,
        session_id: &str,
        metrics: DisplayMetrics,
    ) -> Result<(), PersonaRuntimeError> {
        self.command(
            "Emulation.setDeviceMetricsOverride",
            json!({
                "width": metrics.viewport_width,
                "height": metrics.viewport_height,
                "deviceScaleFactor": metrics.device_scale_factor(),
                "mobile": false,
                "screenWidth": metrics.screen_width,
                "screenHeight": metrics.screen_height
            }),
            Some(session_id),
        )?;
        Ok(())
    }

    fn discover_targets(&mut self) -> Result<(), PersonaRuntimeError> {
        self.command(
            "Target.setDiscoverTargets",
            json!({ "discover": true }),
            None,
        )?;
        Ok(())
    }

    fn auto_attach_related(&mut self, target_id: &str) -> Result<(), PersonaRuntimeError> {
        self.command(
            "Target.autoAttachRelated",
            json!({
                "targetId": target_id,
                "waitForDebuggerOnStart": true
            }),
            None,
        )?;
        Ok(())
    }

    fn set_related_auto_attach(&mut self, session_id: &str) -> Result<(), PersonaRuntimeError> {
        self.command(
            "Target.setAutoAttach",
            json!({
                "autoAttach": true,
                "waitForDebuggerOnStart": true,
                "flatten": true
            }),
            Some(session_id),
        )?;
        Ok(())
    }

    fn resume_target(&mut self, session_id: &str) -> Result<(), PersonaRuntimeError> {
        self.command(
            "Runtime.runIfWaitingForDebugger",
            json!({}),
            Some(session_id),
        )?;
        Ok(())
    }

    fn observe_timezone(&mut self, session_id: &str) -> Result<String, PersonaRuntimeError> {
        let result = self.command(
            "Runtime.evaluate",
            json!({
                "expression": "Intl.DateTimeFormat().resolvedOptions().timeZone",
                "returnByValue": true
            }),
            Some(session_id),
        )?;
        result["result"]["value"]
            .as_str()
            .map(str::to_owned)
            .ok_or_else(|| PersonaRuntimeError::Protocol {
                stage: "观测时区",
                detail: "RealBrowser 未返回字符串结果".to_owned(),
            })
    }

    fn observe_display_metrics(
        &mut self,
        session_id: &str,
    ) -> Result<ObservedDisplayMetrics, PersonaRuntimeError> {
        let result = self.command(
            "Runtime.evaluate",
            json!({
                "expression": "({viewportWidth: innerWidth, viewportHeight: innerHeight, screenWidth: screen.width, screenHeight: screen.height, deviceScaleFactor: devicePixelRatio})",
                "returnByValue": true
            }),
            Some(session_id),
        )?;
        let value = &result["result"]["value"];
        Ok(ObservedDisplayMetrics {
            viewport_width: observed_u16(value, "viewportWidth")?,
            viewport_height: observed_u16(value, "viewportHeight")?,
            screen_width: observed_u16(value, "screenWidth")?,
            screen_height: observed_u16(value, "screenHeight")?,
            device_scale_factor: value["deviceScaleFactor"].as_f64().ok_or_else(|| {
                PersonaRuntimeError::Protocol {
                    stage: "观测设备指标",
                    detail: "RealBrowser 未返回 deviceScaleFactor".to_owned(),
                }
            })?,
        })
    }

    fn observe_canvas(&mut self, session_id: &str) -> Result<String, PersonaRuntimeError> {
        let result = self.command(
            "Runtime.evaluate",
            json!({
                "expression": CANVAS_K1_OBSERVATION_SCRIPT,
                "awaitPromise": true,
                "returnByValue": true
            }),
            Some(session_id),
        )?;
        if let Some(exception) = result.get("exceptionDetails") {
            return Err(PersonaRuntimeError::Protocol {
                stage: "观测 Canvas K1",
                detail: exception.to_string(),
            });
        }
        result["result"]["value"]["status"]
            .as_str()
            .map(str::to_owned)
            .ok_or_else(|| PersonaRuntimeError::Protocol {
                stage: "观测 Canvas K1",
                detail: "RealBrowser 未返回 Canvas K1 观测结果".to_owned(),
            })
    }

    fn navigate(&mut self, session_id: &str, url: &str) -> Result<(), PersonaRuntimeError> {
        self.command("Page.navigate", json!({ "url": url }), Some(session_id))?;
        Ok(())
    }

    fn command(
        &mut self,
        method: &'static str,
        params: Value,
        session_id: Option<&str>,
    ) -> Result<Value, PersonaRuntimeError> {
        let id = self.next_id;
        self.next_id = self.next_id.saturating_add(1);
        let mut payload = json!({ "id": id, "method": method, "params": params });
        if let Some(session_id) = session_id {
            payload["sessionId"] = Value::String(session_id.to_owned());
        }
        self.socket
            .send(Message::Text(payload.to_string().into()))
            .map_err(|error| PersonaRuntimeError::Protocol {
                stage: method,
                detail: error.to_string(),
            })?;

        let deadline = Instant::now() + COMMAND_TIMEOUT;
        loop {
            match self.socket.read() {
                Ok(Message::Text(text)) => {
                    let response = serde_json::from_str::<Value>(&text).map_err(|error| {
                        PersonaRuntimeError::Protocol {
                            stage: method,
                            detail: error.to_string(),
                        }
                    })?;
                    if response["id"].as_u64() != Some(id) {
                        if response.get("method").is_some() {
                            self.pending_events.push_back(response);
                        }
                        continue;
                    }
                    if response.get("error").is_some() {
                        let code = response["error"]["code"].as_i64().unwrap_or_default();
                        let message = response["error"]["message"]
                            .as_str()
                            .unwrap_or("RealBrowser 拒绝命令");
                        return Err(PersonaRuntimeError::Protocol {
                            stage: method,
                            detail: format!("CDP {code}: {message}"),
                        });
                    }
                    return response.get("result").cloned().ok_or_else(|| {
                        PersonaRuntimeError::Protocol {
                            stage: method,
                            detail: "RealBrowser 未返回 result".to_owned(),
                        }
                    });
                }
                Ok(Message::Close(_)) => {
                    return Err(PersonaRuntimeError::Protocol {
                        stage: method,
                        detail: "RealBrowser 提前关闭 CDP".to_owned(),
                    });
                }
                Ok(_) => {}
                Err(tungstenite::Error::Io(error))
                    if matches!(error.kind(), ErrorKind::WouldBlock | ErrorKind::TimedOut) => {}
                Err(error) => {
                    return Err(PersonaRuntimeError::Protocol {
                        stage: method,
                        detail: error.to_string(),
                    });
                }
            }
            if Instant::now() >= deadline {
                return Err(PersonaRuntimeError::Protocol {
                    stage: method,
                    detail: "等待 RealBrowser 响应超时".to_owned(),
                });
            }
        }
    }
}

#[derive(Clone, Copy, Debug, PartialEq)]
struct ObservedDisplayMetrics {
    viewport_width: u16,
    viewport_height: u16,
    screen_width: u16,
    screen_height: u16,
    device_scale_factor: f64,
}

fn observed_u16(value: &Value, field: &'static str) -> Result<u16, PersonaRuntimeError> {
    value[field]
        .as_u64()
        .and_then(|value| u16::try_from(value).ok())
        .ok_or_else(|| PersonaRuntimeError::Protocol {
            stage: "观测设备指标",
            detail: format!("RealBrowser 未返回 {field}"),
        })
}

fn apply_plan_to_session(
    protocol: &mut CdpProtocol<'_>,
    session_id: &str,
    plan: &PersonaRuntimePlan,
) -> Result<(), PersonaRuntimeError> {
    if let Some(timezone_id) = plan.timezone_id.as_deref() {
        protocol.set_timezone(session_id, timezone_id)?;
    }
    if let Some(metrics) = plan.display_metrics {
        protocol.set_display_metrics(session_id, metrics)?;
    }
    Ok(())
}

fn observe_plan(
    protocol: &mut CdpProtocol<'_>,
    session_id: &str,
    plan: &PersonaRuntimePlan,
) -> Result<Vec<ObservedField>, PersonaRuntimeError> {
    let mut fields = Vec::new();
    if let Some(timezone_id) = plan.timezone_id.as_deref() {
        let observed = protocol.observe_timezone(session_id)?;
        push_observation(&mut fields, "region.timezone", timezone_id, observed)?;
    }
    if let Some(metrics) = plan.display_metrics {
        let observed = protocol.observe_display_metrics(session_id)?;
        push_observation(
            &mut fields,
            "display.viewport",
            &format!("{}x{}", metrics.viewport_width, metrics.viewport_height),
            format!("{}x{}", observed.viewport_width, observed.viewport_height),
        )?;
        push_observation(
            &mut fields,
            "display.screen",
            &format!("{}x{}", metrics.screen_width, metrics.screen_height),
            format!("{}x{}", observed.screen_width, observed.screen_height),
        )?;
        push_observation(
            &mut fields,
            "display.devicePixelRatio",
            &format_scale(metrics.device_scale_factor()),
            format_scale(observed.device_scale_factor),
        )?;
    }
    if plan.observe_canvas {
        let observed = protocol.observe_canvas(session_id)?;
        push_observation(&mut fields, "graphics.canvas", CANVAS_OBSERVATION, observed)?;
    }
    Ok(fields)
}

fn push_observation(
    fields: &mut Vec<ObservedField>,
    field: &'static str,
    expected: &str,
    observed: String,
) -> Result<(), PersonaRuntimeError> {
    if observed != expected {
        return Err(PersonaRuntimeError::ObservationMismatch {
            field,
            expected: expected.to_owned(),
            observed,
        });
    }
    fields.push(ObservedField {
        field: field.to_owned(),
        expected: expected.to_owned(),
        observed,
        matches: true,
    });
    Ok(())
}

fn format_scale(value: f64) -> String {
    let value = format!("{value:.2}");
    value.trim_end_matches('0').trim_end_matches('.').to_owned()
}

fn endpoint_file(profile_root: &Path) -> PathBuf {
    profile_root.join(DEVTOOLS_ACTIVE_PORT)
}

fn wait_for_endpoint(
    profile_root: &Path,
    timeout: Duration,
) -> Result<String, PersonaRuntimeError> {
    let path = endpoint_file(profile_root);
    let deadline = Instant::now() + timeout;
    loop {
        match fs::read_to_string(&path) {
            Ok(contents) => match parse_endpoint(&contents) {
                Ok(endpoint) => return Ok(endpoint),
                Err(_) if Instant::now() < deadline => {}
                Err(error) => return Err(error),
            },
            Err(error) if error.kind() == ErrorKind::NotFound && Instant::now() < deadline => {}
            Err(error) if error.kind() == ErrorKind::NotFound => {
                return Err(PersonaRuntimeError::EndpointUnavailable);
            }
            Err(error) => {
                return Err(PersonaRuntimeError::Io {
                    operation: "读取 CDP 端点",
                    detail: error.to_string(),
                });
            }
        }
        thread::sleep(Duration::from_millis(25));
    }
}

fn parse_endpoint(contents: &str) -> Result<String, PersonaRuntimeError> {
    let mut lines = contents.lines();
    let port = lines
        .next()
        .and_then(|value| value.parse::<u16>().ok())
        .filter(|port| *port != 0)
        .ok_or(PersonaRuntimeError::InvalidEndpoint)?;
    let path = lines.next().ok_or(PersonaRuntimeError::InvalidEndpoint)?;
    let valid_path = path.starts_with("/devtools/browser/")
        && path.len() > "/devtools/browser/".len()
        && path.chars().all(|character| {
            character.is_ascii_alphanumeric() || matches!(character, '/' | '-' | '_')
        });
    if !valid_path {
        return Err(PersonaRuntimeError::InvalidEndpoint);
    }
    Ok(format!("ws://127.0.0.1:{port}{path}"))
}

fn set_read_timeout(
    socket: &mut WebSocket<MaybeTlsStream<TcpStream>>,
    timeout: Duration,
) -> Result<(), PersonaRuntimeError> {
    match socket.get_mut() {
        MaybeTlsStream::Plain(stream) => {
            stream
                .set_read_timeout(Some(timeout))
                .map_err(|error| PersonaRuntimeError::Io {
                    operation: "配置 CDP 超时",
                    detail: error.to_string(),
                })
        }
        _ => Err(PersonaRuntimeError::InvalidEndpoint),
    }
}

fn keep_connection(
    mut socket: WebSocket<MaybeTlsStream<TcpStream>>,
    stop: mpsc::Receiver<()>,
    pending_events: VecDeque<Value>,
    plan: &PersonaRuntimePlan,
) -> Result<(), PersonaRuntimeError> {
    let mut protocol = CdpProtocol::with_pending_events(&mut socket, pending_events);
    loop {
        match stop.try_recv() {
            Ok(()) | Err(TryRecvError::Disconnected) => {
                let _ = protocol.socket.close(None);
                return Ok(());
            }
            Err(TryRecvError::Empty) => {}
        }
        if let Some(event) = protocol.pending_events.pop_front() {
            apply_runtime_event(&mut protocol, &event, plan)?;
            continue;
        }
        match protocol.socket.read() {
            Ok(Message::Close(_)) | Err(tungstenite::Error::ConnectionClosed) => return Ok(()),
            Ok(Message::Text(text)) => {
                let event = serde_json::from_str::<Value>(&text).map_err(|error| {
                    PersonaRuntimeError::Protocol {
                        stage: "读取自动附着事件",
                        detail: error.to_string(),
                    }
                })?;
                apply_runtime_event(&mut protocol, &event, plan)?;
                let _ = protocol.socket.flush();
            }
            Ok(_) => {}
            Err(tungstenite::Error::Io(error))
                if matches!(error.kind(), ErrorKind::WouldBlock | ErrorKind::TimedOut) => {}
            Err(error) => {
                return Err(PersonaRuntimeError::Protocol {
                    stage: "保持 CDP 会话",
                    detail: error.to_string(),
                });
            }
        }
    }
}

fn apply_runtime_event(
    protocol: &mut CdpProtocol<'_>,
    event: &Value,
    plan: &PersonaRuntimePlan,
) -> Result<(), PersonaRuntimeError> {
    match event["method"].as_str() {
        Some("Target.attachedToTarget") => apply_attached_target(protocol, event, plan),
        Some("Target.targetCreated") => apply_created_target(protocol, event, plan),
        _ => Ok(()),
    }
}

fn apply_attached_target(
    protocol: &mut CdpProtocol<'_>,
    event: &Value,
    plan: &PersonaRuntimePlan,
) -> Result<(), PersonaRuntimeError> {
    let Some(session_id) = event["params"]["sessionId"].as_str() else {
        return Err(PersonaRuntimeError::Protocol {
            stage: "处理自动附着页面",
            detail: "RealBrowser 未返回 sessionId".to_owned(),
        });
    };
    let target_type = event["params"]["targetInfo"]["type"]
        .as_str()
        .unwrap_or_default();
    let waiting = event["params"]["waitingForDebugger"]
        .as_bool()
        .unwrap_or(false);

    let application = match target_type {
        "page" | "iframe" => apply_plan_to_session(protocol, session_id, plan)
            .and_then(|()| protocol.set_related_auto_attach(session_id)),
        "tab" => protocol.set_related_auto_attach(session_id),
        _ => Ok(()),
    };
    let resume = waiting
        .then(|| protocol.resume_target(session_id))
        .transpose()
        .map(|_| ());
    application.and(resume)
}

fn apply_created_target(
    protocol: &mut CdpProtocol<'_>,
    event: &Value,
    plan: &PersonaRuntimePlan,
) -> Result<(), PersonaRuntimeError> {
    let target = &event["params"]["targetInfo"];
    let Some(target_id) = target["targetId"].as_str() else {
        return Err(PersonaRuntimeError::Protocol {
            stage: "处理新页面",
            detail: "RealBrowser 未返回 targetId".to_owned(),
        });
    };
    match target["type"].as_str() {
        Some("tab") => protocol.auto_attach_related(target_id),
        Some("page") => {
            let session_id = protocol.attach_to_target(target_id)?;
            apply_plan_to_session(protocol, &session_id, plan)?;
            protocol.set_related_auto_attach(&session_id)
        }
        _ => Ok(()),
    }
}

#[derive(Debug, Error)]
pub enum PersonaRuntimeError {
    #[error("{operation}失败：{detail}")]
    Io {
        operation: &'static str,
        detail: String,
    },
    #[error("RealBrowser 未在时限内提供 Persona Runtime 端点")]
    EndpointUnavailable,
    #[error("RealBrowser 返回了无效的 Persona Runtime 端点")]
    InvalidEndpoint,
    #[error("{stage}失败：{detail}")]
    Protocol { stage: &'static str, detail: String },
    #[error("{field} 观测不一致：期望 {expected}，实际 {observed}")]
    ObservationMismatch {
        field: &'static str,
        expected: String,
        observed: String,
    },
}

#[cfg(test)]
mod tests;
