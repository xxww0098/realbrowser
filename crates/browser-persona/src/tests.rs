use std::collections::BTreeSet;

use super::{
    ApplicationConfidence, CURRENT_SCHEMA_VERSION, Coverage, DisplayMetrics, ExecutionBackend,
    KernelSurfaceMode, LocalePolicy, NativeSurfacePolicy, PersonaConfig, PersonaMode, RegionPreset,
    TimezonePolicy, WebRtcPolicy, observed_persona_capabilities, persona_capabilities,
    timezone_catalog,
};

#[test]
fn kernel_persona_seed_is_stable_per_identity_and_domain_separated() {
    let persona = PersonaConfig::native(0x1234_5678);

    let first = persona.compile_kernel_persona("identity-a", 151);
    let restarted = persona.compile_kernel_persona("identity-a", 151);
    let other = persona.compile_kernel_persona("identity-b", 151);

    assert_eq!(first, restarted);
    assert_ne!(first.seed, other.seed);
    assert_eq!(first.seed.len(), 64);
    assert_eq!(first.surfaces.canvas.mode, KernelSurfaceMode::SeededNoise);
    let json = serde_json::to_value(first).unwrap();
    assert_eq!(json["schema_version"], 1);
    assert_eq!(json["engine_major"], 151);
    assert_eq!(json["surfaces"]["canvas"]["mode"], "seeded_noise");
}

#[test]
fn compiles_only_supported_chromium_settings_without_silent_fallback() {
    let config = PersonaConfig {
        region_preset: RegionPreset::Custom,
        locale: LocalePolicy::EnUs,
        webrtc: WebRtcPolicy::DisableNonProxiedUdp,
        ..PersonaConfig::default()
    };
    let plan = config.compile().unwrap();
    let arguments = plan.rendered_arguments();
    assert_eq!(config.mode(), PersonaMode::Managed);
    assert!(arguments.iter().any(|value| value == "--lang=en-US"));
    assert!(
        arguments
            .iter()
            .any(|value| value.contains("disable_non_proxied_udp"))
    );
    assert!(plan.applications.iter().any(|application| {
        application.field == "region.language"
            && application.backend == ExecutionBackend::Cdp
            && application.confidence == ApplicationConfidence::Observed
    }));
}

#[test]
fn compiles_timezone_for_the_observed_cdp_runtime() {
    let config = PersonaConfig {
        region_preset: RegionPreset::Custom,
        timezone: TimezonePolicy::iana("Europe/Berlin").unwrap(),
        ..PersonaConfig::default()
    };
    let plan = config.compile().unwrap();

    assert_eq!(plan.runtime.timezone_id.as_deref(), Some("Europe/Berlin"));
    assert!(plan.applications.iter().any(|application| {
        application.field == "region.timezone"
            && application.backend == ExecutionBackend::Cdp
            && application.confidence == ApplicationConfidence::Observed
    }));
}

#[test]
fn compiles_display_metrics_as_one_observed_runtime_plan() {
    let metrics = DisplayMetrics::desktop(1366, 768, 1920, 1080, 125);
    let config = PersonaConfig {
        display_metrics: Some(metrics),
        ..PersonaConfig::default()
    };
    let plan = config.compile().unwrap();

    assert_eq!(plan.runtime.display_metrics, Some(metrics));
    for field in [
        "display.viewport",
        "display.screen",
        "display.devicePixelRatio",
    ] {
        assert!(plan.applications.iter().any(|application| {
            application.field == field
                && application.backend == ExecutionBackend::Cdp
                && application.confidence == ApplicationConfidence::Observed
        }));
    }
}

#[test]
fn execution_plan_never_mutates_process_timezone() {
    let arguments = PersonaConfig::default()
        .compile()
        .unwrap()
        .rendered_arguments();
    assert!(arguments.iter().all(|value| !value.starts_with("TZ=")));
}

#[test]
fn migrates_v1_without_enabling_new_surfaces() {
    let legacy = serde_json::from_str::<PersonaConfig>(
        r#"{
          "schemaVersion": 1,
          "seed": 7,
          "locale": "en_us",
          "timezone": "system",
          "windowWidth": 1280,
          "windowHeight": 720,
          "webrtc": "native"
        }"#,
    )
    .unwrap();
    let migrated = legacy.migrate().unwrap();

    assert_eq!(migrated.schema_version, CURRENT_SCHEMA_VERSION);
    assert_eq!(
        migrated.surfaces.graphics.canvas,
        NativeSurfacePolicy::Native
    );
    assert_eq!(
        migrated.compile().unwrap().applications.len(),
        persona_capabilities().len()
    );
}

#[test]
fn capability_catalog_is_complete_unique_and_truthful() {
    let capabilities = persona_capabilities();
    let fields = capabilities
        .iter()
        .map(|capability| capability.field)
        .collect::<BTreeSet<_>>();

    assert_eq!(fields.len(), capabilities.len());
    assert!(!fields.contains("network.tls"));
    assert_eq!(
        capabilities
            .iter()
            .filter(|capability| capability.editable)
            .count(),
        8
    );
    let timezone = capabilities
        .iter()
        .find(|capability| capability.field == "region.timezone")
        .unwrap();
    assert_eq!(timezone.backend, ExecutionBackend::Cdp);
    assert_eq!(timezone.confidence, ApplicationConfidence::Observed);
    assert!(timezone.coverage.contains(Coverage::TOP_FRAME));
    assert!(capabilities.iter().all(|capability| {
        capability.confidence != ApplicationConfidence::NotApplied
            || capability.coverage == Coverage::NONE
    }));
}

