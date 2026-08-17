#![forbid(unsafe_code)]

use std::sync::OnceLock;

use jiff::tz;
use serde::{Deserialize, Serialize};
use thiserror::Error;

mod region;

pub use region::{Geolocation, RegionPreset, RegionPresetValues};

#[derive(Clone, Copy, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum PersonaMode {
    Native,
    Managed,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum LocalePolicy {
    System,
    ZhCn,
    EnUs,
    RuRu,
    DeDe,
}

impl LocalePolicy {
    #[must_use]
    pub fn chrome_value(self) -> Option<&'static str> {
        match self {
            Self::System => None,
            Self::ZhCn => Some("zh-CN"),
            Self::EnUs => Some("en-US"),
            Self::RuRu => Some("ru-RU"),
            Self::DeDe => Some("de-DE"),
        }
    }

    #[must_use]
    pub fn icu_value(self) -> Option<&'static str> {
        match self {
            Self::System => None,
            Self::ZhCn => Some("zh_CN"),
            Self::EnUs => Some("en_US"),
            Self::RuRu => Some("ru_RU"),
            Self::DeDe => Some("de_DE"),
        }
    }

    #[must_use]
    pub fn accept_language(self) -> Option<&'static str> {
        match self {
            Self::System => None,
            Self::ZhCn => Some("zh-CN,zh;q=0.9,en;q=0.8"),
            Self::EnUs => Some("en-US,en;q=0.9"),
            Self::RuRu => Some("ru-RU,ru;q=0.9,en;q=0.8"),
            Self::DeDe => Some("de-DE,de;q=0.9,en;q=0.8"),
        }
    }

    fn from_language_tag(value: &str) -> Option<Self> {
        match value {
            "zh-CN" => Some(Self::ZhCn),
            "en-US" => Some(Self::EnUs),
            "ru-RU" => Some(Self::RuRu),
            "de-DE" => Some(Self::DeDe),
            _ => None,
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(transparent)]
pub struct TimezonePolicy(String);

impl TimezonePolicy {
    #[must_use]
    pub fn system() -> Self {
        Self("system".to_owned())
    }

    pub fn iana(value: &str) -> Result<Self, PersonaError> {
        Self(value.to_owned())
            .normalized()
            .map_err(|issue| PersonaError::Invalid(vec![issue]))
    }

    #[must_use]
    pub fn is_system(&self) -> bool {
        self.0 == "system"
    }

    #[must_use]
    pub fn cdp_value(&self) -> Option<&str> {
        if self.is_system() {
            None
        } else {
            Some(&self.0)
        }
    }

    fn normalized(self) -> Result<Self, PersonaIssue> {
        let value = match self.0.as_str() {
            "system" => return Ok(Self::system()),
            "asia_shanghai" => "Asia/Shanghai",
            "europe_moscow" => "Europe/Moscow",
            "europe_berlin" => "Europe/Berlin",
            "america_new_york" => "America/New_York",
            value => value,
        };
        let timezone = tz::TimeZone::get(value).map_err(|_| {
            PersonaIssue::new(
                "persona.timezone",
                "invalid_timezone",
                "请选择有效的 IANA 时区",
            )
        })?;
        let Some(name) = timezone.iana_name().filter(|name| *name != "Etc/Unknown") else {
            return Err(PersonaIssue::new(
                "persona.timezone",
                "invalid_timezone",
                "请选择有效的 IANA 时区",
            ));
        };
        Ok(Self(name.to_owned()))
    }
}

impl Default for TimezonePolicy {
    fn default() -> Self {
        Self::system()
    }
}

static TIMEZONE_CATALOG: OnceLock<Vec<String>> = OnceLock::new();

#[must_use]
pub fn timezone_catalog() -> &'static [String] {
    TIMEZONE_CATALOG.get_or_init(|| {
        let mut names = tz::db()
            .available()
            .filter(|name| name.as_str() != "Etc/Unknown")
            .map(|name| name.as_str().to_owned())
            .collect::<Vec<_>>();
        if !names.iter().any(|name| name == "UTC") {
            names.push("UTC".to_owned());
        }
        names.sort_unstable();
        names.dedup();
        names
    })
}

