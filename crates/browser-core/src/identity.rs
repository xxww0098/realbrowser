use std::path::PathBuf;

use browser_network::NetworkConfig;
use browser_persona::{PersonaConfig, PersonaObservation};
use thiserror::Error;
use url::Url;
use uuid::Uuid;

#[derive(Clone, Copy, Debug, Eq, Hash, PartialEq)]
pub struct IdentityId(Uuid);

impl IdentityId {
    #[must_use]
    pub fn new(value: Uuid) -> Self {
        Self(value)
    }

    pub fn parse(value: &str) -> Result<Self, uuid::Error> {
        Uuid::parse_str(value).map(Self)
    }

    #[must_use]
    pub fn persona_seed(self) -> u32 {
        let [a, b, c, d, ..] = *self.0.as_bytes();
        u32::from_be_bytes([a, b, c, d])
    }

    /// Stable, non-secret operator code. This is derived from the immutable
    /// identity UUID so list sorting, renames, and archive/restore never change it.
    #[must_use]
    pub fn display_code(self) -> String {
        format!("RB-{:08X}", self.persona_seed())
    }
}

impl std::fmt::Display for IdentityId {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        self.0.fmt(formatter)
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum IdentityLifecycle {
    Active,
    Archived,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum EgressMode {
    Direct,
    Proxy,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum RuntimeState {
    Stopped,
    Starting,
    Running,
    Stopping,
    Failed,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum TerminationKind {
    AlreadyExited,
    Graceful,
    Forced,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct BrowserIdentity {
    pub id: IdentityId,
    pub revision: u64,
    pub name: String,
    pub startup_url: Option<String>,
    pub persona: PersonaConfig,
    pub network: NetworkConfig,
    pub lifecycle: IdentityLifecycle,
    pub profile_root: PathBuf,
    pub created_at_ms: i64,
    pub updated_at_ms: i64,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CreateIdentity {
    pub name: String,
    pub startup_url: Option<String>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct BrowserInstall {
    pub executable: PathBuf,
    pub version: String,
    pub engine_major: u16,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct LaunchPlan {
    pub identity_id: IdentityId,
    pub executable: PathBuf,
    pub profile_root: PathBuf,
    pub startup_url: String,
    pub browser_version: String,
    pub arguments: Vec<String>,
    pub environment: Vec<(String, String)>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct BrowserRuntimeRecord {
    pub identity_id: IdentityId,
    pub pid: u32,
    pub executable: PathBuf,
    pub profile_root: PathBuf,
    pub browser_version: String,
    pub started_at_ms: i64,
}

impl BrowserRuntimeRecord {
    #[must_use]
    pub fn session(&self) -> BrowserSession {
        BrowserSession {
            identity_id: self.identity_id,
            pid: self.pid,
            browser_version: self.browser_version.clone(),
            started_at_ms: self.started_at_ms,
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct BrowserSession {
    pub identity_id: IdentityId,
    pub pid: u32,
    pub browser_version: String,
    pub started_at_ms: i64,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct IdentitySnapshot {
    pub identity: BrowserIdentity,
    pub runtime_state: RuntimeState,
    pub session: Option<BrowserSession>,
    pub persona: PersonaConfig,
    pub persona_observation: Option<PersonaObservation>,
    pub egress: EgressMode,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct StopResult {
    pub identity: IdentitySnapshot,
    pub termination: TerminationKind,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SystemSnapshot {
    pub platform: String,
    pub browser: Option<BrowserInstall>,
    pub canvas_kernel_observed: bool,
}

#[derive(Clone, Debug, Eq, Error, PartialEq)]
#[error("{message}")]
pub struct FieldIssue {
    pub field: &'static str,
    pub code: &'static str,
    pub message: String,
}

pub fn normalize_identity_name(value: &str) -> Result<String, FieldIssue> {
    let normalized = value.trim();
    let length = normalized.chars().count();
    if length == 0 {
        return Err(FieldIssue {
            field: "name",
            code: "required",
            message: "请输入环境名称".to_owned(),
        });
    }
    if length > 80 {
        return Err(FieldIssue {
            field: "name",
            code: "too_long",
            message: "环境名称不能超过 80 个字符".to_owned(),
        });
    }
    Ok(normalized.to_owned())
}

pub fn normalize_startup_url(value: Option<&str>) -> Result<Option<String>, FieldIssue> {
    let Some(value) = value.map(str::trim).filter(|value| !value.is_empty()) else {
        return Ok(None);
    };

    let parsed = Url::parse(value).map_err(|_| FieldIssue {
        field: "startupUrl",
        code: "invalid_url",
        message: "请输入完整的 http:// 或 https:// 地址".to_owned(),
    })?;
    if !matches!(parsed.scheme(), "http" | "https") {
        return Err(FieldIssue {
            field: "startupUrl",
            code: "unsupported_scheme",
            message: "启动页只支持 http:// 或 https:// 地址".to_owned(),
        });
    }
    Ok(Some(parsed.into()))
}

#[cfg(test)]
mod tests {
    use super::{IdentityId, normalize_identity_name, normalize_startup_url};
    use uuid::Uuid;

    #[test]
    fn derives_a_stable_operator_code_from_the_identity() {
        let id = IdentityId::new(Uuid::parse_str("3b0b9093-6cf7-4d1d-aa51-d6876128abbe").unwrap());

        assert_eq!(id.display_code(), "RB-3B0B9093");
        assert_eq!(id.display_code(), id.display_code());
    }

    #[test]
    fn normalizes_human_input() {
        assert_eq!(normalize_identity_name("  Store A  ").unwrap(), "Store A");
        assert_eq!(
            normalize_startup_url(Some(" https://seller.example/path ")).unwrap(),
            Some("https://seller.example/path".to_owned())
        );
    }

    #[test]
    fn rejects_non_web_startup_url() {
        let issue = normalize_startup_url(Some("file:///tmp/profile")).unwrap_err();
        assert_eq!(issue.code, "unsupported_scheme");
    }
}
