use std::{collections::HashMap, path::PathBuf, sync::Arc};

use browser_core::{
    BrowserHost, BrowserIdentity, BrowserInstall, BrowserRuntimeRecord, BrowserSession, Clock,
    CreateIdentity, IdGenerator, IdentityId, IdentityStore, LaunchPlan, RuntimeJournal, StoreError,
    TerminationKind,
};
use browser_network::{NetworkConfig, NetworkMode, ProxyConfig, ProxyProtocol};
use browser_persona::{
    DisplayMetrics, LocalePolicy, ObservedField, PersonaConfig, PersonaObservation, RegionPreset,
    TimezonePolicy, WebRtcPolicy,
};
use browser_persona_runtime::{
    ActivePersonaRuntime, PersonaRuntime, PersonaRuntimeError, PersonaRuntimeLaunch,
    PersonaRuntimeRequest,
};
use browser_profile::ProfileLease;
use parking_lot::Mutex;
use tempfile::TempDir;
use uuid::Uuid;

use super::{ControlError, ControlService};

struct MemoryStore(Mutex<HashMap<IdentityId, BrowserIdentity>>);

impl IdentityStore for MemoryStore {
    fn insert(&self, identity: &BrowserIdentity) -> Result<(), StoreError> {
        self.0.lock().insert(identity.id, identity.clone());
        Ok(())
    }

    fn get(&self, id: IdentityId) -> Result<BrowserIdentity, StoreError> {
        self.0.lock().get(&id).cloned().ok_or(StoreError::NotFound)
    }

    fn list_by_lifecycle(
        &self,
        lifecycle: browser_core::IdentityLifecycle,
    ) -> Result<Vec<BrowserIdentity>, StoreError> {
        Ok(self
            .0
            .lock()
            .values()
            .filter(|identity| identity.lifecycle == lifecycle)
            .cloned()
            .collect())
    }

    fn replace(
        &self,
        identity: &BrowserIdentity,
        expected_revision: u64,
    ) -> Result<(), StoreError> {
        let mut records = self.0.lock();
        let current = records.get(&identity.id).ok_or(StoreError::NotFound)?;
        if current.revision != expected_revision {
            return Err(StoreError::RevisionConflict);
        }
        records.insert(identity.id, identity.clone());
        Ok(())
    }
}

struct FakeHost(Mutex<HashMap<IdentityId, BrowserSession>>);

impl BrowserHost for FakeHost {
    fn detect_product_kernel(&self) -> Result<BrowserInstall, String> {
        Ok(BrowserInstall {
            executable: PathBuf::from("/fake/realbrowser-chromium"),
            version: "151.0".to_owned(),
            engine_major: 151,
        })
    }

    fn start(&self, plan: &LaunchPlan, now_ms: i64) -> Result<BrowserSession, String> {
        let session = BrowserSession {
            identity_id: plan.identity_id,
            pid: 42,
            browser_version: "151.0".to_owned(),
            started_at_ms: now_ms,
        };
        self.0.lock().insert(plan.identity_id, session.clone());
        Ok(session)
    }

    fn reconcile(&self, runtime: &BrowserRuntimeRecord) -> Result<Option<BrowserSession>, String> {
        Ok(self.0.lock().get(&runtime.identity_id).cloned())
    }

    fn stop(&self, identity_id: IdentityId) -> Result<TerminationKind, String> {
        Ok(if self.0.lock().remove(&identity_id).is_some() {
            TerminationKind::Graceful
        } else {
            TerminationKind::AlreadyExited
        })
    }

    fn is_running(&self, identity_id: IdentityId) -> bool {
        self.0.lock().contains_key(&identity_id)
    }
}

struct RecordingHost {
    install: BrowserInstall,
    plans: Mutex<Vec<LaunchPlan>>,
    sessions: Mutex<HashMap<IdentityId, BrowserSession>>,
}

impl BrowserHost for RecordingHost {
    fn detect_product_kernel(&self) -> Result<BrowserInstall, String> {
        Ok(self.install.clone())
    }

    fn start(&self, plan: &LaunchPlan, now_ms: i64) -> Result<BrowserSession, String> {
        self.plans.lock().push(plan.clone());
        let session = BrowserSession {
            identity_id: plan.identity_id,
            pid: 43,
            browser_version: plan.browser_version.clone(),
            started_at_ms: now_ms,
        };
        self.sessions
            .lock()
            .insert(plan.identity_id, session.clone());
        Ok(session)
    }