#[derive(Clone, Copy, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum WebRtcPolicy {
    Native,
    DisableNonProxiedUdp,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DisplayMetrics {
    pub viewport_width: u16,
    pub viewport_height: u16,
    pub screen_width: u16,
    pub screen_height: u16,
    pub device_scale_factor_percent: u16,
}

impl DisplayMetrics {
    #[must_use]
    pub const fn desktop(
        viewport_width: u16,
        viewport_height: u16,
        screen_width: u16,
        screen_height: u16,
        device_scale_factor_percent: u16,
    ) -> Self {
        Self {
            viewport_width,
            viewport_height,
            screen_width,
            screen_height,
            device_scale_factor_percent,
        }
    }

    #[must_use]
    pub fn device_scale_factor(self) -> f64 {
        f64::from(self.device_scale_factor_percent) / 100.0
    }
}

pub const CURRENT_SCHEMA_VERSION: u16 = 5;
pub const KERNEL_PERSONA_SCHEMA_VERSION: u16 = 1;

#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum KernelSurfaceMode {
    Native,
    SeededNoise,
}

#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
pub struct KernelCanvasSurface {
    pub mode: KernelSurfaceMode,
}

#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
pub struct KernelPersonaSurfaces {
    pub canvas: KernelCanvasSurface,
}

/// The non-secret, versioned contract passed to the product Chromium.
/// It intentionally contains only K0/K1 fields.
#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
pub struct KernelPersonaConfig {
    pub schema_version: u16,
    pub persona_id: String,
    pub seed: String,
    pub engine_major: u16,
    pub surfaces: KernelPersonaSurfaces,
}

/// A persisted intent for surfaces that the RealBrowser kernel currently leaves untouched.
/// New managed variants require a schema bump and a verified execution backend.
#[derive(Clone, Copy, Debug, Default, Eq, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum NativeSurfacePolicy {
    #[default]
    Native,
}

#[derive(Clone, Debug, Default, Eq, PartialEq, Serialize, Deserialize)]
#[serde(default, rename_all = "camelCase")]
pub struct BrowserSurfaceSpec {
    pub user_agent: NativeSurfacePolicy,
    pub platform: NativeSurfacePolicy,
    pub plugins: NativeSurfacePolicy,
    pub battery: NativeSurfacePolicy,
}

#[derive(Clone, Debug, Default, Eq, PartialEq, Serialize, Deserialize)]
#[serde(default, rename_all = "camelCase")]
pub struct RegionSurfaceSpec {
    pub geolocation: NativeSurfacePolicy,
}

#[derive(Clone, Debug, Default, Eq, PartialEq, Serialize, Deserialize)]
#[serde(default, rename_all = "camelCase")]
pub struct HardwareSurfaceSpec {
    pub cpu: NativeSurfacePolicy,
    pub device_memory: NativeSurfacePolicy,
    pub touch: NativeSurfacePolicy,
}

#[derive(Clone, Debug, Default, Eq, PartialEq, Serialize, Deserialize)]
#[serde(default, rename_all = "camelCase")]
pub struct GraphicsSurfaceSpec {
    pub canvas: NativeSurfacePolicy,
    pub webgl_image: NativeSurfacePolicy,
    pub webgl_metadata: NativeSurfacePolicy,
    pub webgpu: NativeSurfacePolicy,
    pub client_rects: NativeSurfacePolicy,
}

#[derive(Clone, Debug, Default, Eq, PartialEq, Serialize, Deserialize)]
#[serde(default, rename_all = "camelCase")]
pub struct MediaSurfaceSpec {
    pub audio: NativeSurfacePolicy,
    pub fonts: NativeSurfacePolicy,
    pub media_devices: NativeSurfacePolicy,
    pub speech_voices: NativeSurfacePolicy,
}

