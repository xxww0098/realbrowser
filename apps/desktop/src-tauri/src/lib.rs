#![forbid(unsafe_code)]

use std::sync::Arc;

use browser_control::{ControlError, ControlService, SystemClock, UuidGenerator, ensure_data_root};
use browser_core::{
    CreateIdentity, EgressMode, FieldIssue, IdentityId, IdentitySnapshot, RuntimeState, StopResult,
    SystemSnapshot, TerminationKind,
};
use browser_network::NetworkConfig;
use browser_persona::{
    ApplicationConfidence, Coverage, ExecutionBackend, PersonaCapability, PersonaConfig,
    PersonaGroup, PersonaMode, PersonaObservation, observed_persona_capabilities, timezone_catalog,
};
use browser_persona_runtime::CdpPersonaRuntime;
use browser_platform::SystemBrowserHost;
use browser_storage::SqliteIdentityStore;
use serde::{Deserialize, Serialize};
use tauri::{Manager, State};
use tracing::info;

struct DesktopState {
    control: ControlService,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct CreateIdentityInput {
    name: String,
    startup_url: Option<String>,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct IdentityView {
    id: String,
    display_code: String,
    revision: u64,
    name: String,
    startup_url: Option<String>,
    profile_mode: &'static str,
    runtime_state: &'static str,
    persona_mode: &'static str,
    persona: PersonaConfig,
    persona_observation: Option<PersonaObservationView>,
    network: NetworkConfig,
    egress_mode: &'static str,
    browser_version: Option<String>,
    created_at_ms: i64,
    updated_at_ms: i64,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct SystemView {
    platform: String,
    browser_version: Option<String>,
    default_profile_mode: &'static str,
    default_persona_mode: &'static str,
    default_egress_mode: &'static str,
    persona_runtime_available: bool,
    persona_capabilities: Vec<PersonaCapabilityView>,
    timezone_ids: Vec<String>,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct StopView {
    identity: IdentityView,
    termination: &'static str,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct PersonaCapabilityView {
    field: &'static str,
    group: &'static str,
    backend: &'static str,
    confidence: &'static str,
    coverage: Vec<&'static str>,
    editable: bool,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct PersonaObservationView {
    matches: bool,
    fields: Vec<ObservedPersonaFieldView>,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct ObservedPersonaFieldView {
    field: String,
    expected: String,
    observed: String,
    matches: bool,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct FieldIssueView {
    field: &'static str,
    code: &'static str,
    message: String,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct CommandFailure {
    code: &'static str,
    message: String,
    field_issues: Vec<FieldIssueView>,
}

impl From<ControlError> for CommandFailure {
    fn from(error: ControlError) -> Self {
        let code = error.code();
        let message = error.public_message().to_owned();
        let field_issues = error
            .field_issues()
            .iter()
            .cloned()
            .map(FieldIssueView::from)
            .collect();
        Self {
            code,
            message,
            field_issues,
        }
    }
}

impl From<FieldIssue> for FieldIssueView {
    fn from(issue: FieldIssue) -> Self {
        Self {
            field: issue.field,
            code: issue.code,
            message: issue.message,
        }
    }
}

#[tauri::command]
fn list_identities(state: State<'_, DesktopState>) -> Result<Vec<IdentityView>, CommandFailure> {
    state
        .control
        .list()
        .map(|records| records.into_iter().map(IdentityView::from).collect())
        .map_err(CommandFailure::from)
}

#[tauri::command]
fn list_archived_identities(
    state: State<'_, DesktopState>,
) -> Result<Vec<IdentityView>, CommandFailure> {
    state
        .control
        .list_archived()
        .map(|records| records.into_iter().map(IdentityView::from).collect())
        .map_err(CommandFailure::from)
}

#[tauri::command]
fn system_status(state: State<'_, DesktopState>) -> SystemView {
    SystemView::from(state.control.system_snapshot())
}

#[tauri::command]
fn create_identity(
    state: State<'_, DesktopState>,
    input: CreateIdentityInput,
) -> Result<IdentityView, CommandFailure> {
    state
        .control
        .create(CreateIdentity {
            name: input.name,
            startup_url: input.startup_url,
        })
        .map(IdentityView::from)
        .map_err(CommandFailure::from)
}

#[tauri::command]
fn start_identity(
    state: State<'_, DesktopState>,
    id: String,
) -> Result<IdentityView, CommandFailure> {
    state
        .control
        .start(parse_identity_id(&id)?)
        .map(IdentityView::from)
        .map_err(CommandFailure::from)
}

#[tauri::command]
fn stop_identity(state: State<'_, DesktopState>, id: String) -> Result<StopView, CommandFailure> {
    state
        .control
        .stop(parse_identity_id(&id)?)
        .map(StopView::from)
        .map_err(CommandFailure::from)
}

#[tauri::command]
fn rename_identity(
    state: State<'_, DesktopState>,
    id: String,
    revision: u64,
    name: String,
) -> Result<IdentityView, CommandFailure> {
    state
        .control
        .rename(parse_identity_id(&id)?, revision, &name)
        .map(IdentityView::from)
        .map_err(CommandFailure::from)
}

#[tauri::command]
fn update_persona(
    state: State<'_, DesktopState>,
    id: String,
    revision: u64,
    persona: PersonaConfig,
) -> Result<IdentityView, CommandFailure> {
    state
        .control
        .update_persona(parse_identity_id(&id)?, revision, persona)
        .map(IdentityView::from)
        .map_err(CommandFailure::from)
}

#[tauri::command]
fn update_network(
    state: State<'_, DesktopState>,
    id: String,
    revision: u64,
    network: NetworkConfig,
) -> Result<IdentityView, CommandFailure> {
    state
        .control
        .update_network(parse_identity_id(&id)?, revision, network)
        .map(IdentityView::from)
        .map_err(CommandFailure::from)
}

#[tauri::command]
fn archive_identity(
    state: State<'_, DesktopState>,
    id: String,
    revision: u64,
) -> Result<(), CommandFailure> {
    state
        .control
        .archive(parse_identity_id(&id)?, revision)
        .map_err(CommandFailure::from)
}

#[tauri::command]
fn restore_identity(
    state: State<'_, DesktopState>,
    id: String,
    revision: u64,
) -> Result<IdentityView, CommandFailure> {
    state
        .control
        .restore(parse_identity_id(&id)?, revision)
        .map(IdentityView::from)
        .map_err(CommandFailure::from)
}

fn parse_identity_id(value: &str) -> Result<IdentityId, CommandFailure> {
    IdentityId::parse(value).map_err(|_| CommandFailure {
        code: "invalid_identity_id",
        message: "浏览器环境编号无效".to_owned(),
        field_issues: Vec::new(),
    })
}

impl From<IdentitySnapshot> for IdentityView {
    fn from(snapshot: IdentitySnapshot) -> Self {
        let persona_mode = snapshot.persona.mode();
        let browser_version = snapshot
            .session
            .as_ref()
            .map(|session| session.browser_version.clone());
        Self {
            id: snapshot.identity.id.to_string(),
            display_code: snapshot.identity.id.display_code(),
            revision: snapshot.identity.revision,
            name: snapshot.identity.name,
            startup_url: snapshot.identity.startup_url,
            profile_mode: "isolated",
            runtime_state: runtime_name(snapshot.runtime_state),
            persona_mode: persona_name(persona_mode),
            persona: snapshot.persona,
            persona_observation: snapshot
                .persona_observation
                .map(PersonaObservationView::from),
            network: snapshot.identity.network,
            egress_mode: egress_name(snapshot.egress),
            browser_version,
            created_at_ms: snapshot.identity.created_at_ms,
            updated_at_ms: snapshot.identity.updated_at_ms,
        }
    }
}

impl From<SystemSnapshot> for SystemView {
    fn from(snapshot: SystemSnapshot) -> Self {
        let browser_version = snapshot.browser.map(|browser| browser.version);
        Self {
            platform: snapshot.platform,
            browser_version,
            default_profile_mode: "isolated",
            default_persona_mode: "native",
            default_egress_mode: "direct",
            persona_runtime_available: true,
            persona_capabilities: observed_persona_capabilities(snapshot.canvas_kernel_observed)
                .into_iter()
                .map(PersonaCapabilityView::from)
                .collect(),
            timezone_ids: timezone_catalog().to_vec(),
        }
    }
}

impl From<PersonaObservation> for PersonaObservationView {
    fn from(observation: PersonaObservation) -> Self {
        let fields = observation
            .fields
            .into_iter()
            .map(|field| ObservedPersonaFieldView {
                field: field.field,
                expected: field.expected,
                observed: field.observed,
                matches: field.matches,
            })
            .collect::<Vec<_>>();
        Self {
            matches: fields.iter().all(|field| field.matches),
            fields,
        }
    }
}

impl From<StopResult> for StopView {
    fn from(result: StopResult) -> Self {
        Self {
            identity: IdentityView::from(result.identity),
            termination: termination_name(result.termination),
        }
    }
}

impl From<PersonaCapability> for PersonaCapabilityView {
    fn from(capability: PersonaCapability) -> Self {
        let coverage = [
            (Coverage::BROWSER, "browser"),
            (Coverage::PROFILE, "profile"),
            (Coverage::PROCESS, "process"),
            (Coverage::TOP_FRAME, "top_frame"),
            (Coverage::ALL_FRAMES, "all_frames"),
            (Coverage::DEDICATED_WORKER, "dedicated_worker"),
            (Coverage::SHARED_WORKER, "shared_worker"),
            (Coverage::SERVICE_WORKER, "service_worker"),
            (Coverage::NETWORK, "network"),
        ]
        .into_iter()
        .filter_map(|(coverage, name)| capability.coverage.contains(coverage).then_some(name))
        .collect();
        Self {
            field: capability.field,
            group: persona_group_name(capability.group),
            backend: execution_backend_name(capability.backend),
            confidence: application_confidence_name(capability.confidence),
            coverage,
            editable: capability.editable,
        }
    }
}

fn runtime_name(state: RuntimeState) -> &'static str {
    match state {
        RuntimeState::Stopped => "stopped",
        RuntimeState::Starting => "starting",
        RuntimeState::Running => "running",
        RuntimeState::Stopping => "stopping",
        RuntimeState::Failed => "failed",
    }
}

fn persona_name(mode: PersonaMode) -> &'static str {
    match mode {
        PersonaMode::Native => "native",
        PersonaMode::Managed => "managed",
    }
}

fn persona_group_name(group: PersonaGroup) -> &'static str {
    match group {
        PersonaGroup::Region => "region",
        PersonaGroup::Device => "device",
        PersonaGroup::Graphics => "graphics",
        PersonaGroup::Media => "media",
        PersonaGroup::Privacy => "privacy",
    }
}

fn execution_backend_name(backend: ExecutionBackend) -> &'static str {
    match backend {
        ExecutionBackend::Native => "native",
        ExecutionBackend::Profile => "profile",
        ExecutionBackend::LaunchArgument => "launch_argument",
        ExecutionBackend::Cdp => "cdp",
        ExecutionBackend::ExtensionLimited => "extension_limited",
        ExecutionBackend::CustomKernel => "custom_kernel",
        ExecutionBackend::Unavailable => "unavailable",
    }
}

fn application_confidence_name(confidence: ApplicationConfidence) -> &'static str {
    match confidence {
        ApplicationConfidence::Native => "native",
        ApplicationConfidence::MappedUnverified => "mapped_unverified",
        ApplicationConfidence::Observed => "observed",
        ApplicationConfidence::NotApplied => "not_applied",
    }
}

fn egress_name(mode: EgressMode) -> &'static str {
    match mode {
        EgressMode::Direct => "direct",
        EgressMode::Proxy => "proxy",
    }
}

fn termination_name(kind: TerminationKind) -> &'static str {
    match kind {
        TerminationKind::AlreadyExited => "already_exited",
        TerminationKind::Graceful => "graceful",
        TerminationKind::Forced => "forced",
    }
}

pub fn run() {
    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| tracing_subscriber::EnvFilter::new("info")),
        )
        .with_target(false)
        .compact()
        .init();

    tauri::Builder::default()
        .setup(|app| {
            let default_data_root = app.path().app_data_dir()?;
            #[cfg(debug_assertions)]
            let requested_data_root = std::env::var_os("REALBROWSER_DEV_DATA_ROOT")
                .map(std::path::PathBuf::from)
                .unwrap_or(default_data_root);
            #[cfg(not(debug_assertions))]
            let requested_data_root = default_data_root;
            let data_root = ensure_data_root(&requested_data_root)?;
            let store = Arc::new(SqliteIdentityStore::open(&data_root.join("state.sqlite3"))?);
            let control = ControlService::new(
                store.clone(),
                store,
                Arc::new(SystemBrowserHost::new()),
                Arc::new(CdpPersonaRuntime::new()),
                Arc::new(SystemClock),
                Arc::new(UuidGenerator),
                data_root.clone(),
            );
            let reconciliation = control.reconcile()?;
            app.manage(DesktopState { control });
            info!(
                data_root = %data_root.display(),
                recovered = reconciliation.recovered,
                cleared = reconciliation.cleared,
                owned_elsewhere = reconciliation.owned_elsewhere,
                unverified = reconciliation.unverified,
                "desktop control plane initialized"
            );
            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            list_identities,
            list_archived_identities,
            system_status,
            create_identity,
            start_identity,
            stop_identity,
            rename_identity,
            update_persona,
            update_network,
            archive_identity,
            restore_identity,
        ])
        .run(tauri::generate_context!())
        .expect("Tauri desktop runtime failed");
}