    fn reconcile(&self, runtime: &BrowserRuntimeRecord) -> Result<Option<BrowserSession>, String> {
        Ok(self.sessions.lock().get(&runtime.identity_id).cloned())
    }

    fn stop(&self, identity_id: IdentityId) -> Result<TerminationKind, String> {
        Ok(if self.sessions.lock().remove(&identity_id).is_some() {
            TerminationKind::Graceful
        } else {
            TerminationKind::AlreadyExited
        })
    }

    fn is_running(&self, identity_id: IdentityId) -> bool {
        self.sessions.lock().contains_key(&identity_id)
    }
}

struct MemoryJournal(Mutex<HashMap<IdentityId, BrowserRuntimeRecord>>);

impl RuntimeJournal for MemoryJournal {
    fn upsert(&self, runtime: &BrowserRuntimeRecord) -> Result<(), StoreError> {
        self.0.lock().insert(runtime.identity_id, runtime.clone());
        Ok(())
    }

    fn remove(&self, identity_id: IdentityId) -> Result<(), StoreError> {
        self.0.lock().remove(&identity_id);
        Ok(())
    }

    fn list(&self) -> Result<Vec<BrowserRuntimeRecord>, StoreError> {
        Ok(self.0.lock().values().cloned().collect())
    }
}

struct FixedClock;

impl Clock for FixedClock {
    fn now_ms(&self) -> i64 {
        1_700_000_000_000
    }
}

struct FixedIds;

impl IdGenerator for FixedIds {
    fn next_identity_id(&self) -> IdentityId {
        IdentityId::new(Uuid::from_u128(1))
    }
}

struct FakePersonaRuntime;

impl PersonaRuntime for FakePersonaRuntime {
    fn prepare(
        &self,
        request: &PersonaRuntimeRequest<'_>,
    ) -> Result<PersonaRuntimeLaunch, PersonaRuntimeError> {
        Ok(if request.plan.is_required() {
            PersonaRuntimeLaunch::managed(
                vec!["--remote-debugging-port=0".to_owned()],
                "about:blank",
            )
        } else {
            PersonaRuntimeLaunch::native(request.requested_startup_url)
        })
    }

    fn attach(
        &self,
        request: &PersonaRuntimeRequest<'_>,
    ) -> Result<Option<Box<dyn ActivePersonaRuntime>>, PersonaRuntimeError> {
        if !request.plan.is_required() {
            return Ok(None);
        }
        let mut fields = Vec::new();
        if let Some(timezone_id) = request.plan.timezone_id.as_deref() {
            fields.push(ObservedField {
                field: "region.timezone".to_owned(),
                expected: timezone_id.to_owned(),
                observed: timezone_id.to_owned(),
                matches: true,
            });
        }
        if let Some(metrics) = request.plan.display_metrics {
            fields.extend([
                observed_field(
                    "display.viewport",
                    format!("{}x{}", metrics.viewport_width, metrics.viewport_height),
                ),
                observed_field(
                    "display.screen",
                    format!("{}x{}", metrics.screen_width, metrics.screen_height),
                ),
                observed_field(
                    "display.devicePixelRatio",
                    format_scale(metrics.device_scale_factor()),
                ),
            ]);
        }
        if request.plan.observe_canvas {
            fields.push(observed_field(
                "graphics.canvas",
                "seeded_idempotent_copy_all_contexts_native".to_owned(),
            ));
        }
        Ok(Some(Box::new(FakeActivePersonaRuntime {
            observation: PersonaObservation { fields },
        })))
    }
}

fn observed_field(field: &str, value: String) -> ObservedField {
    ObservedField {
        field: field.to_owned(),
        expected: value.clone(),
        observed: value,
        matches: true,
    }
}

fn format_scale(value: f64) -> String {
    let value = format!("{value:.2}");
    value.trim_end_matches('0').trim_end_matches('.').to_owned()
}

struct FakeActivePersonaRuntime {
    observation: PersonaObservation,
}

struct FailingAttachPersonaRuntime;

impl PersonaRuntime for FailingAttachPersonaRuntime {
    fn prepare(
        &self,
        request: &PersonaRuntimeRequest<'_>,
    ) -> Result<PersonaRuntimeLaunch, PersonaRuntimeError> {
        FakePersonaRuntime.prepare(request)
    }

