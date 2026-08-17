use std::sync::Arc;

use thiserror::Error;

use crate::{
    BrowserIdentity, BrowserInstall, BrowserRuntimeRecord, BrowserSession, IdentityId, LaunchPlan,
};

#[derive(Clone, Debug, Error, Eq, PartialEq)]
pub enum StoreError {
    #[error("record not found")]
    NotFound,
    #[error("revision conflict")]
    RevisionConflict,
    #[error("storage failure: {0}")]
    Backend(Arc<str>),
}

pub trait IdentityStore: Send + Sync {
    fn insert(&self, identity: &BrowserIdentity) -> Result<(), StoreError>;
    fn get(&self, id: IdentityId) -> Result<BrowserIdentity, StoreError>;
    fn list_by_lifecycle(
        &self,
        lifecycle: crate::IdentityLifecycle,
    ) -> Result<Vec<BrowserIdentity>, StoreError>;
    fn replace(&self, identity: &BrowserIdentity, expected_revision: u64)
    -> Result<(), StoreError>;
}

pub trait BrowserHost: Send + Sync {
    fn detect_product_kernel(&self) -> Result<BrowserInstall, String>;
    fn start(&self, plan: &LaunchPlan, now_ms: i64) -> Result<BrowserSession, String>;
    fn reconcile(&self, runtime: &BrowserRuntimeRecord) -> Result<Option<BrowserSession>, String>;
    fn stop(&self, identity_id: IdentityId) -> Result<crate::TerminationKind, String>;
    fn is_running(&self, identity_id: IdentityId) -> bool;
}

pub trait RuntimeJournal: Send + Sync {
    fn upsert(&self, runtime: &BrowserRuntimeRecord) -> Result<(), StoreError>;
    fn remove(&self, identity_id: IdentityId) -> Result<(), StoreError>;
    fn list(&self) -> Result<Vec<BrowserRuntimeRecord>, StoreError>;
}

pub trait Clock: Send + Sync {
    fn now_ms(&self) -> i64;
}

pub trait IdGenerator: Send + Sync {
    fn next_identity_id(&self) -> IdentityId;
}