#[derive(Clone, Debug, Default, Eq, PartialEq, Serialize, Deserialize)]
#[serde(default, rename_all = "camelCase")]
pub struct PrivacySurfaceSpec {
    pub permissions: NativeSurfacePolicy,
    pub do_not_track: NativeSurfacePolicy,
    pub global_privacy_control: NativeSurfacePolicy,
}

#[derive(Clone, Debug, Default, Eq, PartialEq, Serialize, Deserialize)]
#[serde(default, rename_all = "camelCase")]
pub struct PersonaSurfaceSpec {
    pub browser: BrowserSurfaceSpec,
    pub region: RegionSurfaceSpec,
    pub hardware: HardwareSurfaceSpec,
    pub graphics: GraphicsSurfaceSpec,
    pub media: MediaSurfaceSpec,
    pub privacy: PrivacySurfaceSpec,
}

/// Persisted user intent. It intentionally contains no claim that a field was applied.
#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(default, rename_all = "camelCase")]
pub struct PersonaSpec {
    pub schema_version: u16,
    pub seed: u32,
    pub region_preset: RegionPreset,
    pub locale: LocalePolicy,
    pub timezone: TimezonePolicy,
    pub geolocation: Option<Geolocation>,
    pub window_width: u16,
    pub window_height: u16,
    pub display_metrics: Option<DisplayMetrics>,
    pub webrtc: WebRtcPolicy,
    pub surfaces: PersonaSurfaceSpec,
}

pub type PersonaConfig = PersonaSpec;

impl Default for PersonaSpec {
    fn default() -> Self {
        Self::native(0)
    }
}

impl PersonaSpec {
    #[must_use]
    pub fn native(seed: u32) -> Self {
        Self {
            schema_version: CURRENT_SCHEMA_VERSION,
            seed,
            region_preset: RegionPreset::Native,
            locale: LocalePolicy::System,
            timezone: TimezonePolicy::system(),
            geolocation: None,
            window_width: 1440,
            window_height: 900,
            display_metrics: None,
            webrtc: WebRtcPolicy::Native,
            surfaces: PersonaSurfaceSpec::default(),
        }
    }

    #[must_use]
    pub fn compile_kernel_persona(
        &self,
        persona_id: &str,
        engine_major: u16,
    ) -> KernelPersonaConfig {
        let mut hasher = blake3::Hasher::new_derive_key(
            "realbrowser kernel persona graphics.canvas readback v1",
        );
        hasher.update(&self.schema_version.to_be_bytes());
        hasher.update(&self.seed.to_be_bytes());
        hasher.update(persona_id.as_bytes());

        KernelPersonaConfig {
            schema_version: KERNEL_PERSONA_SCHEMA_VERSION,
            persona_id: persona_id.to_owned(),
            seed: hasher.finalize().to_hex().to_string(),
            engine_major,
            surfaces: KernelPersonaSurfaces {
                canvas: KernelCanvasSurface {
                    mode: KernelSurfaceMode::SeededNoise,
                },
            },
        }
    }

    /// Migrates persisted intent without claiming any newly managed surface.
    pub fn migrate(mut self) -> Result<Self, PersonaError> {
        match self.schema_version {
            1..=4 => {
                self.region_preset = if self.locale == LocalePolicy::System
                    && self.timezone.is_system()
                    && self.geolocation.is_none()
                {
                    RegionPreset::Native
                } else {
                    RegionPreset::Custom
                };
                self.schema_version = CURRENT_SCHEMA_VERSION;
            }
            CURRENT_SCHEMA_VERSION => {}
            _ => {
                return Err(PersonaError::Invalid(vec![PersonaIssue::new(
                    "persona.schemaVersion",
                    "unsupported_version",
                    "当前版本不支持这份 Persona 配置",
                )]));
            }
        }
        self.timezone = self
            .timezone
            .normalized()
            .map_err(|issue| PersonaError::Invalid(vec![issue]))?;
        Ok(self)
    }

