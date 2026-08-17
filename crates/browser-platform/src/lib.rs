#![forbid(unsafe_code)]

use std::{
    collections::HashMap,
    fs::File,
    io::Read,
    path::{Path, PathBuf},
    process::{Child, Command, Stdio},
    thread,
    time::{Duration, Instant},
};

use browser_core::{
    BrowserHost, BrowserInstall, BrowserRuntimeRecord, BrowserSession, IdentityId, LaunchPlan,
    TerminationKind,
};
use parking_lot::Mutex;
#[cfg(unix)]
use rustix::{
    io::Errno,
    process::{Pid as UnixPid, Signal, getpgid, kill_process_group, test_kill_process_group},
};
use serde::Deserialize;
use sha2::{Digest, Sha256};
#[cfg(unix)]
use std::os::unix::process::CommandExt;
#[cfg(unix)]
use sysinfo::Signal as SysinfoSignal;
use sysinfo::{Pid, ProcessesToUpdate, System};

const DEFAULT_GRACE_PERIOD: Duration = Duration::from_secs(5);
const FORCE_STOP_TIMEOUT: Duration = Duration::from_secs(5);
const STOP_POLL_INTERVAL: Duration = Duration::from_millis(50);
const PRODUCT_KERNEL_MANIFEST: &str = "realbrowser-kernel.json";
const PRODUCT_KERNEL_ID: &str = "com.realbrowser.browser";
const PRODUCT_KERNEL_SCHEMA_VERSION: u16 = 1;
const MANAGED_CHROMIUM_ARGUMENTS: [&str; 3] = [
    "--no-first-run",
    "--no-default-browser-check",
    "--disable-background-mode",
];

pub struct SystemBrowserHost {
    children: Mutex<HashMap<IdentityId, TrackedBrowser>>,
    grace_period: Duration,
    kernel_root: Option<PathBuf>,
}

enum TrackedBrowser {
    Spawned(SpawnedBrowser),
    Reconciled(BrowserRuntimeRecord),
}

struct SpawnedBrowser {
    child: Child,
    #[cfg(unix)]
    executable: PathBuf,
    #[cfg(unix)]
    profile_root: PathBuf,
    #[cfg(unix)]
    owns_process_group: bool,
}

impl SystemBrowserHost {
    #[must_use]
    pub fn new() -> Self {
        Self {
            children: Mutex::new(HashMap::new()),
            grace_period: DEFAULT_GRACE_PERIOD,
            kernel_root: configured_kernel_root(),
        }
    }

    #[must_use]
    pub fn with_kernel_root(kernel_root: PathBuf) -> Self {
        Self {
            children: Mutex::new(HashMap::new()),
            grace_period: DEFAULT_GRACE_PERIOD,
            kernel_root: Some(kernel_root),
        }
    }
}

impl Default for SystemBrowserHost {
    fn default() -> Self {
        Self::new()
    }
}

impl BrowserHost for SystemBrowserHost {
    fn detect_product_kernel(&self) -> Result<BrowserInstall, String> {
        let kernel_root = self
            .kernel_root
            .as_deref()
            .ok_or_else(|| "未安装 RealBrowser 产品内核".to_owned())?;
        load_product_kernel(kernel_root)
    }

    fn start(&self, plan: &LaunchPlan, now_ms: i64) -> Result<BrowserSession, String> {
        let installed = self.detect_product_kernel()?;
        if !same_canonical_path(&installed.executable, &plan.executable)
            || installed.version != plan.browser_version
        {
            return Err("启动计划与已验证的 RealBrowser 产品内核不一致".to_owned());
        }
        let mut children = self.children.lock();
        if children.contains_key(&plan.identity_id) {
            return Err("这个环境已由本地服务持有".to_owned());
        }
        std::fs::create_dir_all(&plan.profile_root)
            .map_err(|error| format!("无法创建 Profile 目录：{error}"))?;
        let user_data_argument = format!("--user-data-dir={}", plan.profile_root.display());
        let mut command = Command::new(&plan.executable);
        command
            .arg(user_data_argument)
            .args(MANAGED_CHROMIUM_ARGUMENTS)
            .args(&plan.arguments)
            .envs(plan.environment.iter().cloned())
            .arg(&plan.startup_url)
            .stdin(Stdio::null())
            .stdout(Stdio::null())
            .stderr(Stdio::null());
        #[cfg(unix)]
        command.process_group(0);
        let child = command
            .spawn()
            .map_err(|error| format!("RealBrowser Chromium 启动失败：{error}"))?;
        let pid = child.id();
        children.insert(
            plan.identity_id,
            TrackedBrowser::Spawned(SpawnedBrowser {
                child,
                #[cfg(unix)]
                executable: plan.executable.clone(),
                #[cfg(unix)]
                profile_root: plan.profile_root.clone(),
                #[cfg(unix)]
                owns_process_group: cfg!(unix),
            }),
        );
        Ok(BrowserSession {
            identity_id: plan.identity_id,
            pid,
            browser_version: plan.browser_version.clone(),
            started_at_ms: now_ms,
        })
    }

