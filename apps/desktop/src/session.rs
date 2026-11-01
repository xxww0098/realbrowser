use std::{path::PathBuf, sync::Arc};

use browser_control::{ControlService, SystemClock, UuidGenerator, ensure_data_root};
use browser_persona_runtime::CdpPersonaRuntime;
use browser_platform::SystemBrowserHost;
use browser_storage::SqliteIdentityStore;

pub fn open_control() -> Result<ControlService, String> {
    let root = ensure_data_root(&data_root()).map_err(|error| error.public_message().to_owned())?;
    let store = SqliteIdentityStore::open(&root.join("state.sqlite3"))
        .map_err(|_| "本地数据暂时不可用".to_owned())?;
    let store = Arc::new(store);
    let control = ControlService::new(
        store.clone(),
        store,
        Arc::new(SystemBrowserHost::new()),
        Arc::new(CdpPersonaRuntime::new()),
        Arc::new(SystemClock),
        Arc::new(UuidGenerator),
        root,
    );
    control
        .reconcile()
        .map_err(|error| error.public_message().to_owned())?;
    Ok(control)
}

fn data_root() -> PathBuf {
    if let Some(path) = std::env::var_os("REALBROWSER_DEV_DATA_ROOT") {
        return PathBuf::from(path);
    }
    if let Some(path) = std::env::var_os("XDG_DATA_HOME") {
        return PathBuf::from(path).join("realbrowser");
    }
    let home = std::env::var_os("HOME")
        .map(PathBuf::from)
        .unwrap_or_else(|| PathBuf::from("."));
    if cfg!(target_os = "macos") {
        home.join("Library/Application Support/realbrowser")
    } else if cfg!(target_os = "windows") {
        std::env::var_os("APPDATA")
            .map(PathBuf::from)
            .unwrap_or(home)
            .join("realbrowser")
    } else {
        home.join(".local/share/realbrowser")
    }
}