    pub fn apply_region_preset(&mut self, preset: RegionPreset) {
        self.region_preset = preset;
        match preset {
            RegionPreset::Native => {
                self.locale = LocalePolicy::System;
                self.timezone = TimezonePolicy::system();
                self.geolocation = None;
            }
            RegionPreset::Custom => {}
            _ => {
                let values = preset.values().expect("named preset has values");
                self.locale = LocalePolicy::from_language_tag(values.locale_tag)
                    .expect("preset locale is supported");
                self.timezone = TimezonePolicy(values.timezone_id.to_owned());
                self.geolocation = Some(values.geolocation);
            }
        }
    }

    #[must_use]
    pub fn mode(&self) -> PersonaMode {
        if self == &Self::native(self.seed) {
            PersonaMode::Native
        } else {
            PersonaMode::Managed
        }
    }

    pub fn compile(&self) -> Result<PersonaExecutionPlan, PersonaError> {
        let issues = self.validation_issues();
        if !issues.is_empty() {
            return Err(PersonaError::Invalid(issues));
        }

        let mut launch_arguments = vec![LaunchArgument::WindowSize {
            width: self.window_width,
            height: self.window_height,
        }];
        let mut applications = vec![FieldApplication::launch_argument(
            "display.window",
            ApplicationConfidence::MappedUnverified,
        )];

        if let Some(locale) = self.locale.chrome_value() {
            launch_arguments.push(LaunchArgument::Language(locale));
        }

        if self.webrtc == WebRtcPolicy::DisableNonProxiedUdp {
            launch_arguments.push(LaunchArgument::DisableNonProxiedWebRtcUdp);
            applications.push(FieldApplication::launch_argument(
                "privacy.webrtc",
                ApplicationConfidence::MappedUnverified,
            ));
        } else {
            applications.push(FieldApplication::native("privacy.webrtc"));
        }

        let runtime = PersonaRuntimePlan {
            locale: self
                .locale
                .chrome_value()
                .map(|language_tag| LocaleRuntimePlan {
                    language_tag,
                    icu_locale: self
                        .locale
                        .icu_value()
                        .expect("managed locale has ICU value"),
                    accept_language: self
                        .locale
                        .accept_language()
                        .expect("managed locale has Accept-Language value"),
                }),
            timezone_id: self.timezone.cdp_value().map(str::to_owned),
            geolocation: self.geolocation,
            display_metrics: self.display_metrics,
            observe_canvas: false,
        };

        if runtime.locale.is_some() {
            applications.push(FieldApplication::cdp("region.language"));
            applications.push(FieldApplication::cdp_mapped("region.acceptLanguage"));
        } else {
            applications.push(FieldApplication::native("region.language"));
            applications.push(FieldApplication::native("region.acceptLanguage"));
        }

        if runtime.timezone_id.is_some() {
            applications.push(FieldApplication::cdp("region.timezone"));
        } else {
            applications.push(FieldApplication::native("region.timezone"));
        }

        if runtime.geolocation.is_some() {
            applications.push(FieldApplication::cdp_mapped("region.geolocation"));
        } else {
            applications.push(FieldApplication::native("region.geolocation"));
        }

        for field in [
            "display.viewport",
            "display.screen",
            "display.devicePixelRatio",
        ] {
            applications.push(if runtime.display_metrics.is_some() {
                FieldApplication::cdp(field)
            } else {
                FieldApplication::native(field)
            });
        }

        for capability in persona_capabilities() {
            if matches!(
                capability.field,
                "region.language"
                    | "region.acceptLanguage"
                    | "region.timezone"
                    | "region.geolocation"
                    | "display.window"
                    | "display.viewport"
                    | "display.screen"
                    | "display.devicePixelRatio"
                    | "privacy.webrtc"
            ) {
                continue;
            }
            applications.push(
                if capability.confidence == ApplicationConfidence::NotApplied {
                    FieldApplication::native(capability.field)
                } else {
                    FieldApplication {
                        field: capability.field,
                        backend: capability.backend,
                        confidence: capability.confidence,
                    }
                },
            );
        }

        Ok(PersonaExecutionPlan {
            launch_arguments,
            applications,
            runtime,
        })
    }