    fn reconcile(&self, runtime: &BrowserRuntimeRecord) -> Result<Option<BrowserSession>, String> {
        let installed = self.detect_product_kernel()?;
        if !same_canonical_path(&installed.executable, &runtime.executable)
            || installed.version != runtime.browser_version
        {
            return Err("运行日志不属于当前 RealBrowser 产品内核".to_owned());
        }
        let mut children = self.children.lock();
        if children.contains_key(&runtime.identity_id) {
            return Err("这个环境已经在当前运行时中".to_owned());
        }
        match inspect_runtime_process(runtime) {
            ProcessOwnership::Exited => Ok(None),
            ProcessOwnership::Owned => {
                children.insert(
                    runtime.identity_id,
                    TrackedBrowser::Reconciled(runtime.clone()),
                );
                Ok(Some(runtime.session()))
            }
            ProcessOwnership::Mismatch => {
                Err("运行日志中的进程与 RealBrowser/Profile 所有权不匹配".to_owned())
            }
        }
    }

    fn stop(&self, identity_id: IdentityId) -> Result<TerminationKind, String> {
        let mut children = self.children.lock();
        let Some(tracked) = children.get_mut(&identity_id) else {
            return Ok(TerminationKind::AlreadyExited);
        };
        let result = match tracked {
            TrackedBrowser::Spawned(browser) => stop_spawned(browser, self.grace_period),
            TrackedBrowser::Reconciled(runtime) => stop_reconciled(runtime, self.grace_period),
        };
        if result.is_ok() {
            children.remove(&identity_id);
        }
        result
    }

    fn is_running(&self, identity_id: IdentityId) -> bool {
        let mut children = self.children.lock();
        let Some(tracked) = children.get_mut(&identity_id) else {
            return false;
        };
        let running = match tracked {
            TrackedBrowser::Spawned(browser) => {
                let parent_running = matches!(browser.child.try_wait(), Ok(None));
                #[cfg(unix)]
                let process_group_running = browser.owns_process_group
                    && unix_process_group_exists(browser.child.id()).unwrap_or(parent_running);
                #[cfg(not(unix))]
                let process_group_running = false;
                parent_running || process_group_running
            }
            TrackedBrowser::Reconciled(runtime) => match inspect_runtime_process(runtime) {
                ProcessOwnership::Owned => true,
                ProcessOwnership::Exited => {
                    profile_processes_exist(&runtime.profile_root, &runtime.executable)
                }
                ProcessOwnership::Mismatch => false,
            },
        };
        if !running {
            children.remove(&identity_id);
        }
        running
    }
}

fn stop_spawned(
    browser: &mut SpawnedBrowser,
    grace_period: Duration,
) -> Result<TerminationKind, String> {
    #[cfg(not(unix))]
    let _ = grace_period;
    let parent_exited = browser
        .child
        .try_wait()
        .map_err(|error| format!("无法读取 RealBrowser 进程状态：{error}"))?
        .is_some();
    #[cfg(unix)]
    let process_group_running =
        browser.owns_process_group && unix_process_group_exists(browser.child.id())?;
    #[cfg(not(unix))]
    let process_group_running = false;
    if parent_exited && !process_group_running {
        return Ok(TerminationKind::AlreadyExited);
    }

    #[cfg(unix)]
    {
        let requested_parent = request_unix_termination(browser.child.id())?;
        let requested = requested_parent
            || (browser.owns_process_group
                && request_unix_process_group_signal(browser.child.id(), Signal::TERM)?);
        if !requested {
            return wait_for_spawned_exit(browser, STOP_POLL_INTERVAL)?
                .then_some(TerminationKind::AlreadyExited)
                .ok_or_else(|| "无法定位待停止的 RealBrowser 进程".to_owned());
        }
        if wait_for_spawned_exit(browser, grace_period)? {
            return Ok(TerminationKind::Graceful);
        }
        if browser.owns_process_group
            && request_unix_process_group_signal(browser.child.id(), Signal::KILL)?
        {
            if !wait_for_spawned_exit(browser, FORCE_STOP_TIMEOUT)? {
                return Err("RealBrowser 进程组未在强制停止超时内退出".to_owned());
            }
            return Ok(TerminationKind::Forced);
        }
        if browser.owns_process_group {
            force_profile_processes(&browser.profile_root, &browser.executable)?;
            if wait_for_spawned_exit(browser, FORCE_STOP_TIMEOUT)? {
                return Ok(TerminationKind::Forced);
            }
        }
    }

    browser
        .child
        .kill()
        .map_err(|error| format!("无法强制停止 RealBrowser：{error}"))?;
    browser
        .child
        .wait()
        .map_err(|error| format!("等待 RealBrowser 强制退出失败：{error}"))?;
    Ok(TerminationKind::Forced)
}

