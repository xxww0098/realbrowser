#![forbid(unsafe_code)]

use std::{
    collections::HashMap,
    fs::OpenOptions,
    io::Write,
    path::{Path, PathBuf},
    sync::Arc,
    time::{SystemTime, UNIX_EPOCH},
};

use browser_core::{
    BrowserHost, BrowserIdentity, BrowserRuntimeRecord, BrowserSession, Clock, CreateIdentity,
    EgressMode, FieldIssue, IdGenerator, IdentityId, IdentityLifecycle, IdentitySnapshot,
    IdentityStore, LaunchPlan, RuntimeJournal, RuntimeState, StopResult, StoreError,
    SystemSnapshot, normalize_identity_name, normalize_startup_url,
};
use browser_network::{NetworkConfig, NetworkError, NetworkMode};
use browser_persona::{PersonaConfig, PersonaError, WebRtcPolicy};
use browser_persona_runtime::{
    ActivePersonaRuntime, PersonaRuntime, PersonaRuntimeNavigation, PersonaRuntimeRequest,
};
use browser_profile::{ProfileLease, ProfileLeaseError, prepare_profile_root};
use parking_lot::Mutex;
use thiserror::Error;
use uuid::Uuid;

#[derive(Debug, Error)]
pub enum ControlError {
    #[error("输入内容需要修正")]
    Validation(Vec<FieldIssue>),
    #[error("找不到这个浏览器环境")]
    NotFound,
    #[error("这个浏览器环境已在运行")]
    AlreadyRunning,
    #[error("请先停止浏览器环境")]
    StillRunning,
    #[error("环境已被其他操作更新，请刷新后重试")]
    RevisionConflict,
    #[error("RealBrowser 产品内核不可用：{0}")]
    BrowserUnavailable(String),
    #[error("浏览器操作失败：{0}")]
    Runtime(String),
    #[error("指纹运行时不可用：{0}")]
    PersonaUnavailable(String),
    #[error("本地存储失败：{0}")]
    Storage(String),
    #[error("本地文件系统失败：{0}")]
    Filesystem(String),
    #[error("Profile 所有权不可用：{0}")]
    ProfileUnavailable(String),
}

impl ControlError {
    #[must_use]
    pub fn code(&self) -> &'static str {
        match self {
            Self::Validation(_) => "validation_failed",
            Self::NotFound => "not_found",
            Self::AlreadyRunning => "already_running",
            Self::StillRunning => "still_running",
            Self::RevisionConflict => "revision_conflict",
            Self::BrowserUnavailable(_) => "browser_unavailable",
            Self::Runtime(_) => "runtime_failed",
            Self::PersonaUnavailable(_) => "persona_unavailable",
            Self::Storage(_) => "storage_failed",
            Self::Filesystem(_) => "filesystem_failed",
            Self::ProfileUnavailable(_) => "profile_unavailable",
        }
    }

    #[must_use]
    pub fn field_issues(&self) -> &[FieldIssue] {
        match self {
            Self::Validation(issues) => issues,
            _ => &[],
        }
    }

    #[must_use]
    pub fn public_message(&self) -> &'static str {
        match self {
            Self::Validation(_) => "输入内容需要修正",
            Self::NotFound => "找不到这个浏览器环境",
            Self::AlreadyRunning => "这个浏览器环境已在运行",
            Self::StillRunning => "请先停止浏览器环境",
            Self::RevisionConflict => "环境已被其他操作更新，请刷新后重试",
            Self::BrowserUnavailable(_) => "RealBrowser 产品内核不可用",
            Self::Runtime(_) => "浏览器操作失败，请重试",
            Self::PersonaUnavailable(_) => "指纹运行时不可用",
            Self::Storage(_) => "本地数据暂时不可用",
            Self::Filesystem(_) => "本地数据目录不可用",
            Self::ProfileUnavailable(_) => "Profile 暂时不可用",
        }
    }
}