    fn attach(
        &self,
        _request: &PersonaRuntimeRequest<'_>,
    ) -> Result<Option<Box<dyn ActivePersonaRuntime>>, PersonaRuntimeError> {
        Err(PersonaRuntimeError::EndpointUnavailable)
    }
}

impl ActivePersonaRuntime for FakeActivePersonaRuntime {
    fn observation(&self) -> &PersonaObservation {
        &self.observation
    }

    fn shutdown(&mut self) -> Result<(), PersonaRuntimeError> {
        Ok(())
    }
}

fn service(root: &TempDir) -> ControlService {
    ControlService::new(
        Arc::new(MemoryStore(Mutex::new(HashMap::new()))),
        Arc::new(MemoryJournal(Mutex::new(HashMap::new()))),
        Arc::new(FakeHost(Mutex::new(HashMap::new()))),
        Arc::new(FakePersonaRuntime),
        Arc::new(FixedClock),
        Arc::new(FixedIds),
        root.path().canonicalize().unwrap(),
    )
}

fn render_launch_plan(plan: &LaunchPlan, data_root: &std::path::Path) -> String {
    let profile_root = plan.profile_root.display().to_string().replacen(
        &data_root.display().to_string(),
        "$DATA_ROOT",
        1,
    );
    let arguments = plan
        .arguments
        .iter()
        .map(|argument| {
            let argument = argument.replacen(&data_root.display().to_string(), "$DATA_ROOT", 1);
            format!("  {argument}")
        })
        .collect::<Vec<_>>()
        .join("\n");
    let environment = plan
        .environment
        .iter()
        .map(|(name, value)| format!("  {name}={value}"))
        .collect::<Vec<_>>()
        .join("\n");
    format!(
        "identity_id={}\nexecutable={}\nprofile_root={}\nstartup_url={}\nbrowser_version={}\narguments:\n{}\nenvironment:\n{}",
        plan.identity_id,
        plan.executable.display(),
        profile_root,
        plan.startup_url,
        plan.browser_version,
        arguments,
        environment,
    )
}

fn recorded_launch_plan(executable: &str) -> (String, TempDir) {
    let root = TempDir::new().unwrap();
    let data_root = root.path().canonicalize().unwrap();
    let host = Arc::new(RecordingHost {
        install: BrowserInstall {
            executable: PathBuf::from(executable),
            version: "151.0.8012.24".to_owned(),
            engine_major: 151,
        },
        plans: Mutex::new(Vec::new()),
        sessions: Mutex::new(HashMap::new()),
    });
    let service = ControlService::new(
        Arc::new(MemoryStore(Mutex::new(HashMap::new()))),
        Arc::new(MemoryJournal(Mutex::new(HashMap::new()))),
        host.clone(),
        Arc::new(FakePersonaRuntime),
        Arc::new(FixedClock),
        Arc::new(FixedIds),
        data_root.clone(),
    );
    let created = service
        .create(CreateIdentity {
            name: "Golden Store".to_owned(),
            startup_url: Some("https://seller.example/dashboard".to_owned()),
        })
        .unwrap();
    let configured = service
        .update_persona(
            created.identity.id,
            created.identity.revision,
            PersonaConfig {
                region_preset: RegionPreset::Custom,
                locale: LocalePolicy::RuRu,
                window_width: 1280,
                window_height: 720,
                webrtc: WebRtcPolicy::DisableNonProxiedUdp,
                ..created.persona
            },
        )
        .unwrap();
    service
        .update_network(
            created.identity.id,
            configured.identity.revision,
            NetworkConfig {
                schema_version: 1,
                mode: NetworkMode::Proxy,
                proxy: Some(ProxyConfig {
                    protocol: ProxyProtocol::Socks5,
                    host: "127.0.0.1".to_owned(),
                    port: 10_800,
                }),
            },
        )
        .unwrap();
    service.start(created.identity.id).unwrap();
    let plan = host.plans.lock().first().cloned().unwrap();
    (render_launch_plan(&plan, &data_root), root)
}

