use browser_core::{
    EgressMode, FieldIssue, IdentityId, IdentitySnapshot, RuntimeState, StopResult, SystemSnapshot,
    TerminationKind,
};
use browser_network::NetworkConfig;
use browser_persona::{
    ApplicationConfidence, Coverage, ExecutionBackend, PersonaCapability, PersonaConfig,
    PersonaGroup, PersonaMode, PersonaObservation, observed_persona_capabilities, timezone_catalog,
};
use serde::Serialize;

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct IdentityView {
    pub id: String,
    pub display_code: String,
    pub revision: u64,
    pub name: String,
    pub startup_url: Option<String>,
    pub profile_mode: &'static str,
    pub runtime_state: &'static str,
    pub persona_mode: &'static str,
    pub persona: PersonaConfig,
    pub persona_observation: Option<PersonaObservationView>,
    pub network: NetworkConfig,
    pub egress_mode: &'static str,
    pub browser_version: Option<String>,
    pub created_at_ms: i64,
    pub updated_at_ms: i64,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct SystemView {
    pub platform: String,
    pub browser_version: Option<String>,
    pub default_profile_mode: &'static str,
    pub default_persona_mode: &'static str,
    pub default_egress_mode: &'static str,
    pub persona_runtime_available: bool,
    pub persona_capabilities: Vec<PersonaCapabilityView>,
    pub timezone_ids: Vec<String>,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct StopView {
    pub identity: IdentityView,
    pub termination: &'static str,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct PersonaCapabilityView {
    pub field: &'static str,
    pub group: &'static str,
    pub backend: &'static str,
    pub confidence: &'static str,
    pub coverage: Vec<&'static str>,
    pub editable: bool,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct PersonaObservationView {
    pub matches: bool,
    pub fields: Vec<ObservedPersonaFieldView>,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ObservedPersonaFieldView {
    pub field: String,
    pub expected: String,
    pub observed: String,
    pub matches: bool,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct FieldIssueView {
    pub field: &'static str,
    pub code: &'static str,
    pub message: String,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct CommandFailure {
    pub code: &'static str,
    pub message: String,
    pub field_issues: Vec<FieldIssueView>,
}

impl From<browser_control::ControlError> for CommandFailure {
    fn from(error: browser_control::ControlError) -> Self {
        Self {
            code: error.code(),
            message: error.public_message().to_owned(),
            field_issues: error
                .field_issues()
                .iter()
                .cloned()
                .map(FieldIssueView::from)
                .collect(),
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

pub fn parse_identity_id(value: &str) -> Result<IdentityId, CommandFailure> {
    IdentityId::parse(value).map_err(|_| CommandFailure {
        code: "invalid_identity_id",
        message: "浏览器环境编号无效".to_owned(),
        field_issues: Vec::new(),
    })
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

    use super::{IdentityView, SystemView};

    #[test]
    fn identity_projection_omits_profile_paths_and_process_ids() {
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
    fn system_projection_reports_capability_without_local_paths() {
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
        assert!(value.get("browserPath").is_none());
        assert!(value.get("dataRoot").is_none());
        assert!(!value.to_string().contains("/Applications"));
    }

    #[test]
    fn command_failure_redacts_internal_diagnostics() {
        let failure = super::CommandFailure::from(browser_control::ControlError::Runtime(
            "spawn failed at /Users/operator/secret/profile token=private".to_owned(),
        ));

        let value = serde_json::to_value(failure).unwrap();
        assert_eq!(value["code"], "runtime_failed");
        assert_eq!(value["message"], "浏览器操作失败，请重试");
        assert!(!value.to_string().contains("/Users/operator"));
        assert!(!value.to_string().contains("token=private"));
    }
}
