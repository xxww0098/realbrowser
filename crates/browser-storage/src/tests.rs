use browser_core::{
    BrowserIdentity, BrowserRuntimeRecord, IdentityId, IdentityLifecycle, IdentityStore,
    RuntimeJournal, StoreError,
};
use browser_network::{NetworkConfig, NetworkMode, ProxyConfig, ProxyProtocol};
use browser_persona::{CURRENT_SCHEMA_VERSION, NativeSurfacePolicy, PersonaConfig, TimezonePolicy};
use rusqlite::Connection;
use tempfile::TempDir;
use uuid::Uuid;

use super::SqliteIdentityStore;

fn identity(root: &TempDir) -> BrowserIdentity {
    BrowserIdentity {
        id: IdentityId::new(Uuid::from_u128(7)),
        revision: 1,
        name: "Store A".to_owned(),
        startup_url: None,
        persona: PersonaConfig::default(),
        network: NetworkConfig::direct(),
        lifecycle: IdentityLifecycle::Active,
        profile_root: root.path().join("profile"),
        created_at_ms: 10,
        updated_at_ms: 10,
    }
}

#[test]
fn persists_and_checks_revisions() {
    let root = TempDir::new().unwrap();
    let store = SqliteIdentityStore::open(&root.path().join("state.sqlite3")).unwrap();
    let mut record = identity(&root);
    store.insert(&record).unwrap();
    assert_eq!(
        store.list_by_lifecycle(IdentityLifecycle::Active).unwrap(),
        vec![record.clone()]
    );

    record.name = "Store B".to_owned();
    record.revision = 2;
    store.replace(&record, 1).unwrap();
    assert_eq!(store.get(record.id).unwrap().name, "Store B");
    assert_eq!(store.replace(&record, 1), Err(StoreError::RevisionConflict));
}

#[test]
fn migrates_native_persona_for_existing_identity_rows() {
    let root = TempDir::new().unwrap();
    let database = root.path().join("state.sqlite3");
    let connection = Connection::open(&database).unwrap();
    connection
        .execute_batch(
            "CREATE TABLE browser_identities (
               id TEXT PRIMARY KEY NOT NULL,
               revision INTEGER NOT NULL,
               name TEXT NOT NULL,
               startup_url TEXT,
               lifecycle TEXT NOT NULL,
               profile_root TEXT NOT NULL UNIQUE,
               created_at_ms INTEGER NOT NULL,
               updated_at_ms INTEGER NOT NULL
             );
             INSERT INTO browser_identities VALUES (
               '00000000-0000-0000-0000-000000000007', 1, 'Store A', NULL,
               'active', '/tmp/legacy-profile', 10, 10
             );",
        )
        .unwrap();
    drop(connection);

    let store = SqliteIdentityStore::open(&database).unwrap();
    let loaded = store.get(IdentityId::new(Uuid::from_u128(7))).unwrap();
    assert_eq!(loaded.persona, PersonaConfig::default());
    assert_eq!(loaded.network, NetworkConfig::direct());
}

#[test]
fn persists_proxy_network_config() {
    let root = TempDir::new().unwrap();
    let store = SqliteIdentityStore::open(&root.path().join("state.sqlite3")).unwrap();
    let mut record = identity(&root);
    store.insert(&record).unwrap();
    record.network = NetworkConfig {
        schema_version: 1,
        mode: NetworkMode::Proxy,
        proxy: Some(ProxyConfig {
            protocol: ProxyProtocol::Http,
            host: "proxy.example".to_owned(),
            port: 8080,
        }),
    };
    record.revision = 2;
    store.replace(&record, 1).unwrap();

    assert_eq!(store.get(record.id).unwrap().network, record.network);
}