#[test]
fn creates_and_runs_one_isolated_identity() {
    let root = TempDir::new().unwrap();
    let service = service(&root);
    let created = service
        .create(CreateIdentity {
            name: " Store A ".to_owned(),
            startup_url: Some("https://seller.example".to_owned()),
        })
        .unwrap();

    assert_eq!(created.identity.name, "Store A");
    assert!(!service.system_snapshot().canvas_kernel_observed);
    assert!(
        created
            .identity
            .profile_root
            .starts_with(root.path().canonicalize().unwrap())
    );
    let running = service.start(created.identity.id).unwrap();
    assert_eq!(running.session.unwrap().pid, 42);
    assert!(service.system_snapshot().canvas_kernel_observed);
    let persona_path = created
        .identity
        .profile_root
        .join(".realbrowser/persona.json");
    let persona: serde_json::Value =
        serde_json::from_slice(&std::fs::read(&persona_path).unwrap()).unwrap();
    assert_eq!(persona["schema_version"], 1);
    assert_eq!(persona["persona_id"], created.identity.id.to_string());
    assert_eq!(persona["engine_major"], 151);
    assert_eq!(persona["surfaces"]["canvas"]["mode"], "seeded_noise");
    assert_eq!(persona["seed"].as_str().unwrap().len(), 64);
    let serialized = persona.to_string();
    assert!(!serialized.contains("password"));
    assert!(!serialized.contains("proxy"));
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        assert_eq!(
            std::fs::metadata(&persona_path)
                .unwrap()
                .permissions()
                .mode()
                & 0o777,
            0o600
        );
    }
    assert!(matches!(
        service.start(created.identity.id),
        Err(ControlError::AlreadyRunning)
    ));
    service.stop(created.identity.id).unwrap();
    assert!(!service.system_snapshot().canvas_kernel_observed);
}

#[test]
fn refuses_a_profile_owned_by_another_application_process() {
    let root = TempDir::new().unwrap();
    let service = service(&root);
    let created = service
        .create(CreateIdentity {
            name: "Store A".to_owned(),
            startup_url: None,
        })
        .unwrap();
    let data_root = root.path().canonicalize().unwrap();
    let _other_owner = ProfileLease::acquire(
        &data_root,
        &created.identity.profile_root,
        &created.identity.id.to_string(),
    )
    .unwrap();

    assert!(matches!(
        service.start(created.identity.id),
        Err(ControlError::AlreadyRunning)
    ));
}

#[test]
fn restart_reconciles_verified_runtime_and_clears_journal_on_stop() {
    let root = TempDir::new().unwrap();
    let data_root = root.path().canonicalize().unwrap();
    let store = Arc::new(MemoryStore(Mutex::new(HashMap::new())));
    let journal = Arc::new(MemoryJournal(Mutex::new(HashMap::new())));
    let host = Arc::new(FakeHost(Mutex::new(HashMap::new())));
    let first = ControlService::new(
        store.clone(),
        journal.clone(),
        host.clone(),
        Arc::new(FakePersonaRuntime),
        Arc::new(FixedClock),
        Arc::new(FixedIds),
        data_root.clone(),
    );
    let created = first
        .create(CreateIdentity {
            name: "Store A".to_owned(),
            startup_url: None,
        })
        .unwrap();
    first
        .update_persona(
            created.identity.id,
            created.identity.revision,
            PersonaConfig {
                region_preset: RegionPreset::Custom,
                timezone: TimezonePolicy::iana("Europe/Berlin").unwrap(),
                ..created.persona.clone()
            },
        )
        .unwrap();
    first.start(created.identity.id).unwrap();
    assert_eq!(journal.list().unwrap().len(), 1);
    drop(first);

    let second = ControlService::new(
        store,
        journal.clone(),
        host,
        Arc::new(FakePersonaRuntime),
        Arc::new(FixedClock),
        Arc::new(FixedIds),
        data_root,
    );
    let report = second.reconcile().unwrap();
    assert_eq!(report.recovered, 1);
    assert_eq!(
        second.list().unwrap()[0].runtime_state,
        browser_core::RuntimeState::Running
    );
    assert_eq!(
        second.list().unwrap()[0]
            .persona_observation
            .as_ref()
            .unwrap()
            .fields[0]
            .observed,
        "Europe/Berlin"
    );
    assert!(matches!(
        second.start(created.identity.id),
        Err(ControlError::AlreadyRunning)
    ));

    let stopped = second.stop(created.identity.id).unwrap();
    assert_eq!(stopped.termination, TerminationKind::Graceful);
    assert_eq!(
        stopped.identity.runtime_state,
        browser_core::RuntimeState::Stopped
    );
    assert!(journal.list().unwrap().is_empty());
}

#[test]
fn conflicting_rename_is_rejected() {
    let root = TempDir::new().unwrap();
    let service = service(&root);
    let created = service
        .create(CreateIdentity {
            name: "Store A".to_owned(),
            startup_url: None,
        })
        .unwrap();
    service
        .rename(created.identity.id, created.identity.revision, "Store B")
        .unwrap();

    assert!(matches!(
        service.rename(created.identity.id, created.identity.revision, "Store C"),
        Err(ControlError::RevisionConflict)
    ));
}