pub struct ControlService {
    store: Arc<dyn IdentityStore>,
    journal: Arc<dyn RuntimeJournal>,
    host: Arc<dyn BrowserHost>,
    persona_runtime: Arc<dyn PersonaRuntime>,
    clock: Arc<dyn Clock>,
    ids: Arc<dyn IdGenerator>,
    data_root: PathBuf,
    sessions: Mutex<HashMap<IdentityId, ActiveRuntime>>,
}

struct ActiveRuntime {
    session: BrowserSession,
    persona_runtime: Option<Box<dyn ActivePersonaRuntime>>,
    _lease: ProfileLease,
}

impl ControlService {
    #[must_use]
    pub fn new(
        store: Arc<dyn IdentityStore>,
        journal: Arc<dyn RuntimeJournal>,
        host: Arc<dyn BrowserHost>,
        persona_runtime: Arc<dyn PersonaRuntime>,
        clock: Arc<dyn Clock>,
        ids: Arc<dyn IdGenerator>,
        data_root: PathBuf,
    ) -> Self {
        Self {
            store,
            journal,
            host,
            persona_runtime,
            clock,
            ids,
            data_root,
            sessions: Mutex::new(HashMap::new()),
        }
    }

    pub fn create(&self, input: CreateIdentity) -> Result<IdentitySnapshot, ControlError> {
        let mut issues = Vec::new();
        let name = match normalize_identity_name(&input.name) {
            Ok(name) => Some(name),
            Err(issue) => {
                issues.push(issue);
                None
            }
        };
        let startup_url = match normalize_startup_url(input.startup_url.as_deref()) {
            Ok(url) => Some(url),
            Err(issue) => {
                issues.push(issue);
                None
            }
        };
        if !issues.is_empty() {
            return Err(ControlError::Validation(issues));
        }

        let id = self.ids.next_identity_id();
        let profile_root =
            prepare_profile_root(&self.data_root, &id.to_string()).map_err(map_profile_error)?;
        let now_ms = self.clock.now_ms();
        let identity = BrowserIdentity {
            id,
            revision: 1,
            name: name.expect("validated name is present"),
            startup_url: startup_url.expect("validated URL result is present"),
            persona: PersonaConfig::native(id.persona_seed()),
            network: NetworkConfig::direct(),
            lifecycle: IdentityLifecycle::Active,
            profile_root,
            created_at_ms: now_ms,
            updated_at_ms: now_ms,
        };
        self.store.insert(&identity).map_err(map_store_error)?;
        Ok(snapshot(identity, None))
    }

    pub fn list(&self) -> Result<Vec<IdentitySnapshot>, ControlError> {
        let identities = self
            .store
            .list_by_lifecycle(IdentityLifecycle::Active)
            .map_err(map_store_error)?;
        let mut sessions = self.sessions.lock();
        let stopped = sessions
            .keys()
            .copied()
            .filter(|id| !self.host.is_running(*id))
            .collect::<Vec<_>>();
        for id in stopped {
            sessions.remove(&id);
            self.journal.remove(id).map_err(map_store_error)?;
        }
        Ok(identities
            .into_iter()
            .map(|identity| {
                let runtime = sessions.get(&identity.id);
                snapshot(identity, runtime)
            })
            .collect())
    }

    pub fn list_archived(&self) -> Result<Vec<IdentitySnapshot>, ControlError> {
        self.store
            .list_by_lifecycle(IdentityLifecycle::Archived)
            .map_err(map_store_error)
            .map(|identities| {
                identities
                    .into_iter()
                    .map(|identity| snapshot(identity, None))
                    .collect()
            })
    }

