#![forbid(unsafe_code)]

use std::{
    fs::{File, OpenOptions},
    io::{Seek, SeekFrom, Write},
    path::{Path, PathBuf},
};

use fs4::{FileExt, TryLockError};
use thiserror::Error;

const LEASE_DIRECTORY: &str = ".realbrowser";
const LEASE_FILE: &str = "profile.lease";

/// An exclusive application lease for one complete RealBrowser User Data root.
/// The OS releases the underlying lock if the owning process exits.
pub struct ProfileLease {
    root: PathBuf,
    file: File,
}

impl ProfileLease {
    pub fn acquire(
        data_root: &Path,
        profile_root: &Path,
        expected_identity: &str,
    ) -> Result<Self, ProfileLeaseError> {
        let expected_root = prepare_profile_root(data_root, expected_identity)?;

        if profile_root != expected_root {
            return Err(ProfileLeaseError::UnexpectedRoot {
                expected: expected_root,
                actual: profile_root.to_path_buf(),
            });
        }

        let canonical_profile_root = canonical_directory(profile_root, "profile root")?;
        if canonical_profile_root != profile_root {
            return Err(ProfileLeaseError::EscapesDataRoot(canonical_profile_root));
        }

        let lease_directory = canonical_profile_root.join(LEASE_DIRECTORY);
        create_plain_directory(&lease_directory, "lease directory")?;
        let canonical_lease_directory = canonical_directory(&lease_directory, "lease directory")?;
        if canonical_lease_directory != lease_directory {
            return Err(ProfileLeaseError::UnsafePath(lease_directory));
        }

        let lease_path = canonical_lease_directory.join(LEASE_FILE);
        reject_symlink_if_present(&lease_path, "lease file")?;
        let mut file = OpenOptions::new()
            .create(true)
            .read(true)
            .write(true)
            .truncate(false)
            .open(&lease_path)
            .map_err(|source| ProfileLeaseError::Io {
                operation: "open lease file",
                source,
            })?;
        match FileExt::try_lock(&file) {
            Ok(()) => {}
            Err(TryLockError::WouldBlock) => return Err(ProfileLeaseError::AlreadyOwned),
            Err(TryLockError::Error(source)) => {
                return Err(ProfileLeaseError::Io {
                    operation: "lock lease file",
                    source,
                });
            }
        }

        write_owner_marker(&mut file)?;
        Ok(Self {
            root: canonical_profile_root,
            file,
        })
    }

    #[must_use]
    pub fn root(&self) -> &Path {
        &self.root
    }
}

/// Creates and validates the only Profile root allowed for an identity.
/// Existing symlinks/reparse points are rejected before any lease file is opened.
pub fn prepare_profile_root(
    data_root: &Path,
    expected_identity: &str,
) -> Result<PathBuf, ProfileLeaseError> {
    let canonical_data_root = canonical_directory(data_root, "data root")?;
    let profiles_root = canonical_data_root.join("profiles");
    create_plain_directory(&profiles_root, "profiles root")?;
    let canonical_profiles_root = canonical_directory(&profiles_root, "profiles root")?;
    let profile_root = canonical_profiles_root.join(expected_identity);
    create_plain_directory(&profile_root, "profile root")?;
    let canonical_profile_root = canonical_directory(&profile_root, "profile root")?;
    if canonical_profile_root != profile_root
        || !canonical_profile_root.starts_with(&canonical_profiles_root)
    {
        return Err(ProfileLeaseError::EscapesDataRoot(canonical_profile_root));
    }
    Ok(canonical_profile_root)
}

impl std::fmt::Debug for ProfileLease {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        formatter
            .debug_struct("ProfileLease")
            .field("root", &self.root)
            .finish_non_exhaustive()
    }
}

impl Drop for ProfileLease {
    fn drop(&mut self) {
        let _ = FileExt::unlock(&self.file);
    }
}

#[derive(Debug, Error)]
pub enum ProfileLeaseError {
    #[error("profile is already owned by another RealBrowser process")]
    AlreadyOwned,
    #[error("profile root does not match its identity: expected {expected:?}, got {actual:?}")]
    UnexpectedRoot { expected: PathBuf, actual: PathBuf },
    #[error("profile path escapes the application data root: {0:?}")]
    EscapesDataRoot(PathBuf),
    #[error("profile path contains a symbolic link or reparse point: {0:?}")]
    UnsafePath(PathBuf),
    #[error("cannot {operation}: {source}")]
    Io {
        operation: &'static str,
        #[source]
        source: std::io::Error,
    },
}

