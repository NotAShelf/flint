//! For my next trick, I'l turn a lockfile into dependency relations.
//! Using nothing but this silly module!
use crate::lock::{FlakeLock, InputRef, Locked};
use std::collections::BTreeMap;

/// `deps` maps a version URL to the nodes that reference it; `reverse_deps` maps
/// a node name to the nodes referencing it. Both are BTreeMaps so output order
/// is stable.
#[derive(Debug, Default)]
pub struct Relations {
    pub deps: BTreeMap<String, Vec<String>>,
    pub reverse_deps: BTreeMap<String, Vec<String>>,
}

/// The repository identity plus a `rev`/`narHash` query, uniquely keying a
/// locked version.
pub fn flake_url(locked: &Locked) -> String {
    let mut url = repo_url(locked);

    let (rev, nar_hash) = (&locked.rev, &locked.nar_hash);
    if !rev.is_empty() || !nar_hash.is_empty() {
        url.push('?');
        if !rev.is_empty() {
            url.push_str("rev=");
            url.push_str(rev);
        }
        if !nar_hash.is_empty() {
            if !rev.is_empty() {
                url.push('&');
            }
            url.push_str("narHash=");
            url.push_str(nar_hash);
        }
    }

    url
}

/// The repository identity, without any version query.
pub fn repo_url(locked: &Locked) -> String {
    match locked.kind.as_str() {
        "github" | "gitlab" | "sourcehut" => {
            let mut url = format!("{}:{}/{}", locked.kind, locked.owner, locked.repo);
            if !locked.host.is_empty() {
                url.push_str(&format!("?host={}", locked.host));
            }
            url
        }
        "git" | "hg" | "tarball" => format!("{}:{}", locked.kind, locked.url),
        "path" => format!("{}:{}", locked.kind, locked.path),
        _ => String::new(),
    }
}

/// Build the dependency relations for a lockfile in a single pass, which is also
/// what keeps circular references from looping forever.
pub fn analyze_flake(lock: &FlakeLock) -> Relations {
    let node_urls: BTreeMap<&str, String> = lock
        .nodes
        .iter()
        .filter_map(|(name, node)| {
            let url = flake_url(node.locked.as_ref()?);
            (!url.is_empty()).then_some((name.as_str(), url))
        })
        .collect();

    let mut deps: BTreeMap<String, Vec<String>> = BTreeMap::new();
    let mut reverse_deps: BTreeMap<String, Vec<String>> = BTreeMap::new();

    let mut record = |target: &str, referrer: &str| {
        if let Some(url) = node_urls.get(target) {
            deps.entry(url.clone())
                .or_default()
                .push(referrer.to_owned());
            reverse_deps
                .entry(target.to_owned())
                .or_default()
                .push(referrer.to_owned());
        }
    };

    for (name, node) in &lock.nodes {
        let Some(inputs) = &node.inputs else { continue };
        for target in inputs.values() {
            match target {
                InputRef::One(t) => record(t, name),
                InputRef::Follows(path) => path.iter().for_each(|t| record(t, name)),
            }
        }
    }

    Relations { deps, reverse_deps }
}