    pub fn start(&self, id: IdentityId) -> Result<IdentitySnapshot, ControlError> {
        let identity = self.store.get(id).map_err(map_store_error)?;
        if identity.lifecycle != IdentityLifecycle::Active {
            return Err(ControlError::NotFound);
        }
        validate_identity_coherence(&identity)?;

        let mut sessions = self.sessions.lock();
        if sessions.contains_key(&id) || self.host.is_running(id) {
            return Err(ControlError::AlreadyRunning);
        }

        let lease = ProfileLease::acquire(
            &self.data_root,
            &identity.profile_root,
            &identity.id.to_string(),
        )
        .map_err(map_profile_error)?;

        let browser = self
            .host
            .detect_product_kernel()
            .map_err(ControlError::BrowserUnavailable)?;
        let mut compiled_persona = identity.persona.compile().map_err(map_persona_error)?;
        let kernel_persona = identity
            .persona
            .compile_kernel_persona(&identity.id.to_string(), browser.engine_major);
        compiled_persona.runtime.observe_canvas = true;
        let persona_file = write_kernel_persona(&identity.profile_root, &kernel_persona)?;
        let compiled_network = identity
            .network
            .compile_chromium()
            .map_err(map_network_error)?;
        let requested_startup_url = identity
            .startup_url
            .clone()
            .unwrap_or_else(|| "about:blank".to_owned());
        let mut arguments = compiled_persona.rendered_arguments();
        arguments.push(format!(
            "--realbrowser-persona-file={}",
            persona_file.display()
        ));
        let runtime_request = PersonaRuntimeRequest {
            profile_root: &identity.profile_root,
            plan: compiled_persona.runtime,
            requested_startup_url: &requested_startup_url,
            navigation: PersonaRuntimeNavigation::Navigate,
        };
        let runtime_launch = self
            .persona_runtime
            .prepare(&runtime_request)
            .map_err(|error| ControlError::PersonaUnavailable(error.to_string()))?;
        arguments.extend(compiled_network.rendered_arguments());
        arguments.extend(runtime_launch.chrome_arguments().iter().cloned());
        let plan = LaunchPlan {
            identity_id: id,
            executable: browser.executable,
            profile_root: identity.profile_root.clone(),
            startup_url: runtime_launch.browser_startup_url().to_owned(),
            browser_version: browser.version,
            arguments,
            environment: Vec::new(),
        };
        let session = self
            .host
            .start(&plan, self.clock.now_ms())
            .map_err(ControlError::Runtime)?;
        let active_persona_runtime = match self.persona_runtime.attach(&runtime_request) {
            Ok(runtime) => runtime,
            Err(error) => {
                let _ = self.host.stop(id);
                return Err(ControlError::PersonaUnavailable(error.to_string()));
            }
        };
        let runtime = BrowserRuntimeRecord {
            identity_id: id,
            pid: session.pid,
            executable: plan.executable.clone(),
            profile_root: plan.profile_root.clone(),
            browser_version: session.browser_version.clone(),
            started_at_ms: session.started_at_ms,
        };
        if let Err(error) = self.journal.upsert(&runtime) {
            let _ = self.host.stop(id);
            return Err(map_store_error(error));
        }
        sessions.insert(
            id,
            ActiveRuntime {
                session: session.clone(),
                persona_runtime: active_persona_runtime,
                _lease: lease,
            },
        );
        Ok(snapshot(identity, sessions.get(&id)))
    }

    pub fn stop(&self, id: IdentityId) -> Result<StopResult, ControlError> {
        let identity = self.store.get(id).map_err(map_store_error)?;
        if let Some(active) = self.sessions.lock().get_mut(&id)
            && let Some(runtime) = active.persona_runtime.as_mut()
        {
            let _ = runtime.shutdown();
        }
        let termination = self.host.stop(id).map_err(ControlError::Runtime)?;
        self.sessions.lock().remove(&id);
        self.journal.remove(id).map_err(map_store_error)?;
        Ok(StopResult {
            identity: snapshot(identity, None),
            termination,
        })
    }

    pub fn rename(
        &self,
        id: IdentityId,
        expected_revision: u64,
        name: &str,
    ) -> Result<IdentitySnapshot, ControlError> {
        let name =
            normalize_identity_name(name).map_err(|issue| ControlError::Validation(vec![issue]))?;
        let mut identity = self.store.get(id).map_err(map_store_error)?;
        identity.name = name;
        identity.revision = expected_revision.saturating_add(1);
        identity.updated_at_ms = self.clock.now_ms();
        self.store
            .replace(&identity, expected_revision)
            .map_err(map_store_error)?;
        let sessions = self.sessions.lock();
        Ok(snapshot(identity, sessions.get(&id)))
    }

