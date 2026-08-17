use std::{thread, time::Duration};

#[cfg(target_os = "macos")]
use std::{process::Command, time::Instant};

use browser_core::{BrowserHost, BrowserRuntimeRecord, IdentityId, LaunchPlan};
use browser_platform::SystemBrowserHost;
use browser_profile::prepare_profile_root;
use tempfile::TempDir;

#[test]
#[ignore = "requires packaged RealBrowser product Chromium and launches a native process"]
fn reconciles_only_the_realbrowser_process_with_the_expected_profile() {
    let root = TempDir::new().unwrap();
    let data_root = root.path().canonicalize().unwrap();
    let identity_id = IdentityId::parse("00000000-0000-0000-0000-000000000151").unwrap();
    let profile_root = prepare_profile_root(&data_root, &identity_id.to_string()).unwrap();
    let first_host = SystemBrowserHost::new();
    let browser = first_host.detect_product_kernel().unwrap();
    let plan = LaunchPlan {
        identity_id,
        executable: browser.executable.clone(),
        profile_root: profile_root.clone(),
        startup_url: "about:blank".to_owned(),
        browser_version: browser.version.clone(),
        arguments: vec![
            kernel_persona_argument(&profile_root, identity_id, browser.engine_major),
            "--headless=new".to_owned(),
            "--disable-gpu".to_owned(),
        ],
        environment: Vec::new(),
    };
    let session = first_host.start(&plan, 1_700_000_000_000).unwrap();
    thread::sleep(Duration::from_millis(500));
    assert!(first_host.is_running(identity_id));
    let runtime = BrowserRuntimeRecord {
        identity_id,
        pid: session.pid,
        executable: browser.executable,
        profile_root,
        browser_version: session.browser_version,
        started_at_ms: session.started_at_ms,
    };

    drop(first_host);
    let second_host = SystemBrowserHost::new();
    let recovered = second_host.reconcile(&runtime).unwrap().unwrap();
    assert_eq!(recovered.pid, runtime.pid);
    assert!(second_host.is_running(identity_id));

    second_host.stop(identity_id).unwrap();
    thread::sleep(Duration::from_millis(150));
    assert!(!second_host.is_running(identity_id));
}

#[cfg(target_os = "macos")]
#[test]
#[ignore = "requires packaged RealBrowser product Chromium and opens a native window"]
fn stopping_managed_realbrowser_removes_its_dock_application() {
    let root = TempDir::new().unwrap();
    let data_root = root.path().canonicalize().unwrap();
    let identity_id = IdentityId::parse("00000000-0000-0000-0000-000000000152").unwrap();
    let profile_root = prepare_profile_root(&data_root, &identity_id.to_string()).unwrap();
    let host = SystemBrowserHost::new();
    let browser = host.detect_product_kernel().unwrap();
    let persona_argument =
        kernel_persona_argument(&profile_root, identity_id, browser.engine_major);
    let plan = LaunchPlan {
        identity_id,
        executable: browser.executable,
        profile_root,
        startup_url: "about:blank".to_owned(),
        browser_version: browser.version,
        arguments: vec![persona_argument],
        environment: Vec::new(),
    };
    let session = host.start(&plan, 1_700_000_000_000).unwrap();

    assert!(wait_until(Duration::from_secs(5), || {
        process_has_argument(session.pid, "--disable-background-mode")
    }));
    assert!(wait_until(Duration::from_secs(5), || {
        launch_services_has_pid(session.pid)
    }));

    host.stop(identity_id).unwrap();

    assert!(wait_until(Duration::from_secs(5), || {
        !launch_services_has_pid(session.pid)
    }));
    assert!(!host.is_running(identity_id));
}

#[cfg(target_os = "macos")]
fn process_has_argument(pid: u32, expected: &str) -> bool {
    Command::new("ps")
        .args(["-o", "command=", "-p", pid.to_string().as_str()])
        .output()
        .is_ok_and(|output| {
            String::from_utf8_lossy(&output.stdout)
                .split_whitespace()
                .any(|argument| argument == expected)
        })
}

#[cfg(target_os = "macos")]
fn launch_services_has_pid(pid: u32) -> bool {
    Command::new("lsappinfo")
        .args([
            "info",
            "-only",
            "bundleID",
            "-app",
            pid.to_string().as_str(),
        ])
        .output()
        .is_ok_and(|output| {
            String::from_utf8_lossy(&output.stdout).contains("com.realbrowser.browser")
        })
}

fn kernel_persona_argument(
    profile_root: &std::path::Path,
    identity_id: IdentityId,
    engine_major: u16,
) -> String {
    let directory = profile_root.join(".realbrowser");
    std::fs::create_dir_all(&directory).unwrap();
    let path = directory.join("persona.json");
    let persona = serde_json::json!({
        "schema_version": 1,
        "persona_id": identity_id.to_string(),
        "seed": "00".repeat(32),
        "engine_major": engine_major,
        "surfaces": { "canvas": { "mode": "seeded_noise" } }
    });
    std::fs::write(&path, serde_json::to_vec_pretty(&persona).unwrap()).unwrap();
    format!("--realbrowser-persona-file={}", path.display())
}

#[cfg(target_os = "macos")]
fn wait_until(timeout: Duration, mut predicate: impl FnMut() -> bool) -> bool {
    let deadline = Instant::now() + timeout;
    while Instant::now() < deadline {
        if predicate() {
            return true;
        }
        thread::sleep(Duration::from_millis(50));
    }
    predicate()
}