fn create_plain_directory(path: &Path, label: &'static str) -> Result<(), ProfileLeaseError> {
    reject_symlink_if_present(path, label)?;
    std::fs::create_dir_all(path).map_err(|source| ProfileLeaseError::Io {
        operation: label,
        source,
    })?;
    reject_symlink_if_present(path, label)
}

fn canonical_directory(path: &Path, label: &'static str) -> Result<PathBuf, ProfileLeaseError> {
    path.canonicalize().map_err(|source| ProfileLeaseError::Io {
        operation: label,
        source,
    })
}

fn reject_symlink_if_present(path: &Path, label: &'static str) -> Result<(), ProfileLeaseError> {
    match std::fs::symlink_metadata(path) {
        Ok(metadata) if metadata.file_type().is_symlink() => {
            Err(ProfileLeaseError::UnsafePath(path.to_path_buf()))
        }
        Ok(_) => Ok(()),
        Err(source) if source.kind() == std::io::ErrorKind::NotFound => Ok(()),
        Err(source) => Err(ProfileLeaseError::Io {
            operation: label,
            source,
        }),
    }
}

fn write_owner_marker(file: &mut File) -> Result<(), ProfileLeaseError> {
    file.set_len(0).map_err(|source| ProfileLeaseError::Io {
        operation: "truncate lease owner marker",
        source,
    })?;
    file.seek(SeekFrom::Start(0))
        .and_then(|_| writeln!(file, "owner_pid={}", std::process::id()))
        .and_then(|_| file.sync_data())
        .map_err(|source| ProfileLeaseError::Io {
            operation: "write lease owner marker",
            source,
        })
}

#[cfg(test)]
mod tests {
    use super::{ProfileLease, ProfileLeaseError, prepare_profile_root};
    use tempfile::TempDir;

    #[test]
    fn one_profile_has_one_application_owner() {
        let root = TempDir::new().unwrap();
        let data_root = root.path().canonicalize().unwrap();
        let profile_root = data_root.join("profiles").join("identity-a");
        let first = ProfileLease::acquire(&data_root, &profile_root, "identity-a").unwrap();

        assert!(matches!(
            ProfileLease::acquire(&data_root, &profile_root, "identity-a"),
            Err(ProfileLeaseError::AlreadyOwned)
        ));

        drop(first);
        ProfileLease::acquire(&data_root, &profile_root, "identity-a").unwrap();
    }

    #[test]
    fn identity_cannot_claim_another_profile_root() {
        let root = TempDir::new().unwrap();
        let data_root = root.path().canonicalize().unwrap();
        let profile_root = data_root.join("profiles").join("identity-b");

        assert!(matches!(
            ProfileLease::acquire(&data_root, &profile_root, "identity-a"),
            Err(ProfileLeaseError::UnexpectedRoot { .. })
        ));
    }

    #[test]
    fn preparation_derives_one_canonical_profile_root() {
        let root = TempDir::new().unwrap();
        let data_root = root.path().canonicalize().unwrap();

        let profile_root = prepare_profile_root(&data_root, "identity-a").unwrap();

        assert_eq!(profile_root, data_root.join("profiles").join("identity-a"));
        assert!(profile_root.is_dir());
    }

    #[cfg(unix)]
    #[test]
    fn symlinked_profile_cannot_escape_the_data_root() {
        use std::os::unix::fs::symlink;

        let root = TempDir::new().unwrap();
        let outside = TempDir::new().unwrap();
        let data_root = root.path().canonicalize().unwrap();
        let profiles_root = data_root.join("profiles");
        std::fs::create_dir_all(&profiles_root).unwrap();
        let profile_root = profiles_root.join("identity-a");
        symlink(outside.path(), &profile_root).unwrap();

        assert!(matches!(
            prepare_profile_root(&data_root, "identity-a"),
            Err(ProfileLeaseError::UnsafePath(path)) if path == profile_root
        ));
    }
}
