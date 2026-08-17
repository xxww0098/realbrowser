#![forbid(unsafe_code)]

use std::{
    collections::{HashSet, VecDeque},
    error::Error,
    fs,
    path::{Path, PathBuf},
    process::Command,
};

use cargo_metadata::MetadataCommand;
use toml_edit::DocumentMut;
use walkdir::WalkDir;

type Result<T> = std::result::Result<T, Box<dyn Error>>;

fn main() -> Result<()> {
    let command = std::env::args().nth(1).unwrap_or_else(|| "help".to_owned());
    match command.as_str() {
        "structure" => check_structure(),
        "ci" => {
            check_structure()?;
            run("cargo", &["fmt", "--all", "--", "--check"])?;
            run(
                "cargo",
                &[
                    "clippy",
                    "--workspace",
                    "--all-targets",
                    "--",
                    "-D",
                    "warnings",
                ],
            )?;
            run("cargo", &["test", "--workspace"])?;
            run("pnpm", &["--filter", "@realbrowser/desktop", "check"])?;
            run("pnpm", &["--filter", "@realbrowser/desktop", "test"])?;
            run("pnpm", &["--filter", "@realbrowser/desktop", "build"])
        }
        _ => {
            eprintln!("Usage: cargo xtask <structure|ci>");
            Ok(())
        }
    }
}

fn check_structure() -> Result<()> {
    let metadata = MetadataCommand::new().no_deps().exec()?;
    let workspace_root = metadata.workspace_root.as_std_path();
    let mut violations = Vec::new();
    check_workspace_manifest(&workspace_root.join("Cargo.toml"), &mut violations)?;
    check_crate_direction(&metadata, &mut violations);

    for package in metadata.workspace_packages() {
        let manifest = package.manifest_path.as_std_path();
        check_manifest(manifest, &mut violations)?;
        let package_root = manifest.parent().ok_or("package has no parent")?;
        check_test_shape(package_root, &mut violations);
        check_orphan_modules(package_root, &mut violations)?;
    }

    if violations.is_empty() {
        println!(
            "structure: ok ({} workspace members)",
            metadata.workspace_members.len()
        );
        return Ok(());
    }

    violations.sort();
    eprintln!("structure: {} violation(s)", violations.len());
    for violation in violations {
        eprintln!("- {violation}");
    }
    Err(format!("structure checks failed under {}", workspace_root.display()).into())
}

fn check_workspace_manifest(path: &Path, violations: &mut Vec<String>) -> Result<()> {
    let source = fs::read_to_string(path)?;
    let document = source.parse::<DocumentMut>()?;
    if document["workspace"]["resolver"].as_str() != Some("3") {
        violations.push("workspace resolver must be 3".to_owned());
    }
    if document["workspace"]["package"]["edition"].as_str() != Some("2024") {
        violations.push("workspace edition must be 2024".to_owned());
    }
    if document["workspace"]["package"]["rust-version"].as_str() != Some("1.97") {
        violations.push("workspace rust-version must be 1.97".to_owned());
    }
    let dev_profile = document
        .get("profile")
        .and_then(|profile| profile.get("dev"));
    let has_build_override = dev_profile
        .and_then(|profile| profile.get("build-override"))
        .is_some();
    let package_overrides = dev_profile
        .and_then(|profile| profile.get("package"))
        .and_then(toml_edit::Item::as_table);
    let has_package_override = package_overrides.is_some_and(|table| !table.is_empty());
    if has_build_override != has_package_override {
        violations.push(
            "profile.dev build-override and named package overrides must be configured together"
                .to_owned(),
        );
    }
    if package_overrides.is_some_and(|table| table.contains_key("*")) {
        violations.push("profile.dev.package must not use the '*' wildcard".to_owned());
    }
    let serde_features = document
        .get("workspace")
        .and_then(|workspace| workspace.get("dependencies"))
        .and_then(|dependencies| dependencies.get("serde"))
        .and_then(|serde| serde.get("features"))
        .and_then(toml_edit::Item::as_array)
        .map(|features| {
            features
                .iter()
                .filter_map(toml_edit::Value::as_str)
                .collect::<Vec<_>>()
        })
        .unwrap_or_default();
    if serde_features.contains(&"derive") {
        violations.push("workspace serde dependency must not enable derive globally".to_owned());
    }
    Ok(())
}

fn check_crate_direction(metadata: &cargo_metadata::Metadata, violations: &mut Vec<String>) {
    let internal = metadata
        .workspace_packages()
        .into_iter()
        .map(|package| package.name.as_str())
        .collect::<HashSet<_>>();
    for package in metadata.workspace_packages() {
        for dependency in &package.dependencies {
            let dependency_name = dependency.name.as_str();
            if internal.contains(dependency_name)
                && !allowed_internal_dependency(package.name.as_str(), dependency_name)
            {
                violations.push(format!(
                    "crate direction forbids {} -> {}",
                    package.name, dependency.name
                ));
            }
        }
    }
}

