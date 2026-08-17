#![forbid(unsafe_code)]

use std::{path::Path, sync::Arc};

use browser_core::{
    BrowserIdentity, BrowserRuntimeRecord, IdentityId, IdentityLifecycle, IdentityStore,
    RuntimeJournal, StoreError,
};
use browser_network::NetworkConfig;
use browser_persona::PersonaConfig;
use parking_lot::Mutex;
use rusqlite::{Connection, OptionalExtension, params};

pub struct SqliteIdentityStore {
    connection: Mutex<Connection>,
}

impl SqliteIdentityStore {
    pub fn open(path: &Path) -> Result<Self, StoreError> {
        let connection = Connection::open(path).map_err(backend)?;
        connection
            .execute_batch(
                r#"PRAGMA journal_mode = WAL;
                 PRAGMA foreign_keys = ON;
                 CREATE TABLE IF NOT EXISTS browser_identities (
                   id TEXT PRIMARY KEY NOT NULL,
                   revision INTEGER NOT NULL,
                   name TEXT NOT NULL,
                   startup_url TEXT,
                   persona_json TEXT NOT NULL DEFAULT '{}',
                   network_json TEXT NOT NULL DEFAULT '{"schemaVersion":1,"mode":"direct","proxy":null}',
                   lifecycle TEXT NOT NULL CHECK (lifecycle IN ('active', 'archived')),
                   profile_root TEXT NOT NULL UNIQUE,
                   created_at_ms INTEGER NOT NULL,
                   updated_at_ms INTEGER NOT NULL
                 );
                 CREATE INDEX IF NOT EXISTS idx_browser_identities_active_updated
                   ON browser_identities(lifecycle, updated_at_ms DESC);
                 CREATE TABLE IF NOT EXISTS browser_runtime_journal (
                   identity_id TEXT PRIMARY KEY NOT NULL,
                   pid INTEGER NOT NULL,
                   executable TEXT NOT NULL,
                   profile_root TEXT NOT NULL,
                   browser_version TEXT NOT NULL,
                   started_at_ms INTEGER NOT NULL,
                   FOREIGN KEY(identity_id) REFERENCES browser_identities(id) ON DELETE CASCADE
                 );"#,
            )
            .map_err(backend)?;
        ensure_persona_column(&connection)?;
        ensure_network_column(&connection)?;
        Ok(Self {
            connection: Mutex::new(connection),
        })
    }
}

impl RuntimeJournal for SqliteIdentityStore {
    fn upsert(&self, runtime: &BrowserRuntimeRecord) -> Result<(), StoreError> {
        let pid = i64::from(runtime.pid);
        self.connection
            .lock()
            .execute(
                "INSERT INTO browser_runtime_journal
                 (identity_id, pid, executable, profile_root, browser_version, started_at_ms)
                 VALUES (?1, ?2, ?3, ?4, ?5, ?6)
                 ON CONFLICT(identity_id) DO UPDATE SET
                   pid = excluded.pid,
                   executable = excluded.executable,
                   profile_root = excluded.profile_root,
                   browser_version = excluded.browser_version,
                   started_at_ms = excluded.started_at_ms",
                params![
                    runtime.identity_id.to_string(),
                    pid,
                    runtime.executable.to_string_lossy(),
                    runtime.profile_root.to_string_lossy(),
                    runtime.browser_version,
                    runtime.started_at_ms,
                ],
            )
            .map_err(backend)?;
        Ok(())
    }

    fn remove(&self, identity_id: IdentityId) -> Result<(), StoreError> {
        self.connection
            .lock()
            .execute(
                "DELETE FROM browser_runtime_journal WHERE identity_id = ?1",
                [identity_id.to_string()],
            )
            .map_err(backend)?;
        Ok(())
    }

    fn list(&self) -> Result<Vec<BrowserRuntimeRecord>, StoreError> {
        let connection = self.connection.lock();
        let mut statement = connection
            .prepare(
                "SELECT identity_id, pid, executable, profile_root, browser_version, started_at_ms
                 FROM browser_runtime_journal ORDER BY started_at_ms ASC",
            )
            .map_err(backend)?;
        statement
            .query_map([], map_runtime)
            .map_err(backend)?
            .collect::<Result<Vec<_>, _>>()
            .map_err(backend)
    }
}