#[test]
fn archived_identity_can_be_listed_and_restored_without_losing_profile() {
    let root = TempDir::new().unwrap();
    let service = service(&root);
    let created = service
        .create(CreateIdentity {
            name: "Store A".to_owned(),
            startup_url: None,
        })
        .unwrap();
    let profile_root = created.identity.profile_root.clone();

    service
        .archive(created.identity.id, created.identity.revision)
        .unwrap();
    assert!(service.list().unwrap().is_empty());
    let archived = service.list_archived().unwrap();
    assert_eq!(archived.len(), 1);
    assert_eq!(archived[0].identity.profile_root, profile_root);

    let restored = service
        .restore(archived[0].identity.id, archived[0].identity.revision)
        .unwrap();
    assert_eq!(restored.identity.profile_root, profile_root);
    assert!(profile_root.is_dir());
    assert!(service.list_archived().unwrap().is_empty());
    assert_eq!(service.list().unwrap().len(), 1);
}

#[test]
fn persists_persona_before_launch() {
    let root = TempDir::new().unwrap();
    let service = service(&root);
    let created = service
        .create(CreateIdentity {
            name: "Store A".to_owned(),
            startup_url: None,
        })
        .unwrap();
    let persona = PersonaConfig {
        region_preset: RegionPreset::Custom,
        locale: LocalePolicy::EnUs,
        ..PersonaConfig::default()
    };
    let updated = service
        .update_persona(
            created.identity.id,
            created.identity.revision,
            persona.clone(),
        )
        .unwrap();
    assert_eq!(updated.persona, persona);
    assert_eq!(updated.identity.revision, 2);
}

#[test]
fn persists_proxy_before_launch_and_refuses_live_changes() {
    let root = TempDir::new().unwrap();
    let service = service(&root);
    let created = service
        .create(CreateIdentity {
            name: "Store A".to_owned(),
            startup_url: None,
        })
        .unwrap();
    let proxy = NetworkConfig {
        schema_version: 1,
        mode: NetworkMode::Proxy,
        proxy: Some(ProxyConfig {
            protocol: ProxyProtocol::Http,
            host: " Proxy.Example ".to_owned(),
            port: 8080,
        }),
    };
    let updated = service
        .update_network(created.identity.id, created.identity.revision, proxy)
        .unwrap();
    assert_eq!(updated.identity.network.mode, NetworkMode::Proxy);
    assert_eq!(
        updated.identity.network.proxy.as_ref().unwrap().host,
        "proxy.example"
    );
    assert_eq!(updated.egress, browser_core::EgressMode::Proxy);

    let configured = service
        .update_persona(
            created.identity.id,
            updated.identity.revision,
            PersonaConfig {
                webrtc: WebRtcPolicy::DisableNonProxiedUdp,
                ..updated.persona
            },
        )
        .unwrap();
    service.start(created.identity.id).unwrap();
    assert!(matches!(
        service.update_network(
            created.identity.id,
            configured.identity.revision,
            NetworkConfig::direct()
        ),
        Err(ControlError::StillRunning)
    ));
}

#[test]
fn refuses_proxy_launch_with_native_webrtc() {
    let root = TempDir::new().unwrap();
    let service = service(&root);
    let created = service
        .create(CreateIdentity {
            name: "Store A".to_owned(),
            startup_url: None,
        })
        .unwrap();
    service
        .update_network(
            created.identity.id,
            created.identity.revision,
            NetworkConfig {
                schema_version: 1,
                mode: NetworkMode::Proxy,
                proxy: Some(ProxyConfig {
                    protocol: ProxyProtocol::Socks5,
                    host: "127.0.0.1".to_owned(),
                    port: 1080,
                }),
            },
        )
        .unwrap();

    assert!(matches!(
        service.start(created.identity.id),
        Err(ControlError::Validation(issues))
            if issues[0].code == "proxy_requires_non_proxied_udp"
    ));
}

