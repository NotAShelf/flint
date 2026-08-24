use crate::lock::{FlakeLock, Locked, Node};
use serde::Deserialize;
use std::thread;

#[derive(Debug, Default, Clone, serde::Serialize)]
pub struct UpdateStatus {
    #[serde(rename = "InputName")]
    pub input_name: String,
    #[serde(rename = "CurrentRev")]
    pub current_rev: String,
    #[serde(rename = "LatestRev")]
    pub latest_rev: String,
    #[serde(rename = "CurrentURL")]
    pub current_url: String,
    #[serde(rename = "LatestURL")]
    pub latest_url: String,
    #[serde(rename = "IsUpdate")]
    pub is_update: bool,
    #[serde(rename = "Error")]
    pub error: String,
}

#[derive(Debug, Default, serde::Serialize)]
pub struct UpdateResults {
    #[serde(rename = "Updates")]
    pub updates: Vec<UpdateStatus>,
}

/// Check every root input for updates, concurrently.
pub fn check_updates(lock: &FlakeLock, verbose: bool) -> Result<UpdateResults, String> {
    let root_inputs = match lock.nodes.get("root").and_then(|n| n.inputs.as_ref()) {
        Some(inputs) => inputs,
        None => return Err("no root inputs found".to_string()),
    };

    if verbose {
        println!("Root inputs: {root_inputs:?}");
    }

    // BTreeMap iteration is name-sorted, so spawning and joining in this order
    // keeps the results ✨ deterministic ✨ .
    let updates = thread::scope(|scope| {
        let handles: Vec<_> = root_inputs
            .iter()
            .map(|(name, target)| {
                use crate::lock::InputRef;
                scope.spawn(move || match target {
                    InputRef::One(reference) => check_input_update(lock, name, reference, verbose),
                    InputRef::Follows(_) => UpdateStatus {
                        input_name: name.clone(),
                        error: "invalid input reference type".to_string(),
                        ..Default::default()
                    },
                })
            })
            .collect();

        handles.into_iter().map(|h| h.join().unwrap()).collect()
    });

    Ok(UpdateResults { updates })
}

/// Check the input resolved by `input_ref` (a node name) for updates.
pub fn check_input_update(
    lock: &FlakeLock,
    input_name: &str,
    input_ref: &str,
    verbose: bool,
) -> UpdateStatus {
    let mut update = UpdateStatus {
        input_name: input_name.to_string(),
        ..Default::default()
    };

    let Some(node) = lock.nodes.get(input_ref) else {
        update.error = format!("input {input_name} has no locked version");
        return update;
    };
    let Some(locked) = &node.locked else {
        update.error = format!("input {input_name} is not a flake (no locked version)");
        return update;
    };

    update.current_rev = locked.rev.clone();
    update.current_url = build_flake_url(Some(locked));

    // Inputs pinned to a specific commit (an evergreen ref) can never update.
    if let Some(original) = &node.original
        && is_commit_hash(&original.ref_)
    {
        if verbose {
            println!("Skipping {input_name}: pinned to specific commit");
        }
        update.latest_rev = locked.rev.clone();
        update.latest_url = update.current_url.clone();
        return update;
    }

    match latest_revision(node, locked, verbose) {
        Ok(rev) => {
            update.latest_url = update.current_url.clone();
            update.is_update = !rev.is_empty() && rev != update.current_rev;
            update.latest_rev = rev;
        }
        Err(err) => update.error = format!("failed to get latest revision: {err}"),
    }

    update
}

/// Build a flake URL from a locked source.
/// Default hosts are omitted.
pub fn build_flake_url(locked: Option<&Locked>) -> String {
    let Some(locked) = locked else {
        return String::new();
    };

    match locked.kind.as_str() {
        "github" | "gitlab" | "sourcehut" => {
            let mut url = format!("{}:{}/{}", locked.kind, locked.owner, locked.repo);
            if !locked.host.is_empty() && locked.host != "github.com" && locked.host != "gitlab.com"
            {
                url.push_str(&format!("?host={}", locked.host));
            }
            url
        }
        "git" | "tarball" => locked.url.clone(),
        "path" => locked.path.clone(),
        _ => String::new(),
    }
}

