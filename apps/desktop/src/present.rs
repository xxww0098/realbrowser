use browser_network::{
    NETWORK_SCHEMA_VERSION, NetworkConfig, NetworkMode, ProxyConfig, ProxyProtocol,
};
use browser_persona::{
    DisplayMetrics, LocalePolicy, PersonaConfig, PersonaMode, RegionPreset, TimezonePolicy,
    WebRtcPolicy,
};

use crate::IdentityView;

pub fn runtime_label(state: &str) -> &'static str {
    match state {
        "running" => "运行中",
        "starting" => "启动中",
        "stopping" => "停止中",
        "failed" => "失败",
        _ => "已停止",
    }
}

pub fn persona_mode_label(mode: &str) -> &'static str {
    if mode == "managed" {
        "已配置"
    } else {
        "原生"
    }
}

pub fn persona_status_label(record: &IdentityView) -> &'static str {
    if record
        .persona_observation
        .as_ref()
        .is_some_and(|observation| observation.matches)
    {
        "已观测"
    } else {
        persona_mode_label(record.persona_mode)
    }
}

pub fn egress_label(mode: &str) -> &'static str {
    if mode == "proxy" { "代理" } else { "直连" }
}

pub fn profile_label() -> &'static str {
    "独立"
}

pub fn region_label(preset: RegionPreset) -> &'static str {
    match preset {
        RegionPreset::Native => "系统原生",
        RegionPreset::Custom => "自定义",
        RegionPreset::Shanghai => "上海",
        RegionPreset::NewYork => "纽约",
        RegionPreset::Berlin => "柏林",
        RegionPreset::Moscow => "莫斯科",
    }
}

pub fn locale_label(locale: LocalePolicy) -> &'static str {
    match locale {
        LocalePolicy::System => "跟随系统",
        LocalePolicy::ZhCn => "简体中文",
        LocalePolicy::EnUs => "English (US)",
        LocalePolicy::RuRu => "Русский",
        LocalePolicy::DeDe => "Deutsch",
    }
}

pub fn timezone_label(policy: &TimezonePolicy) -> String {
    if policy.is_system() {
        "跟随系统".to_owned()
    } else {
        policy.cdp_value().unwrap_or("system").to_owned()
    }
}

pub fn webrtc_label(policy: WebRtcPolicy) -> &'static str {
    match policy {
        WebRtcPolicy::Native => "内核原生",
        WebRtcPolicy::DisableNonProxiedUdp => "禁止直连 UDP",
    }
}

pub fn field_label(field: &str) -> &'static str {
    match field {
        "region.language" => "语言",
        "region.timezone" => "时区",
        "region.geolocation" => "地理位置",
        "browser.userAgent" => "UA / UA-CH",
        "browser.platform" => "操作系统",
        "display.window" => "窗口尺寸",
        "display.viewport" => "Viewport",
        "display.screen" => "屏幕",
        "display.devicePixelRatio" => "像素比",
        "hardware.cpu" => "CPU",
        "hardware.deviceMemory" => "内存",
        "hardware.touch" => "触控",
        "browser.plugins" => "Plugins",
        "browser.battery" => "Battery",
        "graphics.canvas" => "Canvas",
        "graphics.webglImage" => "WebGL 图像",
        "graphics.webglMetadata" => "GPU 信息",
        "graphics.webgpu" => "WebGPU",
        "graphics.clientRects" => "ClientRects",
        "media.audio" => "Audio",
        "media.fonts" => "字体",
        "media.mediaDevices" => "媒体设备",
        "media.speechVoices" => "语音列表",
        "privacy.webrtc" => "WebRTC",
        "privacy.permissions" => "权限",
        "privacy.doNotTrack" => "Do Not Track",
        "privacy.globalPrivacyControl" => "GPC",
        _ => "字段",
    }
}

pub fn readonly_field_value(field: &str, canvas_kernel: bool) -> &'static str {
    match field {
        "graphics.canvas" if canvas_kernel => "种子微扰",
        "graphics.webglImage"
        | "graphics.webglMetadata"
        | "graphics.webgpu"
        | "graphics.clientRects" => "内核原生",
        "media.fonts" => "系统字体",
        "media.mediaDevices" => "站点权限",
        "media.speechVoices" => "系统语音",
        "privacy.permissions" => "独立 Profile",
        "hardware.cpu" | "hardware.deviceMemory" | "hardware.touch" => "本机硬件",
        "browser.userAgent" | "browser.platform" | "browser.plugins" | "browser.battery" => {
            "内核原生"
        }
        "region.geolocation" => "未单独声明",
        _ => "内核原生",
    }
}