    pub fn update_persona(
        &self,
        id: IdentityId,
        expected_revision: u64,
        persona: PersonaConfig,
    ) -> Result<IdentitySnapshot, ControlError> {
        if self.host.is_running(id) || self.sessions.lock().contains_key(&id) {
            return Err(ControlError::StillRunning);
        }
        let persona = persona.migrate().map_err(map_persona_error)?;
        if let Err(PersonaError::Invalid(issues)) = persona.compile() {
            return Err(ControlError::Validation(
                issues
                    .into_iter()
                    .map(|issue| FieldIssue {
                        field: issue.field,
                        code: issue.code,
                        message: issue.message,
                    })
                    .collect(),
            ));
        }
        let mut identity = self.store.get(id).map_err(map_store_error)?;
        identity.persona = persona;
        identity.revision = expected_revision.saturating_add(1);
        identity.updated_at_ms = self.clock.now_ms();
        self.store
            .replace(&identity, expected_revision)
            .map_err(map_store_error)?;
        Ok(snapshot(identity, None))
    }

    pub fn update_network(
        &self,
        id: IdentityId,
        expected_revision: u64,
        network: NetworkConfig,
    ) -> Result<IdentitySnapshot, ControlError> {
        if self.host.is_running(id) || self.sessions.lock().contains_key(&id) {
            return Err(ControlError::StillRunning);
        }
        let network = network.normalized().map_err(map_network_error)?;
        network.compile_chromium().map_err(map_network_error)?;
        let mut identity = self.store.get(id).map_err(map_store_error)?;
        identity.network = network;
        identity.revision = expected_revision.saturating_add(1);
        identity.updated_at_ms = self.clock.now_ms();
        self.store
            .replace(&identity, expected_revision)
            .map_err(map_store_error)?;
        Ok(snapshot(identity, None))
    }

    pub fn archive(&self, id: IdentityId, expected_revision: u64) -> Result<(), ControlError> {
        if self.host.is_running(id) || self.sessions.lock().contains_key(&id) {
            return Err(ControlError::StillRunning);
        }
        let mut identity = self.store.get(id).map_err(map_store_error)?;
        identity.lifecycle = IdentityLifecycle::Archived;
        identity.revision = expected_revision.saturating_add(1);
        identity.updated_at_ms = self.clock.now_ms();
        self.store
            .replace(&identity, expected_revision)
            .map_err(map_store_error)
    }

    pub fn restore(
        &self,
        id: IdentityId,
        expected_revision: u64,
    ) -> Result<IdentitySnapshot, ControlError> {
        let mut identity = self.store.get(id).map_err(map_store_error)?;
        if identity.lifecycle != IdentityLifecycle::Archived {
            return Err(ControlError::RevisionConflict);
        }
        prepare_profile_root(&self.data_root, &id.to_string()).map_err(map_profile_error)?;
        identity.lifecycle = IdentityLifecycle::Active;
        identity.revision = expected_revision.saturating_add(1);
        identity.updated_at_ms = self.clock.now_ms();
        self.store
            .replace(&identity, expected_revision)
            .map_err(map_store_error)?;
        Ok(snapshot(identity, None))
    }