#[cfg(test)]
mod tests {
    use std::path::PathBuf;

    use browser_core::{
        BrowserIdentity, BrowserInstall, BrowserSession, EgressMode, IdentityId, IdentityLifecycle,
        IdentitySnapshot, RuntimeState, SystemSnapshot,
    };
    use browser_network::NetworkConfig;
    use browser_persona::PersonaConfig;
    use uuid::Uuid;

    use super::{CommandFailure, IdentityView, SystemView};

    #[test]
    fn identity_ipc_contract_omits_profile_paths_and_process_ids() {
        let id = IdentityId::new(Uuid::parse_str("3b0b9093-6cf7-4d1d-aa51-d6876128abbe").unwrap());
        let persona = PersonaConfig::native(id.persona_seed());
        let view = IdentityView::from(IdentitySnapshot {
            identity: BrowserIdentity {
                id,
                revision: 4,
                name: "Store A".to_owned(),
                startup_url: Some("https://seller.example".to_owned()),
                persona: persona.clone(),
                network: NetworkConfig::direct(),
                lifecycle: IdentityLifecycle::Active,
                profile_root: PathBuf::from("/Users/operator/secret/profile"),
                created_at_ms: 1,
                updated_at_ms: 2,
            },
            runtime_state: RuntimeState::Running,
            session: Some(BrowserSession {
                identity_id: id,
                pid: 42_424,
                browser_version: "151.0.0.0".to_owned(),
                started_at_ms: 2,
            }),
            persona,
            persona_observation: None,
            egress: EgressMode::Direct,
        });

        let value = serde_json::to_value(view).unwrap();
        assert_eq!(value["profileMode"], "isolated");
        assert_eq!(value["network"]["mode"], "direct");
        assert!(value.get("profileRoot").is_none());
        assert!(value["network"].get("username").is_none());
        assert!(value["network"].get("password").is_none());
        assert!(value.get("pid").is_none());
        assert!(!value.to_string().contains("/Users/operator"));
    }