/// A 40-character lowercase-hex git commit hash.
fn is_commit_hash(s: &str) -> bool {
    s.len() == 40 && s.bytes().all(|b| matches!(b, b'0'..=b'9' | b'a'..=b'f'))
}

/// Resolve the git URL and ref to check for a node, then fetch its latest rev.
fn latest_revision(node: &Node, locked: &Locked, verbose: bool) -> Result<String, String> {
    let (git_url, git_ref) = match &node.original {
        Some(original) => match original.kind.as_str() {
            "github" | "gitlab" | "sourcehut" => (
                host_git_url(&original.kind, locked, &locked.owner, &original.repo),
                original.ref_.clone(),
            ),
            "git" => {
                if locked.url.starts_with("ssh://") {
                    return Err("git+ssh URLs not supported".to_string());
                }
                (locked.url.clone(), original.ref_.clone())
            }
            "tarball" => return latest_from_tarball(locked, verbose),
            other => return Err(format!("unsupported input type: {other}")),
        },
        None => match locked.kind.as_str() {
            "github" | "gitlab" | "sourcehut" => (
                host_git_url(&locked.kind, locked, &locked.owner, &locked.repo),
                String::new(),
            ),
            "git" => (locked.url.clone(), String::new()),
            other => return Err(format!("unsupported locked type: {other}")),
        },
    };

    let git_ref = normalize_ref(git_ref);
    if verbose {
        println!("Checking {git_url} for updates (ref: {git_ref})");
    }
    latest_commit(&git_url, &git_ref, verbose)
}

/// Reduce a fully-qualified ref to its short name so both `nixos-unstable` and
/// `refs/heads/nixos-unstable` resolve the same way.
fn normalize_ref(git_ref: String) -> String {
    for prefix in ["refs/heads/", "refs/tags/"] {
        if let Some(short) = git_ref.strip_prefix(prefix) {
            return short.to_string();
        }
    }
    git_ref
}

/// Build the clone URL for a forge, honoring a custom host.
fn host_git_url(kind: &str, locked: &Locked, owner: &str, repo: &str) -> String {
    let host = if !locked.host.is_empty() {
        locked.host.as_str()
    } else if kind == "sourcehut" {
        "git.sr.ht"
    } else {
        return format!("https://{kind}.com/{owner}/{repo}.git");
    };
    if kind == "sourcehut" {
        format!("https://{host}/~{owner}/{repo}.git")
    } else {
        format!("https://{host}/{owner}/{repo}.git")
    }
}

/// Reconstruct the repository from a tarball URL and check its ref.
fn latest_from_tarball(locked: &Locked, verbose: bool) -> Result<String, String> {
    let (repo_url, git_ref) = parse_tarball(&locked.url)
        .ok_or_else(|| format!("cannot parse tarball URL: {}", locked.url))?;

    if is_commit_hash(&git_ref) {
        return Err("tarball points to specific commit, skipping".to_string());
    }
    if verbose {
        println!("Reconstructed git URL from tarball: {repo_url} (ref: {git_ref})");
    }
    latest_commit(&repo_url, &git_ref, verbose)
}

/// Split a forge tarball URL into (repo clone URL, ref). Handles the common
/// `/archive/` and `/releases/download/` layouts.
fn parse_tarball(url: &str) -> Option<(String, String)> {
    let (base, rest) = url
        .split_once("/archive/")
        .or_else(|| url.split_once("/releases/download/"))?;
    let rest = rest.strip_prefix("refs/tags/").unwrap_or(rest);
    let first = rest.split('/').next().unwrap_or(rest);
    let git_ref = first
        .strip_suffix(".tar.gz")
        .or_else(|| first.strip_suffix(".tar.xz"))
        .or_else(|| first.strip_suffix(".zip"))
        .unwrap_or(first);
    Some((format!("{base}.git"), git_ref.to_string()))
}