fn stop_reconciled(
    runtime: &BrowserRuntimeRecord,
    grace_period: Duration,
) -> Result<TerminationKind, String> {
    #[cfg(not(unix))]
    let _ = grace_period;
    let ownership = inspect_runtime_process(runtime);
    match ownership {
        ProcessOwnership::Exited
            if !profile_processes_exist(&runtime.profile_root, &runtime.executable) =>
        {
            return Ok(TerminationKind::AlreadyExited);
        }
        ProcessOwnership::Exited | ProcessOwnership::Owned => {}
        ProcessOwnership::Mismatch => return Err("拒绝停止未经验证的进程".to_owned()),
    }

    #[cfg(unix)]
    {
        let requested = match ownership {
            ProcessOwnership::Owned => request_unix_termination(runtime.pid)?,
            ProcessOwnership::Exited => request_profile_process_signal(
                &runtime.profile_root,
                &runtime.executable,
                SysinfoSignal::Term,
            )?,
            ProcessOwnership::Mismatch => false,
        };
        if !requested {
            return match (
                inspect_runtime_process(runtime),
                profile_processes_exist(&runtime.profile_root, &runtime.executable),
            ) {
                (ProcessOwnership::Exited, false) => Ok(TerminationKind::AlreadyExited),
                (ProcessOwnership::Owned, _) => Err("无法定位待停止的 RealBrowser 进程".to_owned()),
                (ProcessOwnership::Mismatch, _) => Err("停止期间进程所有权发生变化".to_owned()),
                (ProcessOwnership::Exited, true) => {
                    Err("无法请求已恢复的 RealBrowser 子进程退出".to_owned())
                }
            };
        }
        if wait_for_runtime_exit(runtime, grace_period)? {
            return Ok(TerminationKind::Graceful);
        }
        force_profile_processes(&runtime.profile_root, &runtime.executable)?;
        if !wait_for_runtime_exit(runtime, FORCE_STOP_TIMEOUT)? {
            return Err("RealBrowser Profile 进程未在强制停止超时内退出".to_owned());
        }
        Ok(TerminationKind::Forced)
    }

    #[cfg(not(unix))]
    {
        force_verified_runtime(runtime)?;
        if !wait_for_runtime_exit(runtime, FORCE_STOP_TIMEOUT)? {
            return Err("RealBrowser 未在强制停止超时内退出".to_owned());
        }
        Ok(TerminationKind::Forced)
    }
}

#[cfg(unix)]
fn request_unix_termination(pid: u32) -> Result<bool, String> {
    let mut system = System::new_all();
    let pid = Pid::from_u32(pid);
    system.refresh_processes(ProcessesToUpdate::Some(&[pid]), true);
    let Some(process) = system.process(pid) else {
        return Ok(false);
    };
    match process.kill_with(SysinfoSignal::Term) {
        Some(true) => Ok(true),
        Some(false) => Err("无法请求 RealBrowser 优雅退出".to_owned()),
        None => Err("当前平台不支持 RealBrowser 优雅退出信号".to_owned()),
    }
}

#[cfg(unix)]
fn request_unix_process_group_signal(pid: u32, signal: Signal) -> Result<bool, String> {
    let Some(pid) = UnixPid::from_raw(pid.cast_signed()) else {
        return Err("RealBrowser PID 超出当前平台范围".to_owned());
    };
    if getpgid(Some(pid)).is_ok_and(|process_group| process_group != pid) {
        return Ok(false);
    }
    match kill_process_group(pid, signal) {
        Ok(()) => Ok(true),
        Err(Errno::PERM | Errno::SRCH) => Ok(false),
        Err(error) => Err(format!("无法向 RealBrowser 进程组发送信号：{error}")),
    }
}

#[cfg(unix)]
fn unix_process_group_exists(pid: u32) -> Result<bool, String> {
    let Some(pid) = UnixPid::from_raw(pid.cast_signed()) else {
        return Err("RealBrowser PID 超出当前平台范围".to_owned());
    };
    match test_kill_process_group(pid) {
        Ok(()) | Err(Errno::PERM) => Ok(true),
        Err(Errno::SRCH) => Ok(false),
        Err(error) => Err(format!("无法读取 RealBrowser 进程组状态：{error}")),
    }
}