    #[must_use]
    pub fn validation_issues(&self) -> Vec<PersonaIssue> {
        let mut issues = Vec::new();
        if self.schema_version != CURRENT_SCHEMA_VERSION {
            issues.push(PersonaIssue::new(
                "persona.schemaVersion",
                "unsupported_version",
                "当前版本仅支持 Persona schema v5",
            ));
        }
        if !(800..=3840).contains(&self.window_width) {
            issues.push(PersonaIssue::new(
                "persona.windowWidth",
                "out_of_range",
                "窗口宽度需在 800 到 3840 之间",
            ));
        }
        if !(600..=2160).contains(&self.window_height) {
            issues.push(PersonaIssue::new(
                "persona.windowHeight",
                "out_of_range",
                "窗口高度需在 600 到 2160 之间",
            ));
        }
        if let Err(issue) = self.timezone.clone().normalized() {
            issues.push(issue);
        }
        validate_region(self, &mut issues);
        if let Some(metrics) = self.display_metrics {
            validate_display_metrics(metrics, &mut issues);
        }
        issues
    }
}

fn validate_region(persona: &PersonaSpec, issues: &mut Vec<PersonaIssue>) {
    if let Some(geolocation) = persona.geolocation {
        if !(-90_000_000..=90_000_000).contains(&geolocation.latitude_e6) {
            issues.push(PersonaIssue::new(
                "persona.geolocation.latitudeE6",
                "out_of_range",
                "纬度需在 -90 到 90 之间",
            ));
        }
        if !(-180_000_000..=180_000_000).contains(&geolocation.longitude_e6) {
            issues.push(PersonaIssue::new(
                "persona.geolocation.longitudeE6",
                "out_of_range",
                "经度需在 -180 到 180 之间",
            ));
        }
        if !(1..=10_000).contains(&geolocation.accuracy_meters) {
            issues.push(PersonaIssue::new(
                "persona.geolocation.accuracyMeters",
                "out_of_range",
                "定位精度需在 1 到 10000 米之间",
            ));
        }
    }

    match persona.region_preset {
        RegionPreset::Native => {
            if persona.locale != LocalePolicy::System
                || !persona.timezone.is_system()
                || persona.geolocation.is_some()
            {
                issues.push(PersonaIssue::new(
                    "persona.regionPreset",
                    "incoherent_native_region",
                    "系统原生地区不能同时保存地区覆盖",
                ));
            }
        }
        RegionPreset::Custom => {
            if persona.locale == LocalePolicy::System
                && persona.timezone.is_system()
                && persona.geolocation.is_none()
            {
                issues.push(PersonaIssue::new(
                    "persona.regionPreset",
                    "empty_custom_region",
                    "自定义地区至少需要修改一项",
                ));
            }
        }
        preset => {
            let values = preset.values().expect("named preset has values");
            if persona.locale.chrome_value() != Some(values.locale_tag)
                || persona.timezone.cdp_value() != Some(values.timezone_id)
                || persona.geolocation != Some(values.geolocation)
            {
                issues.push(PersonaIssue::new(
                    "persona.regionPreset",
                    "incoherent_region_preset",
                    "地区预设的语言、时区和坐标必须保持联动",
                ));
            }
        }
    }
}