/// Fetch the latest commit for a ref, routing to the cheapest method per host.
fn latest_commit(git_url: &str, git_ref: &str, verbose: bool) -> Result<String, String> {
    if git_url.contains("github.com") {
        github_commit(git_url, git_ref, verbose)
    } else if git_url.contains("gitlab.com") {
        gitlab_commit(git_url, git_ref, verbose)
    } else {
        ls_remote(git_url, git_ref, verbose)
    }
}

/// Split a clone URL into (owner, repo).
fn owner_repo(git_url: &str) -> Option<(&str, &str)> {
    let after_scheme = git_url.split_once("://")?.1;
    let path = after_scheme.split_once('/')?.1;
    let path = path.strip_suffix(".git").unwrap_or(path);
    path.split_once('/')
}

/// A GET returning (status, body), with a User-Agent and optional GitHub token.
fn http_get(url: &str, verbose: bool) -> Result<(u16, Vec<u8>), String> {
    if verbose {
        println!("Fetching: {url}");
    }
    let mut request = attohttpc::get(url).header("User-Agent", "flint");
    if url.contains("api.github.com")
        && let Ok(token) = std::env::var("GITHUB_TOKEN")
        && !token.is_empty()
    {
        request = request.header("Authorization", format!("Bearer {token}"));
    }
    let response = request
        .send()
        .map_err(|err| format!("request failed: {err}"))?;
    let status = response.status().as_u16();
    let body = response
        .bytes()
        .map_err(|err| format!("reading response failed: {err}"))?;
    Ok((status, body))
}

fn decode<T: for<'de> Deserialize<'de>>(body: &[u8]) -> Result<T, String> {
    serde_json::from_slice(body).map_err(|err| format!("failed to decode response: {err}"))
}

#[derive(Deserialize, Default)]
struct DefaultBranch {
    #[serde(rename = "default_branch", default)]
    default_branch: String,
}

#[derive(Deserialize, Default)]
struct GitObject {
    #[serde(default)]
    sha: String,
    #[serde(rename = "type", default)]
    kind: String,
}

#[derive(Deserialize, Default)]
struct GitRef {
    #[serde(default)]
    object: GitObject,
}

fn github_commit(git_url: &str, git_ref: &str, verbose: bool) -> Result<String, String> {
    let (owner, repo) =
        owner_repo(git_url).ok_or_else(|| format!("invalid GitHub URL format: {git_url}"))?;

    let mut git_ref = git_ref.to_string();
    if git_ref.is_empty() || git_ref == "HEAD" {
        let (status, body) = http_get(
            &format!("https://api.github.com/repos/{owner}/{repo}"),
            verbose,
        )?;
        if status != 200 {
            return Err(format!("GitHub API returned status {status}"));
        }
        git_ref = decode::<DefaultBranch>(&body)?.default_branch;
    }

    let base = format!("https://api.github.com/repos/{owner}/{repo}/git/refs");
    let (mut status, mut body) = http_get(&format!("{base}/heads/{git_ref}"), verbose)?;
    if status == 404 {
        (status, body) = http_get(&format!("{base}/tags/{git_ref}"), verbose)?;
    }
    if status != 200 {
        return Err(format!(
            "GitHub API returned status {status} for ref {git_ref}"
        ));
    }

    let git_ref_info = decode::<GitRef>(&body)?;
    // Annotated tags point at a tag object; dereference it to the commit.
    if git_ref_info.object.kind == "tag" {
        let tag_url = format!(
            "https://api.github.com/repos/{owner}/{repo}/git/tags/{}",
            git_ref_info.object.sha
        );
        if let Ok((200, tag_body)) = http_get(&tag_url, verbose)
            && let Ok(tag) = decode::<GitRef>(&tag_body)
        {
            return Ok(tag.object.sha);
        }
    }
    Ok(git_ref_info.object.sha)
}

#[derive(Deserialize, Default)]
struct GitlabCommit {
    #[serde(default)]
    commit: GitlabCommitId,
}

#[derive(Deserialize, Default)]
struct GitlabCommitId {
    #[serde(default)]
    id: String,
}