/// Strip the version query from a URL, keeping a `?host=` parameter but cutting a
/// later `?` that begins the version.
pub fn extract_repo_identity(url: &str) -> String {
    if let Some(host_idx) = url.find("?host=") {
        let prefix_len = host_idx + "?host=".len();
        return match url[prefix_len..].find('?') {
            Some(version_idx) => url[..prefix_len + version_idx].to_owned(),
            None => url.to_owned(),
        };
    }

    match url.find('?') {
        Some(idx) => url[..idx].to_owned(),
        None => url.to_owned(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::lock::FlakeLock;

    fn load(data: &str) -> FlakeLock {
        serde_json::from_str(data).expect("failed to unmarshal lock")
    }

    #[test]
    fn single_input() {
        let data = r#"
{
  "nodes": {
    "nixpkgs": {
      "locked": {
        "lastModified": 1759381078,
        "narHash": "sha256-abc",
        "owner": "NixOS",
        "repo": "nixpkgs",
        "rev": "abcdef",
        "type": "github"
      }
    },
    "root": {
      "inputs": {
        "nixpkgs": "nixpkgs"
      }
    }
  },
  "root": "root",
  "version": 7
}
"#;
        let result = analyze_flake(&load(data));
        assert_eq!(result.deps.len(), 1);
        let (url, aliases) = result.deps.iter().next().unwrap();
        assert_eq!(url, "github:NixOS/nixpkgs?rev=abcdef&narHash=sha256-abc");
        assert_eq!(aliases, &vec!["root".to_string()]);
    }

    #[test]
    fn duplicate_inputs_same_version() {
        let data = r#"
{
  "nodes": {
    "nixpkgs": {
      "locked": {
        "lastModified": 1759381078,
        "narHash": "sha256-abc",
        "owner": "NixOS",
        "repo": "nixpkgs",
        "rev": "abcdef",
        "type": "github"
      }
    },
    "foo": { "inputs": { "nixpkgs": "nixpkgs" } },
    "bar": { "inputs": { "nixpkgs": "nixpkgs" } }
  },
  "root": "foo",
  "version": 7
}
"#;
        let result = analyze_flake(&load(data));
        assert_eq!(result.deps.len(), 1);
        let (url, aliases) = result.deps.iter().next().unwrap();
        assert_eq!(url, "github:NixOS/nixpkgs?rev=abcdef&narHash=sha256-abc");
        assert_eq!(aliases.len(), 2);
        assert!(aliases.contains(&"foo".to_string()));
        assert!(aliases.contains(&"bar".to_string()));
    }

    #[test]
    fn duplicate_inputs_two_versions() {
        let data = r#"
{
  "nodes": {
    "nixpkgs": {
      "locked": {
        "narHash": "sha256-abc", "owner": "NixOS", "repo": "nixpkgs",
        "rev": "abcdef", "type": "github"
      }
    },
    "nixpkgs2": {
      "locked": {
        "narHash": "sha256-def", "owner": "NixOS", "repo": "nixpkgs",
        "rev": "fedcba", "type": "github"
      }
    },
    "foo": { "inputs": { "nixpkgs": "nixpkgs" } },
    "bar": { "inputs": { "nixpkgs": "nixpkgs2" } }
  },
  "root": "foo",
  "version": 7
}
"#;
        let result = analyze_flake(&load(data));
        assert_eq!(result.deps.len(), 2);
        assert_eq!(
            result.deps["github:NixOS/nixpkgs?rev=abcdef&narHash=sha256-abc"],
            vec!["foo".to_string()]
        );
        assert_eq!(
            result.deps["github:NixOS/nixpkgs?rev=fedcba&narHash=sha256-def"],
            vec!["bar".to_string()]
        );
    }

    #[test]
    fn transitive_dependencies() {
        let data = r#"
{
  "nodes": {
    "baz": { "locked": { "narHash": "sha256-baz", "owner": "example", "repo": "baz", "rev": "baz123", "type": "github" } },
    "bar": { "locked": { "narHash": "sha256-bar", "owner": "example", "repo": "bar", "rev": "bar456", "type": "github" }, "inputs": { "baz": "baz" } },
    "foo": { "locked": { "narHash": "sha256-foo", "owner": "example", "repo": "foo", "rev": "foo789", "type": "github" }, "inputs": { "bar": "bar" } },
    "root": { "inputs": { "foo": "foo" } }
  },
  "root": "root",
  "version": 7
}
"#;
        let result = analyze_flake(&load(data));
        assert_eq!(result.deps.len(), 3);
        assert_eq!(
            result.deps["github:example/baz?rev=baz123&narHash=sha256-baz"],
            vec!["bar".to_string()]
        );
        assert_eq!(
            result.deps["github:example/bar?rev=bar456&narHash=sha256-bar"],
            vec!["foo".to_string()]
        );
        assert_eq!(
            result.deps["github:example/foo?rev=foo789&narHash=sha256-foo"],
            vec!["root".to_string()]
        );
    }

    #[test]
    fn mixed_repository_types() {
        let data = r#"
{
  "nodes": {
    "github-repo": { "locked": { "narHash": "sha256-github", "owner": "owner1", "repo": "repo1", "rev": "github123", "type": "github" } },
    "gitlab-repo": { "locked": { "narHash": "sha256-gitlab", "owner": "owner2", "repo": "repo2", "rev": "gitlab456", "type": "gitlab" } },
    "git-repo": { "locked": { "narHash": "sha256-git", "rev": "git789", "type": "git", "url": "https://example.com/repo.git" } },
    "path-repo": { "locked": { "narHash": "sha256-path", "type": "path", "path": "/local/path" } },
    "tarball-repo": { "locked": { "narHash": "sha256-tarball", "type": "tarball", "url": "https://example.com/archive.tar.gz" } },
    "sourcehut-repo": { "locked": { "narHash": "sha256-srht", "owner": "owner3", "repo": "repo3", "rev": "srht012", "type": "sourcehut" } },
    "root": {
      "inputs": {
        "github": "github-repo", "gitlab": "gitlab-repo", "git": "git-repo",
        "path": "path-repo", "tarball": "tarball-repo", "sourcehut": "sourcehut-repo"
      }
    }
  },
  "root": "root",
  "version": 7
}
"#;
        let result = analyze_flake(&load(data));
        assert_eq!(result.deps.len(), 6);
        for url in [
            "github:owner1/repo1?rev=github123&narHash=sha256-github",
            "gitlab:owner2/repo2?rev=gitlab456&narHash=sha256-gitlab",
            "git:https://example.com/repo.git?rev=git789&narHash=sha256-git",
            "path:/local/path?narHash=sha256-path",
            "tarball:https://example.com/archive.tar.gz?narHash=sha256-tarball",
            "sourcehut:owner3/repo3?rev=srht012&narHash=sha256-srht",
        ] {
            assert_eq!(result.deps[url], vec!["root".to_string()], "url {url}");
        }
    }

    #[test]
    fn hosted_gitlab_double_question_mark() {
        let data = r#"
{
  "nodes": {
    "gitlab-repo": {
      "locked": {
        "narHash": "sha256-gitlab", "owner": "user", "repo": "project",
        "rev": "gitlab123", "type": "gitlab", "host": "gitlab.example.com"
      }
    },
    "root": { "inputs": { "gitlab": "gitlab-repo" } }
  },
  "root": "root",
  "version": 7
}
"#;
        let result = analyze_flake(&load(data));
        assert_eq!(result.deps.len(), 1);
        let (url, aliases) = result.deps.iter().next().unwrap();
        assert_eq!(
            url,
            "gitlab:user/project?host=gitlab.example.com?rev=gitlab123&narHash=sha256-gitlab"
        );
        assert_eq!(aliases, &vec!["root".to_string()]);
    }

    #[test]
    fn edge_case_empty_inputs() {
        let data = r#"{ "nodes": { "root": {} }, "root": "root", "version": 7 }"#;
        assert_eq!(analyze_flake(&load(data)).deps.len(), 0);
    }

    #[test]
    fn edge_case_locked_but_no_inputs() {
        let data = r#"{
  "nodes": {
    "some-repo": { "locked": { "narHash": "sha256-test", "owner": "owner", "repo": "repo", "rev": "rev123", "type": "github" } },
    "root": { "inputs": {} }
  },
  "root": "root", "version": 7
}"#;
        assert_eq!(analyze_flake(&load(data)).deps.len(), 0);
    }

    #[test]
    fn edge_case_circular_reference() {
        let data = r#"{
  "nodes": {
    "a": { "locked": { "narHash": "sha256-a", "owner": "owner", "repo": "a", "rev": "a123", "type": "github" }, "inputs": { "b": "b" } },
    "b": { "locked": { "narHash": "sha256-b", "owner": "owner", "repo": "b", "rev": "b456", "type": "github" }, "inputs": { "a": "a" } },
    "root": { "inputs": { "a": "a" } }
  },
  "root": "root", "version": 7
}"#;
        assert_eq!(analyze_flake(&load(data)).deps.len(), 2);
    }

    #[test]
    fn edge_case_missing_reference() {
        let data = r#"{
  "nodes": { "root": { "inputs": { "nonexistent": "missing-node" } } },
  "root": "root", "version": 7
}"#;
        assert_eq!(analyze_flake(&load(data)).deps.len(), 0);
    }

    #[test]
    fn edge_case_array_inputs_follows() {
        let data = r#"{
  "nodes": {
    "shared": { "locked": { "narHash": "sha256-shared", "owner": "owner", "repo": "shared", "rev": "shared123", "type": "github" } },
    "package1": { "locked": { "narHash": "sha256-pkg1", "owner": "owner", "repo": "package1", "rev": "pkg1456", "type": "github" }, "inputs": { "shared": ["shared"] } },
    "root": { "inputs": { "pkg1": "package1" } }
  },
  "root": "root", "version": 7
}"#;
        assert_eq!(analyze_flake(&load(data)).deps.len(), 2);
    }

    #[test]
    fn extract_repo_identity_cases() {
        let cases = [
            (
                "github:NixOS/nixpkgs?rev=abcdef&narHash=sha256-abc",
                "github:NixOS/nixpkgs",
            ),
            (
                "gitlab:user/project?host=gitlab.example.com?rev=123&narHash=hash",
                "gitlab:user/project?host=gitlab.example.com",
            ),
            (
                "git:https://example.com/repo.git?rev=abc&narHash=hash",
                "git:https://example.com/repo.git",
            ),
            ("path:/local/path?narHash=hash", "path:/local/path"),
            (
                "tarball:https://example.com/archive.tar.gz?narHash=hash",
                "tarball:https://example.com/archive.tar.gz",
            ),
            ("github:owner/repo", "github:owner/repo"),
        ];
        for (input, expected) in cases {
            assert_eq!(extract_repo_identity(input), expected, "input {input}");
        }
    }
}