    pub fn reconcile(&self) -> Result<ReconcileReport, ControlError> {
        let runtimes = self.journal.list().map_err(map_store_error)?;
        let mut report = ReconcileReport::default();
        let mut sessions = self.sessions.lock();
        for runtime in runtimes {
            let identity = match self.store.get(runtime.identity_id) {
                Ok(identity) if identity.lifecycle == IdentityLifecycle::Active => identity,
                Ok(_) | Err(StoreError::NotFound) => {
                    self.journal
                        .remove(runtime.identity_id)
                        .map_err(map_store_error)?;
                    report.cleared = report.cleared.saturating_add(1);
                    continue;
                }
                Err(error) => return Err(map_store_error(error)),
            };
            let lease = match ProfileLease::acquire(
                &self.data_root,
                &identity.profile_root,
                &identity.id.to_string(),
            ) {
                Ok(lease) => lease,
                Err(ProfileLeaseError::AlreadyOwned) => {
                    report.owned_elsewhere = report.owned_elsewhere.saturating_add(1);
                    continue;
                }
                Err(error) => return Err(map_profile_error(error)),
            };
            match self.host.reconcile(&runtime) {
                Ok(Some(session)) => {
                    let mut compiled_persona =
                        identity.persona.compile().map_err(map_persona_error)?;
                    compiled_persona.runtime.observe_canvas = true;
                    let requested_startup_url =
                        identity.startup_url.as_deref().unwrap_or("about:blank");
                    let runtime_request = PersonaRuntimeRequest {
                        profile_root: &identity.profile_root,
                        plan: compiled_persona.runtime,
                        requested_startup_url,
                        navigation: PersonaRuntimeNavigation::Preserve,
                    };
                    let active_persona_runtime = match self.persona_runtime.attach(&runtime_request)
                    {
                        Ok(runtime) => runtime,
                        Err(_) => {
                            self.host.stop(identity.id).map_err(ControlError::Runtime)?;
                            self.journal
                                .remove(runtime.identity_id)
                                .map_err(map_store_error)?;
                            report.persona_failed = report.persona_failed.saturating_add(1);
                            continue;
                        }
                    };
                    sessions.insert(
                        identity.id,
                        ActiveRuntime {
                            session,
                            persona_runtime: active_persona_runtime,
                            _lease: lease,
                        },
                    );
                    report.recovered = report.recovered.saturating_add(1);
                }
                Ok(None) => {
                    self.journal
                        .remove(runtime.identity_id)
                        .map_err(map_store_error)?;
                    report.cleared = report.cleared.saturating_add(1);
                }
                Err(_) => {
                    self.journal
                        .remove(runtime.identity_id)
                        .map_err(map_store_error)?;
                    report.unverified = report.unverified.saturating_add(1);
                }
            }
        }
        Ok(report)
    }

    #[must_use]
    pub fn system_snapshot(&self) -> SystemSnapshot {
        let canvas_kernel_observed = self.sessions.lock().values().any(|runtime| {
            runtime
                .persona_runtime
                .as_ref()
                .is_some_and(|persona_runtime| {
                    persona_runtime
                        .observation()
                        .fields
                        .iter()
                        .any(|field| field.field == "graphics.canvas" && field.matches)
                })
        });
        SystemSnapshot {
            platform: std::env::consts::OS.to_owned(),
            browser: self.host.detect_product_kernel().ok(),
            canvas_kernel_observed,
        }
    }
}

fn write_kernel_persona(
    profile_root: &Path,
    persona: &browser_persona::KernelPersonaConfig,
) -> Result<PathBuf, ControlError> {
    let directory = profile_root.join(".realbrowser");
    std::fs::create_dir_all(&directory)
        .map_err(|error| ControlError::Filesystem(format!("无法创建内核 Persona 目录：{error}")))?;
    let directory = directory
        .canonicalize()
        .map_err(|error| ControlError::Filesystem(format!("无法校验内核 Persona 目录：{error}")))?;
    let path = directory.join("persona.json");
    let nonce = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_nanos();
    let temporary = directory.join(format!("persona.json.tmp-{}-{nonce}", std::process::id()));
    if std::fs::symlink_metadata(&path).is_ok_and(|metadata| metadata.file_type().is_symlink()) {
        return Err(ControlError::Filesystem(
            "内核 Persona 文件不能是符号链接".to_owned(),
        ));
    }
    let contents = serde_json::to_vec_pretty(persona)
        .map_err(|error| ControlError::Filesystem(format!("无法序列化内核 Persona：{error}")))?;
    let mut options = OpenOptions::new();
    options.create_new(true).write(true);
    #[cfg(unix)]
    {
        use std::os::unix::fs::OpenOptionsExt;
        options.mode(0o600);
    }
    let mut file = options.open(&temporary).map_err(|error| {
        ControlError::Filesystem(format!("无法写入内核 Persona 临时文件：{error}"))
    })?;
    file.write_all(&contents)
        .and_then(|_| file.write_all(b"\n"))
        .and_then(|_| file.sync_all())
        .map_err(|error| ControlError::Filesystem(format!("无法持久化内核 Persona：{error}")))?;
    std::fs::rename(&temporary, &path)
        .map_err(|error| ControlError::Filesystem(format!("无法发布内核 Persona：{error}")))?;
    Ok(path)
}