#[test]
fn migrates_persisted_v1_persona_to_current_without_enabling_surfaces() {
    let root = TempDir::new().unwrap();
    let database = root.path().join("state.sqlite3");
    let store = SqliteIdentityStore::open(&database).unwrap();
    drop(store);
    let connection = Connection::open(&database).unwrap();
    connection
        .execute(
            "INSERT INTO browser_identities
             (id, revision, name, startup_url, persona_json, lifecycle, profile_root, created_at_ms, updated_at_ms)
             VALUES (?1, 1, 'Legacy', NULL, ?2, 'active', ?3, 10, 10)",
            (
                "00000000-0000-0000-0000-000000000008",
                r#"{"schemaVersion":1,"seed":8,"locale":"system","timezone":"system","windowWidth":1440,"windowHeight":900,"webrtc":"native"}"#,
                root.path().join("legacy-v1-profile").to_string_lossy(),
            ),
        )
        .unwrap();
    drop(connection);

    let store = SqliteIdentityStore::open(&database).unwrap();
    let loaded = store.get(IdentityId::new(Uuid::from_u128(8))).unwrap();
    assert_eq!(loaded.persona.schema_version, CURRENT_SCHEMA_VERSION);
    assert_eq!(
        loaded.persona.surfaces.graphics.canvas,
        NativeSurfacePolicy::Native
    );
}

#[test]
fn migrates_persisted_v2_timezone_token_to_iana_id() {
    let root = TempDir::new().unwrap();
    let database = root.path().join("state.sqlite3");
    let store = SqliteIdentityStore::open(&database).unwrap();
    drop(store);
    let connection = Connection::open(&database).unwrap();
    connection
        .execute(
            "INSERT INTO browser_identities
             (id, revision, name, startup_url, persona_json, lifecycle, profile_root, created_at_ms, updated_at_ms)
             VALUES (?1, 1, 'Legacy timezone', NULL, ?2, 'active', ?3, 10, 10)",
            (
                "00000000-0000-0000-0000-000000000009",
                r#"{"schemaVersion":2,"seed":9,"locale":"de_de","timezone":"europe_berlin","windowWidth":1440,"windowHeight":900,"webrtc":"native"}"#,
                root.path().join("legacy-v2-profile").to_string_lossy(),
            ),
        )
        .unwrap();
    drop(connection);

    let store = SqliteIdentityStore::open(&database).unwrap();
    let loaded = store.get(IdentityId::new(Uuid::from_u128(9))).unwrap();
    assert_eq!(loaded.persona.schema_version, CURRENT_SCHEMA_VERSION);
    assert_eq!(
        loaded.persona.timezone,
        TimezonePolicy::iana("Europe/Berlin").unwrap()
    );
}

#[test]
fn migrates_persisted_v3_display_placeholders_to_native_metrics() {
    let root = TempDir::new().unwrap();
    let database = root.path().join("state.sqlite3");
    let store = SqliteIdentityStore::open(&database).unwrap();
    drop(store);
    let connection = Connection::open(&database).unwrap();
    connection
        .execute(
            "INSERT INTO browser_identities
             (id, revision, name, startup_url, persona_json, lifecycle, profile_root, created_at_ms, updated_at_ms)
             VALUES (?1, 1, 'Legacy display', NULL, ?2, 'active', ?3, 10, 10)",
            (
                "00000000-0000-0000-0000-00000000000a",
                r#"{"schemaVersion":3,"seed":10,"locale":"system","timezone":"system","windowWidth":1440,"windowHeight":900,"webrtc":"native","surfaces":{"display":{"viewport":"native","screen":"native","devicePixelRatio":"native"}}}"#,
                root.path().join("legacy-v3-profile").to_string_lossy(),
            ),
        )
        .unwrap();
    drop(connection);

    let store = SqliteIdentityStore::open(&database).unwrap();
    let loaded = store.get(IdentityId::new(Uuid::from_u128(10))).unwrap();
    assert_eq!(loaded.persona.schema_version, CURRENT_SCHEMA_VERSION);
    assert_eq!(loaded.persona.display_metrics, None);
}

#[test]
fn persists_and_clears_runtime_reconciliation_record() {
    let root = TempDir::new().unwrap();
    let store = SqliteIdentityStore::open(&root.path().join("state.sqlite3")).unwrap();
    let identity = identity(&root);
    store.insert(&identity).unwrap();
    let runtime = BrowserRuntimeRecord {
        identity_id: identity.id,
        pid: 42,
        executable: "/Applications/RealBrowser.app/Contents/Resources/realbrowser-kernel/RealBrowser.app/Contents/MacOS/RealBrowser".into(),
        profile_root: identity.profile_root.clone(),
        browser_version: "151.0".to_owned(),
        started_at_ms: 20,
    };

    store.upsert(&runtime).unwrap();
    assert_eq!(store.list().unwrap(), vec![runtime]);
    store.remove(identity.id).unwrap();
    assert!(store.list().unwrap().is_empty());
}