fn allowed_internal_dependency(package: &str, dependency: &str) -> bool {
    matches!(
        (package, dependency),
        ("browser-core", "browser-persona" | "browser-network")
            | (
                "browser-control",
                "browser-core"
                    | "browser-network"
                    | "browser-persona"
                    | "browser-persona-runtime"
                    | "browser-profile"
            )
            | ("browser-persona-runtime", "browser-persona")
            | (
                "browser-storage",
                "browser-core" | "browser-network" | "browser-persona"
            )
            | ("browser-platform", "browser-core" | "browser-profile")
            | (
                "realbrowser-desktop",
                "browser-control"
                    | "browser-core"
                    | "browser-network"
                    | "browser-platform"
                    | "browser-persona"
                    | "browser-persona-runtime"
                    | "browser-storage"
            )
    )
}

fn check_manifest(path: &Path, violations: &mut Vec<String>) -> Result<()> {
    let source = fs::read_to_string(path)?;
    let document = source.parse::<DocumentMut>()?;
    let package_name = document["package"]["name"]
        .as_str()
        .unwrap_or("workspace-root");
    if package_name != "workspace-root" && document["lints"]["workspace"].as_bool() != Some(true) {
        violations.push(format!(
            "{} does not enable [lints] workspace = true",
            path.display()
        ));
    }

    for table_name in ["dependencies", "dev-dependencies", "build-dependencies"] {
        let Some(table) = document.get(table_name).and_then(toml_edit::Item::as_table) else {
            continue;
        };
        for (name, dependency) in table {
            let has_path = dependency.get("path").is_some();
            let is_workspace = dependency
                .get("workspace")
                .and_then(toml_edit::Item::as_bool)
                == Some(true);
            if has_path && !is_workspace {
                violations.push(format!(
                    "{} dependency {name} uses a raw path; inherit it from [workspace.dependencies]",
                    path.display()
                ));
            }
        }
    }

    if package_name != "xtask"
        && document.get("lib").is_some()
        && document["lib"]["doctest"].as_bool() != Some(false)
    {
        violations.push(format!("{} must set [lib] doctest = false", path.display()));
    }
    Ok(())
}

fn check_test_shape(package_root: &Path, violations: &mut Vec<String>) {
    if package_root.join("tests/common.rs").exists() {
        violations.push(format!(
            "{} must use tests/common/mod.rs instead of tests/common.rs",
            package_root.display()
        ));
    }
    if package_root.join("src/tests").is_dir() {
        violations.push(format!(
            "{} contains src/tests/, which rustc silently ignores without an explicit module",
            package_root.display()
        ));
    }
}

fn check_orphan_modules(package_root: &Path, violations: &mut Vec<String>) -> Result<()> {
    let src = package_root.join("src");
    if !src.is_dir() {
        return Ok(());
    }

    let all_files = WalkDir::new(&src)
        .into_iter()
        .filter_map(std::result::Result::ok)
        .filter(|entry| entry.file_type().is_file())
        .map(|entry| entry.into_path())
        .filter(|path| path.extension().is_some_and(|extension| extension == "rs"))
        .collect::<HashSet<_>>();
    let mut reached = HashSet::new();
    let mut pending = VecDeque::new();
    for root in [src.join("lib.rs"), src.join("main.rs")] {
        if root.is_file() {
            pending.push_back(root);
        }
    }
    let bin = src.join("bin");
    if bin.is_dir() {
        for entry in WalkDir::new(&bin).max_depth(1).into_iter().flatten() {
            if entry
                .path()
                .extension()
                .is_some_and(|extension| extension == "rs")
            {
                pending.push_back(entry.into_path());
            }
        }
    }

    while let Some(file) = pending.pop_front() {
        if !reached.insert(file.clone()) {
            continue;
        }
        let source = fs::read_to_string(&file)?;
        for module in declared_modules(&source) {
            if let Some(candidate) = module_path(&file, &module)
                && candidate.is_file()
            {
                pending.push_back(candidate);
            }
        }
    }

    for orphan in all_files.difference(&reached) {
        violations.push(format!("unreachable Rust module: {}", orphan.display()));
    }
    Ok(())
}

fn declared_modules(source: &str) -> Vec<String> {
    source
        .lines()
        .map(str::trim)
        .filter_map(|line| {
            let line = line.strip_prefix("pub ").unwrap_or(line);
            let line = line.strip_prefix("pub(crate) ").unwrap_or(line);
            line.strip_prefix("mod ")
                .and_then(|value| value.strip_suffix(';'))
                .map(str::trim)
                .filter(|value| {
                    value
                        .chars()
                        .all(|character| character == '_' || character.is_alphanumeric())
                })
                .map(str::to_owned)
        })
        .collect()
}

fn module_path(parent: &Path, module: &str) -> Option<PathBuf> {
    let directory = parent.parent()?;
    let base = if parent.file_name().is_some_and(|name| name == "mod.rs") {
        directory.to_owned()
    } else {
        directory.join(parent.file_stem()?)
    };
    let sibling = directory.join(format!("{module}.rs"));
    let nested = base.join(format!("{module}.rs"));
    let mod_file = base.join(module).join("mod.rs");
    [sibling, nested, mod_file]
        .into_iter()
        .find(|candidate| candidate.is_file())
}

fn run(program: &str, arguments: &[&str]) -> Result<()> {
    let status = Command::new(program).args(arguments).status()?;
    if status.success() {
        Ok(())
    } else {
        Err(format!("{program} {} failed", arguments.join(" ")).into())
    }
}