impl IdentityStore for SqliteIdentityStore {
    fn insert(&self, identity: &BrowserIdentity) -> Result<(), StoreError> {
        let revision = sqlite_revision(identity.revision)?;
        let persona_json = serde_json::to_string(&identity.persona)
            .map_err(|error| StoreError::Backend(Arc::from(error.to_string())))?;
        let network_json = serde_json::to_string(&identity.network)
            .map_err(|error| StoreError::Backend(Arc::from(error.to_string())))?;
        self.connection
            .lock()
            .execute(
                "INSERT INTO browser_identities
                 (id, revision, name, startup_url, persona_json, network_json, lifecycle, profile_root, created_at_ms, updated_at_ms)
                 VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10)",
                params![
                    identity.id.to_string(),
                    revision,
                    identity.name,
                    identity.startup_url,
                    persona_json,
                    network_json,
                    lifecycle_name(identity.lifecycle),
                    identity.profile_root.to_string_lossy(),
                    identity.created_at_ms,
                    identity.updated_at_ms,
                ],
            )
            .map_err(backend)?;
        Ok(())
    }

    fn get(&self, id: IdentityId) -> Result<BrowserIdentity, StoreError> {
        self.connection
            .lock()
            .query_row(
                "SELECT id, revision, name, startup_url, persona_json, network_json, lifecycle, profile_root, created_at_ms, updated_at_ms
                 FROM browser_identities WHERE id = ?1",
                [id.to_string()],
                map_identity,
            )
            .optional()
            .map_err(backend)?
            .ok_or(StoreError::NotFound)
    }

    fn list_by_lifecycle(
        &self,
        lifecycle: IdentityLifecycle,
    ) -> Result<Vec<BrowserIdentity>, StoreError> {
        let connection = self.connection.lock();
        let mut statement = connection
            .prepare(
                "SELECT id, revision, name, startup_url, persona_json, network_json, lifecycle, profile_root, created_at_ms, updated_at_ms
                 FROM browser_identities
                 WHERE lifecycle = ?1
                 ORDER BY updated_at_ms DESC, created_at_ms DESC",
            )
            .map_err(backend)?;
        statement
            .query_map([lifecycle_name(lifecycle)], map_identity)
            .map_err(backend)?
            .collect::<Result<Vec<_>, _>>()
            .map_err(backend)
    }

    fn replace(
        &self,
        identity: &BrowserIdentity,
        expected_revision: u64,
    ) -> Result<(), StoreError> {
        let revision = sqlite_revision(identity.revision)?;
        let expected_revision = sqlite_revision(expected_revision)?;
        let persona_json = serde_json::to_string(&identity.persona)
            .map_err(|error| StoreError::Backend(Arc::from(error.to_string())))?;
        let network_json = serde_json::to_string(&identity.network)
            .map_err(|error| StoreError::Backend(Arc::from(error.to_string())))?;
        let changed = self
            .connection
            .lock()
            .execute(
                "UPDATE browser_identities
                 SET revision = ?1, name = ?2, startup_url = ?3, persona_json = ?4, network_json = ?5,
                     lifecycle = ?6, profile_root = ?7, updated_at_ms = ?8
                 WHERE id = ?9 AND revision = ?10",
                params![
                    revision,
                    identity.name,
                    identity.startup_url,
                    persona_json,
                    network_json,
                    lifecycle_name(identity.lifecycle),
                    identity.profile_root.to_string_lossy(),
                    identity.updated_at_ms,
                    identity.id.to_string(),
                    expected_revision,
                ],
            )
            .map_err(backend)?;
        if changed == 0 {
            return Err(StoreError::RevisionConflict);
        }
        Ok(())
    }
}