    #[test]
    fn system_ipc_contract_reports_capability_without_local_paths() {
        let view = SystemView::from(SystemSnapshot {
            platform: "macos".to_owned(),
            browser: Some(BrowserInstall {
                executable: PathBuf::from(
                    "/Applications/RealBrowser.app/Contents/Resources/realbrowser-kernel/RealBrowser.app/Contents/MacOS/RealBrowser",
                ),
                version: "151.0.0.0".to_owned(),
                engine_major: 151,
            }),
            canvas_kernel_observed: false,
        });

        let value = serde_json::to_value(view).unwrap();
        assert_eq!(value["browserVersion"], "151.0.0.0");
        assert_eq!(value["defaultProfileMode"], "isolated");
        assert_eq!(value["defaultPersonaMode"], "native");
        assert_eq!(value["defaultEgressMode"], "direct");
        assert_eq!(value["personaRuntimeAvailable"], true);
        assert!(
            value["timezoneIds"]
                .as_array()
                .unwrap()
                .iter()
                .any(|timezone| timezone == "Asia/Tokyo")
        );
        assert!(value["personaCapabilities"].as_array().unwrap().iter().any(
            |capability| capability["field"] == "region.timezone"
                && capability["backend"] == "cdp"
                && capability["confidence"] == "observed"
                && capability["editable"] == true
        ));
        assert!(value["personaCapabilities"].as_array().unwrap().iter().any(
            |capability| capability["field"] == "display.viewport"
                && capability["backend"] == "cdp"
                && capability["confidence"] == "observed"
                && capability["editable"] == true
        ));
        assert!(value["personaCapabilities"].as_array().unwrap().iter().any(
            |capability| capability["field"] == "graphics.canvas"
                && capability["backend"] == "native"
                && capability["confidence"] == "native"
        ));
        assert!(value.get("browserPath").is_none());
        assert!(value.get("dataRoot").is_none());
        assert!(!value.to_string().contains("/Applications"));

        let observed = serde_json::to_value(SystemView::from(SystemSnapshot {
            platform: "macos".to_owned(),
            browser: None,
            canvas_kernel_observed: true,
        }))
        .unwrap();
        assert!(
            observed["personaCapabilities"]
                .as_array()
                .unwrap()
                .iter()
                .any(|capability| capability["field"] == "graphics.canvas"
                    && capability["backend"] == "custom_kernel"
                    && capability["confidence"] == "observed"
                    && capability["coverage"]
                        == serde_json::json!(["all_frames", "dedicated_worker"]))
        );
    }

    #[test]
    fn command_failure_ipc_contract_redacts_internal_diagnostics() {
        let failure = CommandFailure::from(browser_control::ControlError::Runtime(
            "spawn failed at /Users/operator/secret/profile token=private".to_owned(),
        ));

        let value = serde_json::to_value(failure).unwrap();
        assert_eq!(value["code"], "runtime_failed");
        assert_eq!(value["message"], "浏览器操作失败，请重试");
        assert!(!value.to_string().contains("/Users/operator"));
        assert!(!value.to_string().contains("token=private"));
    }
}