fn gitlab_commit(git_url: &str, git_ref: &str, verbose: bool) -> Result<String, String> {
    let (owner, repo) =
        owner_repo(git_url).ok_or_else(|| format!("invalid GitLab URL format: {git_url}"))?;
    let project = format!("{owner}%2F{repo}");
    let base = format!("https://gitlab.com/api/v4/projects/{project}");

    let mut git_ref = git_ref.to_string();
    if git_ref.is_empty() || git_ref == "HEAD" {
        let (status, body) = http_get(&base, verbose)?;
        if status != 200 {
            return Err(format!("GitLab API returned status {status}"));
        }
        git_ref = decode::<DefaultBranch>(&body)?.default_branch;
    }

    let (mut status, mut body) =
        http_get(&format!("{base}/repository/branches/{git_ref}"), verbose)?;
    if status == 404 {
        (status, body) = http_get(&format!("{base}/repository/tags/{git_ref}"), verbose)?;
    }
    if status != 200 {
        return Err(format!(
            "GitLab API returned status {status} for ref {git_ref}"
        ));
    }

    Ok(decode::<GitlabCommit>(&body)?.commit.id)
}

/// Fallback for hosts without a known API: a libgit2 `ls-remote`.
fn ls_remote(git_url: &str, git_ref: &str, verbose: bool) -> Result<String, String> {
    if verbose {
        println!("Using git ls-remote for: {git_url}");
    }

    let mut remote = git2::Remote::create_detached(git_url.as_bytes())
        .map_err(|err| format!("git ls-remote failed: {err}"))?;
    remote
        .connect(git2::Direction::Fetch)
        .map_err(|err| format!("git ls-remote failed: {err}"))?;
    let refs = remote
        .list()
        .map_err(|err| format!("git ls-remote failed: {err}"))?;

    if git_ref.is_empty() || git_ref == "HEAD" {
        return refs
            .iter()
            .find(|head| head.name() == "HEAD")
            .map(|head| head.oid().to_string())
            .ok_or_else(|| "no HEAD ref found".to_string());
    }

    // Prefer the peeled target of an annotated tag, then a direct branch/tag,
    // then any ref ending in the requested name.
    let peeled = format!("refs/tags/{git_ref}^{{}}");
    let branch = format!("refs/heads/{git_ref}");
    let tag = format!("refs/tags/{git_ref}");
    let suffix = format!("/{git_ref}");

    let pick = |want: &dyn Fn(&str) -> bool| {
        refs.iter()
            .find(|head| want(head.name()))
            .map(|head| head.oid().to_string())
    };

    pick(&|name| name == peeled)
        .or_else(|| pick(&|name| name == branch || name == tag))
        .or_else(|| pick(&|name| name.ends_with(&suffix)))
        .ok_or_else(|| format!("ref {git_ref} not found"))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::lock::Locked;

    fn locked(kind: &str) -> Locked {
        Locked {
            kind: kind.to_string(),
            ..Default::default()
        }
    }

    #[test]
    fn is_commit_hash_cases() {
        assert!(is_commit_hash(&"a".repeat(40)));
        assert!(is_commit_hash("0123456789abcdef0123456789abcdef01234567"));
        assert!(!is_commit_hash(&"a".repeat(39)));
        assert!(!is_commit_hash(&"A".repeat(40))); // uppercase not accepted
        assert!(!is_commit_hash("nixos-unstable"));
    }

    #[test]
    fn owner_repo_parsing() {
        assert_eq!(
            owner_repo("https://github.com/NixOS/nixpkgs.git"),
            Some(("NixOS", "nixpkgs"))
        );
        assert_eq!(
            owner_repo("https://gitlab.com/user/project"),
            Some(("user", "project"))
        );
        assert_eq!(owner_repo("not a url"), None);
    }

    #[test]
    fn normalize_ref_strips_qualifiers() {
        assert_eq!(
            normalize_ref("refs/heads/nixos-unstable".into()),
            "nixos-unstable"
        );
        assert_eq!(normalize_ref("refs/tags/v1.0".into()), "v1.0");
        assert_eq!(normalize_ref("main".into()), "main");
    }

    #[test]
    fn tarball_parsing() {
        assert_eq!(
            parse_tarball("https://github.com/o/r/archive/refs/tags/v1.2.3.tar.gz"),
            Some(("https://github.com/o/r.git".into(), "v1.2.3".into()))
        );
        assert_eq!(
            parse_tarball("https://github.com/o/r/releases/download/v2.0/source.tar.gz"),
            Some(("https://github.com/o/r.git".into(), "v2.0".into()))
        );
        assert_eq!(parse_tarball("https://example.com/x.tar.gz"), None);
    }

    #[test]
    fn host_url_construction() {
        assert_eq!(
            host_git_url("github", &locked("github"), "o", "r"),
            "https://github.com/o/r.git"
        );
        assert_eq!(
            host_git_url("sourcehut", &locked("sourcehut"), "o", "r"),
            "https://git.sr.ht/~o/r.git"
        );
        let hosted = Locked {
            host: "gitlab.example.com".into(),
            ..locked("gitlab")
        };
        assert_eq!(
            host_git_url("gitlab", &hosted, "o", "r"),
            "https://gitlab.example.com/o/r.git"
        );
    }

    #[test]
    fn build_flake_url_cases() {
        assert_eq!(
            build_flake_url(Some(&Locked {
                owner: "NixOS".into(),
                repo: "nixpkgs".into(),
                ..locked("github")
            })),
            "github:NixOS/nixpkgs"
        );
        assert_eq!(
            build_flake_url(Some(&Locked {
                owner: "user".into(),
                repo: "project".into(),
                host: "gitlab.example.com".into(),
                ..locked("gitlab")
            })),
            "gitlab:user/project?host=gitlab.example.com"
        );
        assert_eq!(
            build_flake_url(Some(&Locked {
                url: "https://example.com/repo.git".into(),
                ..locked("git")
            })),
            "https://example.com/repo.git"
        );
        assert_eq!(build_flake_url(None), "");
    }

    #[test]
    fn evergreen_input_is_never_an_update() {
        let lock: FlakeLock = serde_json::from_str(
            r#"{
  "nodes": {
    "pkg": {
      "locked": { "type": "github", "owner": "o", "repo": "r", "rev": "deadbeef" },
      "original": { "type": "github", "owner": "o", "repo": "r", "ref": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" }
    },
    "root": { "inputs": { "pkg": "pkg" } }
  },
  "root": "root"
}"#,
        )
        .unwrap();

        let update = check_input_update(&lock, "pkg", "pkg", false);
        assert!(update.error.is_empty());
        assert!(!update.is_update);
        assert_eq!(update.latest_rev, "deadbeef");
    }

    #[test]
    fn missing_node_errors() {
        let lock: FlakeLock =
            serde_json::from_str(r#"{ "nodes": { "root": { "inputs": {} } } }"#).unwrap();
        let update = check_input_update(&lock, "nonexistent", "missing", false);
        assert!(!update.error.is_empty());
    }

    #[test]
    fn node_without_locked_is_not_a_flake() {
        let lock: FlakeLock = serde_json::from_str(
            r#"{ "nodes": {
                "no-lock": { "original": { "owner": "someone", "repo": "something" } },
                "root": { "inputs": { "no-lock": "no-lock" } }
            } }"#,
        )
        .unwrap();
        let update = check_input_update(&lock, "no-lock", "no-lock", false);
        assert_eq!(
            update.error,
            "input no-lock is not a flake (no locked version)"
        );
    }

    #[test]
    fn check_updates_requires_root_inputs() {
        let no_inputs: FlakeLock = serde_json::from_str(r#"{ "nodes": { "root": {} } }"#).unwrap();
        assert!(check_updates(&no_inputs, false).is_err());

        let no_root: FlakeLock = serde_json::from_str(r#"{ "nodes": {} }"#).unwrap();
        assert!(check_updates(&no_root, false).is_err());
    }
}