fn map_identity(row: &rusqlite::Row<'_>) -> rusqlite::Result<BrowserIdentity> {
    let raw_id: String = row.get(0)?;
    let id = IdentityId::parse(&raw_id).map_err(|error| {
        rusqlite::Error::FromSqlConversionFailure(0, rusqlite::types::Type::Text, Box::new(error))
    })?;
    let raw_persona: String = row.get(4)?;
    let persona = serde_json::from_str::<PersonaConfig>(&raw_persona).map_err(|error| {
        rusqlite::Error::FromSqlConversionFailure(4, rusqlite::types::Type::Text, Box::new(error))
    })?;
    let persona = persona.migrate().map_err(|error| {
        rusqlite::Error::FromSqlConversionFailure(4, rusqlite::types::Type::Text, Box::new(error))
    })?;
    let raw_network: String = row.get(5)?;
    let network = serde_json::from_str::<NetworkConfig>(&raw_network).map_err(|error| {
        rusqlite::Error::FromSqlConversionFailure(5, rusqlite::types::Type::Text, Box::new(error))
    })?;
    let network = network.normalized().map_err(|error| {
        rusqlite::Error::FromSqlConversionFailure(5, rusqlite::types::Type::Text, Box::new(error))
    })?;
    let lifecycle: String = row.get(6)?;
    let lifecycle = match lifecycle.as_str() {
        "active" => IdentityLifecycle::Active,
        "archived" => IdentityLifecycle::Archived,
        _ => {
            return Err(rusqlite::Error::InvalidColumnType(
                6,
                "lifecycle".to_owned(),
                rusqlite::types::Type::Text,
            ));
        }
    };
    let raw_revision: i64 = row.get(1)?;
    let revision = u64::try_from(raw_revision).map_err(|error| {
        rusqlite::Error::FromSqlConversionFailure(
            1,
            rusqlite::types::Type::Integer,
            Box::new(error),
        )
    })?;
    Ok(BrowserIdentity {
        id,
        revision,
        name: row.get(2)?,
        startup_url: row.get(3)?,
        persona,
        network,
        lifecycle,
        profile_root: row.get::<_, String>(7)?.into(),
        created_at_ms: row.get(8)?,
        updated_at_ms: row.get(9)?,
    })
}

fn map_runtime(row: &rusqlite::Row<'_>) -> rusqlite::Result<BrowserRuntimeRecord> {
    let raw_id: String = row.get(0)?;
    let identity_id = IdentityId::parse(&raw_id).map_err(|error| {
        rusqlite::Error::FromSqlConversionFailure(0, rusqlite::types::Type::Text, Box::new(error))
    })?;
    let raw_pid: i64 = row.get(1)?;
    let pid = u32::try_from(raw_pid).map_err(|error| {
        rusqlite::Error::FromSqlConversionFailure(
            1,
            rusqlite::types::Type::Integer,
            Box::new(error),
        )
    })?;
    Ok(BrowserRuntimeRecord {
        identity_id,
        pid,
        executable: row.get::<_, String>(2)?.into(),
        profile_root: row.get::<_, String>(3)?.into(),
        browser_version: row.get(4)?,
        started_at_ms: row.get(5)?,
    })
}

fn ensure_persona_column(connection: &Connection) -> Result<(), StoreError> {
    let mut statement = connection
        .prepare("PRAGMA table_info(browser_identities)")
        .map_err(backend)?;
    let columns = statement
        .query_map([], |row| row.get::<_, String>(1))
        .map_err(backend)?
        .collect::<Result<Vec<_>, _>>()
        .map_err(backend)?;
    drop(statement);
    if !columns.iter().any(|column| column == "persona_json") {
        connection
            .execute(
                "ALTER TABLE browser_identities ADD COLUMN persona_json TEXT NOT NULL DEFAULT '{}'",
                [],
            )
            .map_err(backend)?;
    }
    Ok(())
}

fn ensure_network_column(connection: &Connection) -> Result<(), StoreError> {
    let mut statement = connection
        .prepare("PRAGMA table_info(browser_identities)")
        .map_err(backend)?;
    let columns = statement
        .query_map([], |row| row.get::<_, String>(1))
        .map_err(backend)?
        .collect::<Result<Vec<_>, _>>()
        .map_err(backend)?;
    drop(statement);
    if !columns.iter().any(|column| column == "network_json") {
        connection
            .execute(
                "ALTER TABLE browser_identities ADD COLUMN network_json TEXT NOT NULL DEFAULT '{\"schemaVersion\":1,\"mode\":\"direct\",\"proxy\":null}'",
                [],
            )
            .map_err(backend)?;
    }
    Ok(())
}

fn lifecycle_name(lifecycle: IdentityLifecycle) -> &'static str {
    match lifecycle {
        IdentityLifecycle::Active => "active",
        IdentityLifecycle::Archived => "archived",
    }
}

fn backend(error: rusqlite::Error) -> StoreError {
    StoreError::Backend(Arc::from(error.to_string()))
}

fn sqlite_revision(revision: u64) -> Result<i64, StoreError> {
    i64::try_from(revision)
        .map_err(|_| StoreError::Backend(Arc::from("revision exceeds SQLite INTEGER range")))
}

#[cfg(test)]
mod tests;