#[derive(Clone, Copy, Debug, Default, Eq, PartialEq)]
pub struct ReconcileReport {
    pub recovered: u32,
    pub cleared: u32,
    pub owned_elsewhere: u32,
    pub unverified: u32,
    pub persona_failed: u32,
}

fn snapshot(identity: BrowserIdentity, runtime: Option<&ActiveRuntime>) -> IdentitySnapshot {
    let egress = match identity.network.mode {
        NetworkMode::Direct => EgressMode::Direct,
        NetworkMode::Proxy => EgressMode::Proxy,
    };
    IdentitySnapshot {
        runtime_state: if runtime.is_some() {
            RuntimeState::Running
        } else {
            RuntimeState::Stopped
        },
        persona: identity.persona.clone(),
        persona_observation: runtime
            .and_then(|runtime| runtime.persona_runtime.as_ref())
            .map(|runtime| runtime.observation().clone()),
        identity,
        session: runtime.map(|runtime| runtime.session.clone()),
        egress,
    }
}

fn validate_identity_coherence(identity: &BrowserIdentity) -> Result<(), ControlError> {
    if identity.network.mode == NetworkMode::Proxy
        && identity.persona.webrtc != WebRtcPolicy::DisableNonProxiedUdp
    {
        return Err(ControlError::Validation(vec![FieldIssue {
            field: "persona.webrtc",
            code: "proxy_requires_non_proxied_udp",
            message: "代理模式需将 WebRTC 设为禁止直连 UDP".to_owned(),
        }]));
    }
    Ok(())
}

fn map_persona_error(error: PersonaError) -> ControlError {
    match error {
        PersonaError::Invalid(issues) => ControlError::Validation(
            issues
                .into_iter()
                .map(|issue| FieldIssue {
                    field: issue.field,
                    code: issue.code,
                    message: issue.message,
                })
                .collect(),
        ),
    }
}

fn map_network_error(error: NetworkError) -> ControlError {
    match error {
        NetworkError::Invalid(issues) => ControlError::Validation(
            issues
                .into_iter()
                .map(|issue| FieldIssue {
                    field: issue.field,
                    code: issue.code,
                    message: issue.message,
                })
                .collect(),
        ),
    }
}

fn map_store_error(error: StoreError) -> ControlError {
    match error {
        StoreError::NotFound => ControlError::NotFound,
        StoreError::RevisionConflict => ControlError::RevisionConflict,
        StoreError::Backend(message) => ControlError::Storage(message.to_string()),
    }
}

fn map_profile_error(error: ProfileLeaseError) -> ControlError {
    match error {
        ProfileLeaseError::AlreadyOwned => ControlError::AlreadyRunning,
        other => ControlError::ProfileUnavailable(other.to_string()),
    }
}

pub struct SystemClock;

impl Clock for SystemClock {
    fn now_ms(&self) -> i64 {
        SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map_or(0, |duration| duration.as_millis() as i64)
    }
}

pub struct UuidGenerator;

impl IdGenerator for UuidGenerator {
    fn next_identity_id(&self) -> IdentityId {
        IdentityId::new(Uuid::new_v4())
    }
}

pub fn ensure_data_root(root: &Path) -> Result<PathBuf, ControlError> {
    std::fs::create_dir_all(root).map_err(|error| ControlError::Filesystem(error.to_string()))?;
    root.canonicalize()
        .map_err(|error| ControlError::Filesystem(error.to_string()))
}

#[cfg(test)]
mod tests;