pub fn relative_time(updated_at_ms: i64, now_ms: i64) -> String {
    let minutes = (updated_at_ms - now_ms) / 60_000;
    let minutes = minutes.clamp(-10_000, 10_000);
    if minutes.abs() < 60 {
        return if minutes == 0 {
            "刚刚".to_owned()
        } else if minutes < 0 {
            format!("{} 分钟前", -minutes)
        } else {
            format!("{} 分钟后", minutes)
        };
    }
    let hours = minutes / 60;
    if hours.abs() < 24 {
        return if hours < 0 {
            format!("{} 小时前", -hours)
        } else {
            format!("{} 小时后", hours)
        };
    }
    let days = hours / 24;
    if days < 0 {
        format!("{} 天前", -days)
    } else {
        format!("{} 天后", days)
    }
}

pub fn apply_preset(persona: &PersonaConfig, preset: RegionPreset) -> PersonaConfig {
    let mut next = persona.clone();
    next.apply_region_preset(preset);
    next
}

pub fn apply_locale(persona: &PersonaConfig, locale: LocalePolicy) -> PersonaConfig {
    let mut next = persona.clone();
    next.locale = locale;
    sync_region(&mut next);
    next
}

pub fn apply_timezone(persona: &PersonaConfig, timezone: TimezonePolicy) -> PersonaConfig {
    let mut next = persona.clone();
    next.timezone = timezone;
    sync_region(&mut next);
    next
}

pub fn apply_window(persona: &PersonaConfig, width: u16, height: u16) -> PersonaConfig {
    let mut next = persona.clone();
    next.window_width = width;
    next.window_height = height;
    next
}

pub fn apply_viewport(
    persona: &PersonaConfig,
    width: Option<u16>,
    height: Option<u16>,
) -> PersonaConfig {
    let mut next = persona.clone();
    let Some(width) = width else {
        next.display_metrics = None;
        return next;
    };
    let height = height.unwrap_or(width);
    let current = next
        .display_metrics
        .unwrap_or(DisplayMetrics::desktop(1366, 768, 1920, 1080, 100));
    next.window_width = next.window_width.max(width);
    next.window_height = next.window_height.max(height);
    next.display_metrics = Some(DisplayMetrics::desktop(
        width,
        height,
        current.screen_width.max(width),
        current.screen_height.max(height),
        current.device_scale_factor_percent,
    ));
    next
}

pub fn apply_screen(persona: &PersonaConfig, width: u16, height: u16) -> PersonaConfig {
    let mut next = persona.clone();
    if let Some(metrics) = next.display_metrics.as_mut() {
        metrics.screen_width = width;
        metrics.screen_height = height;
    }
    next
}

pub fn apply_scale(persona: &PersonaConfig, percent: u16) -> PersonaConfig {
    let mut next = persona.clone();
    if let Some(metrics) = next.display_metrics.as_mut() {
        metrics.device_scale_factor_percent = percent;
    }
    next
}

pub fn apply_webrtc(persona: &PersonaConfig, policy: WebRtcPolicy) -> PersonaConfig {
    let mut next = persona.clone();
    next.webrtc = policy;
    next
}

pub fn reset_native(persona: &PersonaConfig) -> PersonaConfig {
    PersonaConfig::native(persona.seed)
}

pub fn network_direct() -> NetworkConfig {
    NetworkConfig::direct()
}

pub fn network_proxy(protocol: ProxyProtocol, host: String, port: u16) -> NetworkConfig {
    NetworkConfig {
        schema_version: NETWORK_SCHEMA_VERSION,
        mode: NetworkMode::Proxy,
        proxy: Some(ProxyConfig {
            protocol,
            host,
            port,
        }),
    }
}

pub fn persona_is_native(persona: &PersonaConfig) -> bool {
    persona.mode() == PersonaMode::Native
}

fn sync_region(persona: &mut PersonaConfig) {
    persona.region_preset = if persona.locale == LocalePolicy::System
        && persona.timezone.is_system()
        && persona.geolocation.is_none()
    {
        RegionPreset::Native
    } else {
        RegionPreset::Custom
    };
}

#[cfg(test)]
mod tests {
    use browser_persona::{LocalePolicy, PersonaConfig, PersonaMode, RegionPreset};

    use super::{apply_locale, apply_viewport, reset_native};

    #[test]
    fn locale_change_marks_custom_and_native_reset_restores_seed() {
        let persona = PersonaConfig::native(7);
        let managed = apply_locale(&persona, LocalePolicy::ZhCn);
        assert_eq!(managed.region_preset, RegionPreset::Custom);
        assert_eq!(managed.mode(), PersonaMode::Managed);
        let restored = reset_native(&managed);
        assert_eq!(restored.seed, 7);
        assert_eq!(restored.mode(), PersonaMode::Native);
    }

    #[test]
    fn clearing_viewport_returns_to_native_display() {
        let persona = apply_viewport(&PersonaConfig::native(1), Some(1280), Some(720));
        assert!(persona.display_metrics.is_some());
        let cleared = apply_viewport(&persona, None, None);
        assert!(cleared.display_metrics.is_none());
    }
}