#[cfg(not(unix))]
fn force_verified_runtime(runtime: &BrowserRuntimeRecord) -> Result<(), String> {
    if inspect_runtime_process(runtime) != ProcessOwnership::Owned {
        return Err("拒绝强制停止未经验证的进程".to_owned());
    }
    let mut system = System::new_all();
    let pid = Pid::from_u32(runtime.pid);
    system.refresh_processes(ProcessesToUpdate::Some(&[pid]), true);
    match system.process(pid) {
        Some(process) if process.kill() => Ok(()),
        Some(_) => Err("无法强制停止已恢复的 RealBrowser 进程".to_owned()),
        None => Ok(()),
    }
}

#[cfg(unix)]
fn request_profile_process_signal(
    profile_root: &std::path::Path,
    product_executable: &std::path::Path,
    signal: SysinfoSignal,
) -> Result<bool, String> {
    let mut system = System::new_all();
    system.refresh_processes(ProcessesToUpdate::All, true);
    let matching = profile_process_ids(&system, profile_root, product_executable);
    let mut requested = false;
    let mut rejected = false;
    for pid in matching {
        let Some(process) = system.process(pid) else {
            continue;
        };
        match process.kill_with(signal) {
            Some(true) => requested = true,
            Some(false) | None => rejected = true,
        }
    }
    if requested || !rejected {
        Ok(requested)
    } else {
        Err("无法请求已验证的 RealBrowser Profile 进程退出".to_owned())
    }
}

#[cfg(unix)]
fn force_profile_processes(
    profile_root: &std::path::Path,
    product_executable: &std::path::Path,
) -> Result<(), String> {
    let mut system = System::new_all();
    system.refresh_processes(ProcessesToUpdate::All, true);
    let matching = profile_process_ids(&system, profile_root, product_executable);
    let mut rejected = false;
    for pid in matching {
        if system.process(pid).is_some_and(|process| !process.kill()) {
            rejected = true;
        }
    }
    if rejected {
        Err("无法强制停止已验证的 RealBrowser Profile 进程".to_owned())
    } else {
        Ok(())
    }
}

fn profile_processes_exist(
    profile_root: &std::path::Path,
    product_executable: &std::path::Path,
) -> bool {
    let mut system = System::new_all();
    system.refresh_processes(ProcessesToUpdate::All, true);
    !profile_process_ids(&system, profile_root, product_executable).is_empty()
}

fn profile_process_ids(
    system: &System,
    profile_root: &std::path::Path,
    product_executable: &std::path::Path,
) -> Vec<Pid> {
    let expected = format!("--user-data-dir={}", profile_root.display());
    system
        .processes()
        .iter()
        .filter_map(|(pid, process)| {
            let profile_matches = process
                .cmd()
                .iter()
                .any(|argument| argument == expected.as_str());
            (profile_matches
                && process.exe().is_some_and(|actual| {
                    product_process_executable_matches(actual, product_executable)
                }))
            .then_some(*pid)
        })
        .collect()
}

#[cfg(target_os = "macos")]
fn product_process_executable_matches(actual: &Path, product_executable: &Path) -> bool {
    let Some(bundle_root) = product_executable
        .ancestors()
        .find(|path| path.extension().is_some_and(|extension| extension == "app"))
        .and_then(|path| path.canonicalize().ok())
    else {
        return false;
    };
    actual
        .canonicalize()
        .is_ok_and(|actual| actual.starts_with(bundle_root))
}

#[cfg(not(target_os = "macos"))]
fn product_process_executable_matches(actual: &Path, product_executable: &Path) -> bool {
    same_canonical_path(actual, product_executable)
}

#[cfg(unix)]
fn wait_for_spawned_exit(browser: &mut SpawnedBrowser, timeout: Duration) -> Result<bool, String> {
    let deadline = Instant::now() + timeout;
    loop {
        let child_exited = browser
            .child
            .try_wait()
            .map_err(|error| format!("无法读取 RealBrowser 进程状态：{error}"))?
            .is_some();
        let process_group_exited =
            !browser.owns_process_group || !unix_process_group_exists(browser.child.id())?;
        if child_exited && process_group_exited {
            return Ok(true);
        }
        if Instant::now() >= deadline {
            return Ok(false);
        }
        thread::sleep(STOP_POLL_INTERVAL.min(deadline.saturating_duration_since(Instant::now())));
    }
}