fn validate_display_metrics(metrics: DisplayMetrics, issues: &mut Vec<PersonaIssue>) {
    if !(800..=3840).contains(&metrics.viewport_width) {
        issues.push(PersonaIssue::new(
            "persona.displayMetrics.viewportWidth",
            "out_of_range",
            "Viewport 宽度需在 800 到 3840 之间",
        ));
    }
    if !(600..=2160).contains(&metrics.viewport_height) {
        issues.push(PersonaIssue::new(
            "persona.displayMetrics.viewportHeight",
            "out_of_range",
            "Viewport 高度需在 600 到 2160 之间",
        ));
    }
    if metrics.screen_width < metrics.viewport_width || metrics.screen_width > 7680 {
        issues.push(PersonaIssue::new(
            "persona.displayMetrics.screenWidth",
            "invalid_screen_size",
            "屏幕宽度不得小于 Viewport，且不能超过 7680",
        ));
    }
    if metrics.screen_height < metrics.viewport_height || metrics.screen_height > 4320 {
        issues.push(PersonaIssue::new(
            "persona.displayMetrics.screenHeight",
            "invalid_screen_size",
            "屏幕高度不得小于 Viewport，且不能超过 4320",
        ));
    }
    if !(100..=300).contains(&metrics.device_scale_factor_percent)
        || !metrics.device_scale_factor_percent.is_multiple_of(25)
    {
        issues.push(PersonaIssue::new(
            "persona.displayMetrics.deviceScaleFactorPercent",
            "invalid_scale_factor",
            "像素比需在 1.00 到 3.00 之间，并以 0.25 递增",
        ));
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum LaunchArgument {
    WindowSize { width: u16, height: u16 },
    Language(&'static str),
    DisableNonProxiedWebRtcUdp,
}

impl LaunchArgument {
    #[must_use]
    pub fn render(self) -> String {
        match self {
            Self::WindowSize { width, height } => format!("--window-size={width},{height}"),
            Self::Language(locale) => format!("--lang={locale}"),
            Self::DisableNonProxiedWebRtcUdp => {
                "--force-webrtc-ip-handling-policy=disable_non_proxied_udp".to_owned()
            }
        }
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ExecutionBackend {
    Native,
    Profile,
    LaunchArgument,
    Cdp,
    ExtensionLimited,
    CustomKernel,
    Unavailable,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ApplicationConfidence {
    Native,
    MappedUnverified,
    Observed,
    NotApplied,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum PersonaGroup {
    Region,
    Device,
    Graphics,
    Media,
    Privacy,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct Coverage(u16);

impl Coverage {
    pub const NONE: Self = Self(0);
    pub const BROWSER: Self = Self(1 << 0);
    pub const PROFILE: Self = Self(1 << 1);
    pub const PROCESS: Self = Self(1 << 2);
    pub const TOP_FRAME: Self = Self(1 << 3);
    pub const ALL_FRAMES: Self = Self(1 << 4);
    pub const DEDICATED_WORKER: Self = Self(1 << 5);
    pub const SHARED_WORKER: Self = Self(1 << 6);
    pub const SERVICE_WORKER: Self = Self(1 << 7);
    pub const NETWORK: Self = Self(1 << 8);

    #[must_use]
    pub const fn contains(self, other: Self) -> bool {
        self.0 & other.0 == other.0
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct PersonaCapability {
    pub field: &'static str,
    pub group: PersonaGroup,
    pub backend: ExecutionBackend,
    pub confidence: ApplicationConfidence,
    pub coverage: Coverage,
    pub editable: bool,
}

impl PersonaCapability {
    const fn native(field: &'static str, group: PersonaGroup) -> Self {
        Self {
            field,
            group,
            backend: ExecutionBackend::Native,
            confidence: ApplicationConfidence::Native,
            coverage: Coverage::BROWSER,
            editable: false,
        }
    }

    const fn profile(field: &'static str, group: PersonaGroup) -> Self {
        Self {
            field,
            group,
            backend: ExecutionBackend::Profile,
            confidence: ApplicationConfidence::Native,
            coverage: Coverage::PROFILE,
            editable: false,
        }
    }

    const fn launch(field: &'static str, group: PersonaGroup) -> Self {
        Self {
            field,
            group,
            backend: ExecutionBackend::LaunchArgument,
            confidence: ApplicationConfidence::MappedUnverified,
            coverage: Coverage::PROCESS,
            editable: true,
        }
    }

    const fn cdp_observed(field: &'static str, group: PersonaGroup) -> Self {
        Self {
            field,
            group,
            backend: ExecutionBackend::Cdp,
            confidence: ApplicationConfidence::Observed,
            coverage: Coverage::TOP_FRAME,
            editable: true,
        }
    }

    const fn cdp_mapped(
        field: &'static str,
        group: PersonaGroup,
        coverage: Coverage,
        editable: bool,
    ) -> Self {
        Self {
            field,
            group,
            backend: ExecutionBackend::Cdp,
            confidence: ApplicationConfidence::MappedUnverified,
            coverage,
            editable,
        }
    }

    const fn custom_kernel(field: &'static str, group: PersonaGroup, coverage: Coverage) -> Self {
        Self {
            field,
            group,
            backend: ExecutionBackend::CustomKernel,
            confidence: ApplicationConfidence::Observed,
            coverage,
            editable: false,
        }
    }
}

static REALBROWSER_CAPABILITIES: [PersonaCapability; 28] = [
    PersonaCapability::cdp_observed("region.language", PersonaGroup::Region),
    PersonaCapability::cdp_mapped(
        "region.acceptLanguage",
        PersonaGroup::Region,
        Coverage::NETWORK,
        false,
    ),
    PersonaCapability::cdp_observed("region.timezone", PersonaGroup::Region),
    PersonaCapability::cdp_mapped(
        "region.geolocation",
        PersonaGroup::Region,
        Coverage::TOP_FRAME,
        true,
    ),
    PersonaCapability::native("browser.userAgent", PersonaGroup::Device),
    PersonaCapability::native("browser.platform", PersonaGroup::Device),
    PersonaCapability::launch("display.window", PersonaGroup::Device),
    PersonaCapability::cdp_observed("display.viewport", PersonaGroup::Device),
    PersonaCapability::cdp_observed("display.screen", PersonaGroup::Device),
    PersonaCapability::cdp_observed("display.devicePixelRatio", PersonaGroup::Device),
    PersonaCapability::native("hardware.cpu", PersonaGroup::Device),
    PersonaCapability::native("hardware.deviceMemory", PersonaGroup::Device),
    PersonaCapability::native("hardware.touch", PersonaGroup::Device),
    PersonaCapability::native("browser.plugins", PersonaGroup::Device),
    PersonaCapability::native("browser.battery", PersonaGroup::Device),
    PersonaCapability::native("graphics.canvas", PersonaGroup::Graphics),
    PersonaCapability::native("graphics.webglImage", PersonaGroup::Graphics),
    PersonaCapability::native("graphics.webglMetadata", PersonaGroup::Graphics),
    PersonaCapability::native("graphics.webgpu", PersonaGroup::Graphics),
    PersonaCapability::native("graphics.clientRects", PersonaGroup::Graphics),
    PersonaCapability::native("media.audio", PersonaGroup::Media),
    PersonaCapability::native("media.fonts", PersonaGroup::Media),
    PersonaCapability::native("media.mediaDevices", PersonaGroup::Media),
    PersonaCapability::native("media.speechVoices", PersonaGroup::Media),
    PersonaCapability::launch("privacy.webrtc", PersonaGroup::Privacy),
    PersonaCapability::profile("privacy.permissions", PersonaGroup::Privacy),
    PersonaCapability::native("privacy.doNotTrack", PersonaGroup::Privacy),
    PersonaCapability::native("privacy.globalPrivacyControl", PersonaGroup::Privacy),
];

#[must_use]
pub const fn persona_capabilities() -> &'static [PersonaCapability] {
    &REALBROWSER_CAPABILITIES
}

/// Returns the catalog projected from runtime evidence. Canvas is only presented as
/// CustomKernel after the running product kernel passed the K1 observation matrix.
#[must_use]
pub fn observed_persona_capabilities(canvas_kernel_observed: bool) -> Vec<PersonaCapability> {
    let mut capabilities = REALBROWSER_CAPABILITIES.to_vec();
    if canvas_kernel_observed
        && let Some(canvas) = capabilities
            .iter_mut()
            .find(|capability| capability.field == "graphics.canvas")
    {
        *canvas = PersonaCapability::custom_kernel(
            "graphics.canvas",
            PersonaGroup::Graphics,
            Coverage(Coverage::ALL_FRAMES.0 | Coverage::DEDICATED_WORKER.0),
        );
    }
    capabilities
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct FieldApplication {
    pub field: &'static str,
    pub backend: ExecutionBackend,
    pub confidence: ApplicationConfidence,
}

impl FieldApplication {
    const fn native(field: &'static str) -> Self {
        Self {
            field,
            backend: ExecutionBackend::Native,
            confidence: ApplicationConfidence::Native,
        }
    }

    const fn launch_argument(field: &'static str, confidence: ApplicationConfidence) -> Self {
        Self {
            field,
            backend: ExecutionBackend::LaunchArgument,
            confidence,
        }
    }

    const fn cdp(field: &'static str) -> Self {
        Self {
            field,
            backend: ExecutionBackend::Cdp,
            confidence: ApplicationConfidence::Observed,
        }
    }

    const fn cdp_mapped(field: &'static str) -> Self {
        Self {
            field,
            backend: ExecutionBackend::Cdp,
            confidence: ApplicationConfidence::MappedUnverified,
        }
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct LocaleRuntimePlan {
    pub language_tag: &'static str,
    pub icu_locale: &'static str,
    pub accept_language: &'static str,
}

#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct PersonaRuntimePlan {
    pub locale: Option<LocaleRuntimePlan>,
    pub timezone_id: Option<String>,
    pub geolocation: Option<Geolocation>,
    pub display_metrics: Option<DisplayMetrics>,
    /// Require the product-kernel K1 Canvas observation matrix during attach.
    pub observe_canvas: bool,
}

impl PersonaRuntimePlan {
    #[must_use]
    pub const fn is_required(&self) -> bool {
        self.locale.is_some()
            || self.timezone_id.is_some()
            || self.geolocation.is_some()
            || self.display_metrics.is_some()
            || self.observe_canvas
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PersonaExecutionPlan {
    pub launch_arguments: Vec<LaunchArgument>,
    pub applications: Vec<FieldApplication>,
    pub runtime: PersonaRuntimePlan,
}

impl PersonaExecutionPlan {
    #[must_use]
    pub fn rendered_arguments(&self) -> Vec<String> {
        self.launch_arguments
            .iter()
            .copied()
            .map(LaunchArgument::render)
            .collect()
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PersonaObservation {
    pub fields: Vec<ObservedField>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ObservedField {
    pub field: String,
    pub expected: String,
    pub observed: String,
    pub matches: bool,
}

#[derive(Clone, Debug, Eq, Error, PartialEq)]
pub enum PersonaError {
    #[error("指纹设置需要修正")]
    Invalid(Vec<PersonaIssue>),
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PersonaIssue {
    pub field: &'static str,
    pub code: &'static str,
    pub message: String,
}

impl PersonaIssue {
    fn new(field: &'static str, code: &'static str, message: &str) -> Self {
        Self {
            field,
            code,
            message: message.to_owned(),
        }
    }
}

#[cfg(test)]
mod tests;