#[test]
fn canvas_becomes_custom_kernel_only_after_runtime_observation() {
    let before = observed_persona_capabilities(false);
    let after = observed_persona_capabilities(true);
    let before_canvas = before
        .iter()
        .find(|capability| capability.field == "graphics.canvas")
        .unwrap();
    let after_canvas = after
        .iter()
        .find(|capability| capability.field == "graphics.canvas")
        .unwrap();

    assert_eq!(before_canvas.backend, ExecutionBackend::Native);
    assert_eq!(before_canvas.confidence, ApplicationConfidence::Native);
    assert_eq!(after_canvas.backend, ExecutionBackend::CustomKernel);
    assert_eq!(after_canvas.confidence, ApplicationConfidence::Observed);
    assert!(after_canvas.coverage.contains(Coverage::ALL_FRAMES));
    assert!(after_canvas.coverage.contains(Coverage::DEDICATED_WORKER));
}

#[test]
fn v4_persona_json_round_trips_managed_region_and_display() {
    let persona = PersonaConfig {
        region_preset: RegionPreset::Custom,
        timezone: TimezonePolicy::iana("Asia/Tokyo").unwrap(),
        display_metrics: Some(DisplayMetrics::desktop(1440, 900, 2560, 1440, 200)),
        ..PersonaConfig::native(42)
    };
    let encoded = serde_json::to_string(&persona).unwrap();
    let decoded = serde_json::from_str::<PersonaConfig>(&encoded).unwrap();
    assert_eq!(decoded, persona);
    assert!(encoded.contains("Asia/Tokyo"));
    assert!(encoded.contains("deviceScaleFactorPercent"));
}

#[test]
fn migrates_legacy_timezone_tokens_to_iana_ids() {
    let legacy = serde_json::from_str::<PersonaConfig>(
        r#"{
          "schemaVersion": 2,
          "seed": 7,
          "locale": "de_de",
          "timezone": "europe_berlin",
          "windowWidth": 1440,
          "windowHeight": 900,
          "webrtc": "native"
        }"#,
    )
    .unwrap();
    let migrated = legacy.migrate().unwrap();

    assert_eq!(migrated.schema_version, CURRENT_SCHEMA_VERSION);
    assert_eq!(migrated.timezone.cdp_value(), Some("Europe/Berlin"));
}

#[test]
fn rejects_unknown_timezone_ids_and_publishes_the_bundled_catalog() {
    let invalid = PersonaConfig {
        timezone: TimezonePolicy("Mars/Olympus_Mons".to_owned()),
        ..PersonaConfig::default()
    };

    assert!(matches!(
        invalid.compile(),
        Err(super::PersonaError::Invalid(issues))
            if issues[0].field == "persona.timezone"
                && issues[0].code == "invalid_timezone"
    ));
    let catalog = timezone_catalog();
    assert!(catalog.len() > 400);
    assert!(catalog.iter().any(|name| name == "Asia/Tokyo"));
    assert!(catalog.iter().any(|name| name == "America/New_York"));
}

#[test]
fn rejects_an_incoherent_display_atom() {
    let invalid = PersonaConfig {
        display_metrics: Some(DisplayMetrics::desktop(1920, 1080, 1366, 768, 110)),
        ..PersonaConfig::default()
    };
    let Err(super::PersonaError::Invalid(issues)) = invalid.compile() else {
        panic!("incoherent display metrics should fail");
    };

    assert!(issues.iter().any(|issue| {
        issue.field == "persona.displayMetrics.screenWidth" && issue.code == "invalid_screen_size"
    }));
    assert!(issues.iter().any(|issue| {
        issue.field == "persona.displayMetrics.deviceScaleFactorPercent"
            && issue.code == "invalid_scale_factor"
    }));
}

#[test]
fn migrates_v3_with_native_display_metrics() {
    let legacy = serde_json::from_str::<PersonaConfig>(
        r#"{
          "schemaVersion": 3,
          "seed": 9,
          "locale": "system",
          "timezone": "Asia/Tokyo",
          "windowWidth": 1440,
          "windowHeight": 900,
          "webrtc": "native",
          "surfaces": {
            "display": {
              "viewport": "native",
              "screen": "native",
              "devicePixelRatio": "native"
            }
          }
        }"#,
    )
    .unwrap();
    let migrated = legacy.migrate().unwrap();

    assert_eq!(migrated.schema_version, CURRENT_SCHEMA_VERSION);
    assert_eq!(migrated.display_metrics, None);
}

#[test]
fn native_persona_json_round_trips() {
    let persona = PersonaConfig::native(42);
    let encoded = serde_json::to_string(&persona).unwrap();
    let decoded = serde_json::from_str::<PersonaConfig>(&encoded).unwrap();
    assert_eq!(decoded, persona);
}