#[test]
fn rejects_proxy_credentials_in_host() {
    let root = TempDir::new().unwrap();
    let service = service(&root);
    let created = service
        .create(CreateIdentity {
            name: "Store A".to_owned(),
            startup_url: None,
        })
        .unwrap();

    assert!(matches!(
        service.update_network(
            created.identity.id,
            created.identity.revision,
            NetworkConfig {
                schema_version: 1,
                mode: NetworkMode::Proxy,
                proxy: Some(ProxyConfig {
                    protocol: ProxyProtocol::Http,
                    host: "user:pass@proxy.example".to_owned(),
                    port: 8080,
                }),
            }
        ),
        Err(ControlError::Validation(issues)) if issues[0].field == "network.proxy.host"
    ));
}

#[test]
fn launches_and_observes_a_managed_timezone_backend() {
    let root = TempDir::new().unwrap();
    let service = service(&root);
    let created = service
        .create(CreateIdentity {
            name: "Store A".to_owned(),
            startup_url: None,
        })
        .unwrap();
    let configured = service
        .update_persona(
            created.identity.id,
            created.identity.revision,
            PersonaConfig {
                region_preset: RegionPreset::Custom,
                timezone: TimezonePolicy::iana("Europe/Berlin").unwrap(),
                ..PersonaConfig::default()
            },
        )
        .unwrap();
    let running = service.start(configured.identity.id).unwrap();

    assert_eq!(running.session.unwrap().pid, 42);
    let observation = running.persona_observation.unwrap();
    assert_eq!(observation.fields[0].field, "region.timezone");
    assert_eq!(observation.fields[0].observed, "Europe/Berlin");
}

#[test]
fn launches_and_observes_managed_display_metrics_as_one_atom() {
    let root = TempDir::new().unwrap();
    let service = service(&root);
    let created = service
        .create(CreateIdentity {
            name: "Store A".to_owned(),
            startup_url: None,
        })
        .unwrap();
    let metrics = DisplayMetrics::desktop(1366, 768, 1920, 1080, 125);
    let configured = service
        .update_persona(
            created.identity.id,
            created.identity.revision,
            PersonaConfig {
                display_metrics: Some(metrics),
                ..created.persona
            },
        )
        .unwrap();
    let running = service.start(configured.identity.id).unwrap();

    let observation = running.persona_observation.unwrap();
    assert_eq!(observation.fields.len(), 4);
    assert!(observation.fields.iter().all(|field| field.matches));
    assert!(
        observation
            .fields
            .iter()
            .any(|field| { field.field == "display.viewport" && field.observed == "1366x768" })
    );
    assert!(
        observation
            .fields
            .iter()
            .any(|field| { field.field == "display.devicePixelRatio" && field.observed == "1.25" })
    );
}

#[test]
fn rolls_back_realbrowser_when_the_persona_runtime_cannot_attach() {
    let root = TempDir::new().unwrap();
    let host = Arc::new(FakeHost(Mutex::new(HashMap::new())));
    let journal = Arc::new(MemoryJournal(Mutex::new(HashMap::new())));
    let service = ControlService::new(
        Arc::new(MemoryStore(Mutex::new(HashMap::new()))),
        journal.clone(),
        host.clone(),
        Arc::new(FailingAttachPersonaRuntime),
        Arc::new(FixedClock),
        Arc::new(FixedIds),
        root.path().canonicalize().unwrap(),
    );
    let created = service
        .create(CreateIdentity {
            name: "Store A".to_owned(),
            startup_url: None,
        })
        .unwrap();
    service
        .update_persona(
            created.identity.id,
            created.identity.revision,
            PersonaConfig {
                region_preset: RegionPreset::Custom,
                timezone: TimezonePolicy::iana("Europe/Berlin").unwrap(),
                ..created.persona
            },
        )
        .unwrap();

    assert!(matches!(
        service.start(created.identity.id),
        Err(ControlError::PersonaUnavailable(_))
    ));
    assert!(!host.is_running(created.identity.id));
    assert!(journal.list().unwrap().is_empty());
}

#[test]
fn launch_plan_matches_macos_and_windows_goldens() {
    let (macos, _macos_root) = recorded_launch_plan(
        "/Applications/RealBrowser.app/Contents/Resources/realbrowser-kernel/RealBrowser.app/Contents/MacOS/RealBrowser",
    );
    assert_eq!(
        macos.trim_end(),
        include_str!("fixtures/launch-plan-macos.golden").trim_end()
    );

    let (windows, _windows_root) =
        recorded_launch_plan(r"C:\Program Files\RealBrowser\realbrowser-kernel\chrome.exe");
    assert_eq!(
        windows.trim_end(),
        include_str!("fixtures/launch-plan-windows.golden").trim_end()
    );
}