fn wait_for_runtime_exit(
    runtime: &BrowserRuntimeRecord,
    timeout: Duration,
) -> Result<bool, String> {
    let deadline = Instant::now() + timeout;
    loop {
        match inspect_runtime_process(runtime) {
            ProcessOwnership::Exited => {
                if !profile_processes_exist(&runtime.profile_root, &runtime.executable) {
                    return Ok(true);
                }
            }
            ProcessOwnership::Mismatch => {
                return Err("停止期间进程所有权发生变化".to_owned());
            }
            ProcessOwnership::Owned => {}
        }
        if Instant::now() >= deadline {
            return Ok(false);
        }
        thread::sleep(STOP_POLL_INTERVAL.min(deadline.saturating_duration_since(Instant::now())));
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum ProcessOwnership {
    Exited,
    Owned,
    Mismatch,
}

fn inspect_runtime_process(runtime: &BrowserRuntimeRecord) -> ProcessOwnership {
    let mut system = System::new_all();
    let pid = Pid::from_u32(runtime.pid);
    system.refresh_processes(ProcessesToUpdate::Some(&[pid]), true);
    let Some(process) = system.process(pid) else {
        return ProcessOwnership::Exited;
    };
    let executable_matches = process
        .exe()
        .is_some_and(|actual| same_canonical_path(actual, &runtime.executable));
    let expected_profile_argument = format!("--user-data-dir={}", runtime.profile_root.display());
    let profile_matches = process
        .cmd()
        .iter()
        .any(|argument| argument == expected_profile_argument.as_str());
    if executable_matches && profile_matches {
        ProcessOwnership::Owned
    } else {
        ProcessOwnership::Mismatch
    }
}

fn same_canonical_path(actual: &std::path::Path, expected: &std::path::Path) -> bool {
    match (actual.canonicalize(), expected.canonicalize()) {
        (Ok(actual), Ok(expected)) => actual == expected,
        _ => actual == expected,
    }
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct ProductKernelManifest {
    schema_version: u16,
    product_id: String,
    version: String,
    engine_major: u16,
    executable: PathBuf,
    executable_sha256: String,
}

fn configured_kernel_root() -> Option<PathBuf> {
    if let Some(root) = std::env::var_os("REALBROWSER_KERNEL_DIR") {
        return Some(PathBuf::from(root));
    }

    let executable = std::env::current_exe().ok()?;
    #[cfg(target_os = "macos")]
    {
        let macos_dir = executable.parent()?;
        let contents_dir = macos_dir.parent()?;
        Some(contents_dir.join("Resources/realbrowser-kernel"))
    }
    #[cfg(not(target_os = "macos"))]
    {
        executable
            .parent()
            .map(|directory| directory.join("realbrowser-kernel"))
    }
}

fn load_product_kernel(kernel_root: &Path) -> Result<BrowserInstall, String> {
    let root = kernel_root
        .canonicalize()
        .map_err(|_| "未安装 RealBrowser 产品内核".to_owned())?;
    let manifest_path = root.join(PRODUCT_KERNEL_MANIFEST);
    let manifest_bytes = std::fs::read(&manifest_path)
        .map_err(|error| format!("无法读取 RealBrowser 产品内核清单：{error}"))?;
    let manifest: ProductKernelManifest = serde_json::from_slice(&manifest_bytes)
        .map_err(|error| format!("RealBrowser 产品内核清单无效：{error}"))?;
    if manifest.schema_version != PRODUCT_KERNEL_SCHEMA_VERSION
        || manifest.product_id != PRODUCT_KERNEL_ID
    {
        return Err("RealBrowser 产品内核清单版本或产品标识不受支持".to_owned());
    }
    if manifest.executable.is_absolute() {
        return Err("RealBrowser 产品内核清单包含绝对可执行路径".to_owned());
    }
    let executable = root
        .join(&manifest.executable)
        .canonicalize()
        .map_err(|_| {
            "RealBrowser 产品内核可执行文件不存在；不会回退到本机 Google Chrome".to_owned()
        })?;
    if !executable.starts_with(&root) || !executable.is_file() {
        return Err("RealBrowser 产品内核可执行路径越出产品目录".to_owned());
    }
    let expected_major = manifest
        .version
        .split('.')
        .next()
        .and_then(|value| value.parse::<u16>().ok())
        .ok_or_else(|| "RealBrowser 产品内核版本无效".to_owned())?;
    if expected_major != manifest.engine_major {
        return Err("RealBrowser 产品内核 major 与清单版本不一致".to_owned());
    }
    verify_executable_hash(&executable, &manifest.executable_sha256)?;

    let output = Command::new(&executable)
        .arg("--version")
        .output()
        .map_err(|error| format!("无法读取 RealBrowser Chromium 版本：{error}"))?;
    let stdout = String::from_utf8_lossy(&output.stdout).trim().to_owned();
    let stderr = String::from_utf8_lossy(&output.stderr).trim().to_owned();
    let reported = if stdout.is_empty() { stderr } else { stdout };
    if reported != format!("RealBrowser {}", manifest.version) {
        return Err("产品内核版本输出与清单不一致，或错误指向 Google Chrome".to_owned());
    }

    Ok(BrowserInstall {
        executable,
        version: manifest.version,
        engine_major: manifest.engine_major,
    })
}

fn verify_executable_hash(executable: &Path, expected: &str) -> Result<(), String> {
    if expected.len() != 64 || !expected.bytes().all(|byte| byte.is_ascii_hexdigit()) {
        return Err("RealBrowser 产品内核 SHA-256 无效".to_owned());
    }
    let mut file = File::open(executable)
        .map_err(|error| format!("无法校验 RealBrowser 产品内核：{error}"))?;
    let mut hasher = Sha256::new();
    let mut buffer = [0_u8; 64 * 1024];
    loop {
        let read = file
            .read(&mut buffer)
            .map_err(|error| format!("无法校验 RealBrowser 产品内核：{error}"))?;
        if read == 0 {
            break;
        }
        hasher.update(&buffer[..read]);
    }
    let actual = format!("{:x}", hasher.finalize());
    if !actual.eq_ignore_ascii_case(expected) {
        return Err("RealBrowser 产品内核 SHA-256 与清单不一致".to_owned());
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use std::{
        collections::HashMap,
        process::{Command, Stdio},
        thread,
        time::Duration,
    };

    use browser_core::{
        BrowserHost, BrowserRuntimeRecord, IdentityId, LaunchPlan, TerminationKind,
    };
    use parking_lot::Mutex;
    #[cfg(unix)]
    use sha2::{Digest, Sha256};
    #[cfg(unix)]
    use std::os::unix::process::CommandExt;

    #[cfg(unix)]
    use super::unix_process_group_exists;
    use super::{
        ProcessOwnership, SpawnedBrowser, SystemBrowserHost, TrackedBrowser,
        inspect_runtime_process, profile_process_ids,
    };

    #[test]
    fn missing_product_kernel_fails_closed_without_system_browser_fallback() {
        let root = tempfile::TempDir::new().unwrap();
        let host = SystemBrowserHost::with_kernel_root(root.path().join("missing"));

        let error = host.detect_product_kernel().unwrap_err();

        assert!(error.contains("未安装 RealBrowser 产品内核"));
    }

    #[cfg(unix)]
    #[test]
    fn accepts_only_a_hashed_realbrowser_kernel_manifest() {
        use std::os::unix::fs::PermissionsExt;

        let root = tempfile::TempDir::new().unwrap();
        let executable = root.path().join("RealBrowser Chromium");
        std::fs::write(
            &executable,
            "#!/bin/sh\nprintf 'RealBrowser 151.0.7922.138\\n'\n",
        )
        .unwrap();
        std::fs::set_permissions(&executable, std::fs::Permissions::from_mode(0o755)).unwrap();
        let digest = format!("{:x}", Sha256::digest(std::fs::read(&executable).unwrap()));
        let manifest = serde_json::json!({
            "schema_version": 1,
            "product_id": "com.realbrowser.browser",
            "version": "151.0.7922.138",
            "engine_major": 151,
            "executable": "RealBrowser Chromium",
            "executable_sha256": digest,
        });
        std::fs::write(
            root.path().join("realbrowser-kernel.json"),
            serde_json::to_vec_pretty(&manifest).unwrap(),
        )
        .unwrap();
        let host = SystemBrowserHost::with_kernel_root(root.path().to_path_buf());

        let install = host.detect_product_kernel().unwrap();

        assert_eq!(install.executable, executable.canonicalize().unwrap());
        assert_eq!(install.version, "151.0.7922.138");
        assert_eq!(install.engine_major, 151);
    }

    #[cfg(unix)]
    #[test]
    fn rejects_a_google_chrome_binary_even_with_a_forged_product_manifest() {
        use std::os::unix::fs::PermissionsExt;

        let root = tempfile::TempDir::new().unwrap();
        let executable = root.path().join("fake-kernel");
        std::fs::write(
            &executable,
            "#!/bin/sh\nprintf 'Google Chrome 151.0.7922.138\\n'\n",
        )
        .unwrap();
        std::fs::set_permissions(&executable, std::fs::Permissions::from_mode(0o755)).unwrap();
        let digest = format!("{:x}", Sha256::digest(std::fs::read(&executable).unwrap()));
        let manifest = serde_json::json!({
            "schema_version": 1,
            "product_id": "com.realbrowser.browser",
            "version": "151.0.7922.138",
            "engine_major": 151,
            "executable": "fake-kernel",
            "executable_sha256": digest,
        });
        std::fs::write(
            root.path().join("realbrowser-kernel.json"),
            serde_json::to_vec_pretty(&manifest).unwrap(),
        )
        .unwrap();
        let host = SystemBrowserHost::with_kernel_root(root.path().to_path_buf());

        assert!(
            host.detect_product_kernel()
                .unwrap_err()
                .contains("错误指向 Google Chrome")
        );
    }

    #[test]
    fn pid_reuse_without_expected_profile_is_not_owned() {
        let runtime = BrowserRuntimeRecord {
            identity_id: IdentityId::parse("00000000-0000-0000-0000-000000000151").unwrap(),
            pid: std::process::id(),
            executable: std::env::current_exe().unwrap(),
            profile_root: "/definitely/not/this/test-process-profile".into(),
            browser_version: "test".to_owned(),
            started_at_ms: 0,
        };

        assert_eq!(
            inspect_runtime_process(&runtime),
            ProcessOwnership::Mismatch
        );
    }

    #[test]
    fn failed_stop_keeps_runtime_tracked_for_retry_or_diagnostics() {
        let identity_id = IdentityId::parse("00000000-0000-0000-0000-000000000152").unwrap();
        let runtime = BrowserRuntimeRecord {
            identity_id,
            pid: std::process::id(),
            executable: std::env::current_exe().unwrap(),
            profile_root: "/definitely/not/this/test-process-profile".into(),
            browser_version: "test".to_owned(),
            started_at_ms: 0,
        };
        let host = SystemBrowserHost::new();
        host.children
            .lock()
            .insert(identity_id, TrackedBrowser::Reconciled(runtime));

        assert!(host.stop(identity_id).is_err());
        assert!(host.children.lock().contains_key(&identity_id));
    }

    #[cfg(unix)]
    #[test]
    fn spawned_process_gets_a_graceful_stop_window() {
        let identity_id = IdentityId::parse("00000000-0000-0000-0000-000000000153").unwrap();
        let child = Command::new("sleep")
            .arg("30")
            .stdin(Stdio::null())
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .spawn()
            .unwrap();
        let host = SystemBrowserHost {
            children: Mutex::new(HashMap::from([(
                identity_id,
                TrackedBrowser::Spawned(SpawnedBrowser {
                    child,
                    executable: std::env::current_exe().unwrap(),
                    profile_root: "/unused/test-profile".into(),
                    owns_process_group: false,
                }),
            )])),
            grace_period: Duration::from_millis(250),
            kernel_root: None,
        };

        assert_eq!(host.stop(identity_id).unwrap(), TerminationKind::Graceful);
        assert!(!host.children.lock().contains_key(&identity_id));
    }

    #[cfg(unix)]
    #[test]
    fn spawned_process_is_forced_after_the_grace_window() {
        let identity_id = IdentityId::parse("00000000-0000-0000-0000-000000000154").unwrap();
        let child = Command::new("sh")
            .args(["-c", "trap '' TERM; while :; do :; done"])
            .stdin(Stdio::null())
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .spawn()
            .unwrap();
        thread::sleep(Duration::from_millis(50));
        let host = SystemBrowserHost {
            children: Mutex::new(HashMap::from([(
                identity_id,
                TrackedBrowser::Spawned(SpawnedBrowser {
                    child,
                    executable: std::env::current_exe().unwrap(),
                    profile_root: "/unused/test-profile".into(),
                    owns_process_group: false,
                }),
            )])),
            grace_period: Duration::from_millis(50),
            kernel_root: None,
        };

        assert_eq!(host.stop(identity_id).unwrap(), TerminationKind::Forced);
        assert!(!host.children.lock().contains_key(&identity_id));
    }

    #[cfg(unix)]
    #[test]
    fn spawned_process_group_is_forced_without_leaving_its_child() {
        let identity_id = IdentityId::parse("00000000-0000-0000-0000-000000000155").unwrap();
        let mut command = Command::new("sh");
        command
            .args([
                "-c",
                "trap '' TERM; (trap '' TERM; while :; do :; done) & wait",
            ])
            .stdin(Stdio::null())
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .process_group(0);
        let child = command.spawn().unwrap();
        let process_group = child.id();
        thread::sleep(Duration::from_millis(50));
        let host = SystemBrowserHost {
            children: Mutex::new(HashMap::from([(
                identity_id,
                TrackedBrowser::Spawned(SpawnedBrowser {
                    child,
                    executable: std::env::current_exe().unwrap(),
                    profile_root: "/unused/test-profile".into(),
                    owns_process_group: true,
                }),
            )])),
            grace_period: Duration::from_millis(50),
            kernel_root: None,
        };

        assert_eq!(host.stop(identity_id).unwrap(), TerminationKind::Forced);
        assert!(!unix_process_group_exists(process_group).unwrap());
        assert!(!host.children.lock().contains_key(&identity_id));
    }

    #[cfg(unix)]
    #[test]
    fn profile_scan_rejects_unknown_process_with_matching_user_data_argument() {
        let root = tempfile::TempDir::new().unwrap();
        let profile_root = root.path().join("profile");
        let profile_argument = format!("--user-data-dir={}", profile_root.display());
        let mut child = Command::new("sh")
            .args([
                "-c",
                "trap 'exit 0' TERM; while :; do :; done",
                "realbrowser-ownership-test",
                &profile_argument,
            ])
            .stdin(Stdio::null())
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .spawn()
            .unwrap();
        thread::sleep(Duration::from_millis(50));
        let mut system = sysinfo::System::new_all();
        system.refresh_processes(sysinfo::ProcessesToUpdate::All, true);

        let matching =
            profile_process_ids(&system, &profile_root, &std::env::current_exe().unwrap());

        child.kill().unwrap();
        child.wait().unwrap();
        assert!(!matching.contains(&sysinfo::Pid::from_u32(child.id())));
    }

    #[cfg(target_os = "macos")]
    #[test]
    fn macos_helper_must_remain_inside_verified_realbrowser_app() {
        let root = tempfile::TempDir::new().unwrap();
        let bundle = root.path().join("RealBrowser.app");
        let product = bundle.join("Contents/MacOS/RealBrowser");
        let helper = bundle
            .join("Contents/Frameworks/RealBrowser Helper.app/Contents/MacOS/RealBrowser Helper");
        std::fs::create_dir_all(product.parent().unwrap()).unwrap();
        std::fs::create_dir_all(helper.parent().unwrap()).unwrap();
        std::fs::write(&product, b"product").unwrap();
        std::fs::write(&helper, b"helper").unwrap();

        assert!(super::product_process_executable_matches(&helper, &product));
        assert!(!super::product_process_executable_matches(
            std::path::Path::new("/bin/sh"),
            &product
        ));
    }

    #[cfg(target_os = "macos")]
    #[test]
    #[ignore = "requires packaged RealBrowser product Chromium and force-stops a native process group"]
    fn managed_realbrowser_zero_grace_stop_removes_every_profile_process() {
        let root = tempfile::TempDir::new().unwrap();
        let profile_root = root.path().join("profile");
        let identity_id = IdentityId::parse("00000000-0000-0000-0000-000000000156").unwrap();
        let host = SystemBrowserHost {
            children: Mutex::new(HashMap::new()),
            grace_period: Duration::ZERO,
            kernel_root: super::configured_kernel_root(),
        };
        let browser = host.detect_product_kernel().unwrap();
        std::fs::create_dir_all(profile_root.join(".realbrowser")).unwrap();
        let persona_path = profile_root.join(".realbrowser/persona.json");
        let persona = serde_json::json!({
            "schema_version": 1,
            "persona_id": identity_id.to_string(),
            "seed": "00".repeat(32),
            "engine_major": browser.engine_major,
            "surfaces": { "canvas": { "mode": "seeded_noise" } }
        });
        std::fs::write(&persona_path, serde_json::to_vec_pretty(&persona).unwrap()).unwrap();
        let plan = LaunchPlan {
            identity_id,
            executable: browser.executable,
            profile_root: profile_root.clone(),
            startup_url: "about:blank".to_owned(),
            browser_version: browser.version,
            arguments: vec![
                format!("--realbrowser-persona-file={}", persona_path.display()),
                "--headless=new".to_owned(),
                "--disable-gpu".to_owned(),
            ],
            environment: Vec::new(),
        };
        host.start(&plan, 1_700_000_000_000).unwrap();
        assert!(wait_until(Duration::from_secs(5), || {
            managed_profile_process_count(&profile_root) > 0
        }));

        assert_eq!(host.stop(identity_id).unwrap(), TerminationKind::Forced);
        assert!(wait_until(Duration::from_secs(5), || {
            managed_profile_process_count(&profile_root) == 0
        }));
        assert!(!host.is_running(identity_id));
    }

    #[cfg(target_os = "macos")]
    fn managed_profile_process_count(profile_root: &std::path::Path) -> usize {
        let expected = format!("--user-data-dir={}", profile_root.display());
        Command::new("ps")
            .args(["ax", "-o", "command="])
            .output()
            .map(|output| {
                String::from_utf8_lossy(&output.stdout)
                    .lines()
                    .filter(|line| line.contains(&expected))
                    .count()
            })
            .unwrap_or_default()
    }

    #[cfg(target_os = "macos")]
    fn wait_until(timeout: Duration, mut predicate: impl FnMut() -> bool) -> bool {
        let deadline = std::time::Instant::now() + timeout;
        while std::time::Instant::now() < deadline {
            if predicate() {
                return true;
            }
            thread::sleep(Duration::from_millis(50));
        }
        predicate()
    }
}
