use crate::analyze::extract_repo_identity;
use crate::updates::UpdateResults;
use std::collections::{BTreeMap, BTreeSet};
use yansi::Paint;

const SEPARATOR: &str = "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━";

type Deps = BTreeMap<String, Vec<String>>;

#[derive(Debug, Default, Clone)]
pub struct Options {
    pub output_format: String,
    pub verbose: bool,
    pub merge: bool,
    pub fail_if_multiple_versions: bool,
    pub quiet: bool,
}

/// True when `NO_COLOR` is set to a non-empty value, per <https://no-color.org>
/// (an empty `NO_COLOR` does not disable color).
pub fn no_color() -> bool {
    std::env::var_os("NO_COLOR").is_some_and(|value| !value.is_empty())
}

/// Status icons: (success, warning, error, info). ASCII under `NO_COLOR`.
fn icons() -> (&'static str, &'static str, &'static str, &'static str) {
    if no_color() {
        ("[✓]", "[!]", "[✗]", "[i]")
    } else {
        ("✓", "⚠", "✗", "ℹ")
    }
}

/// First eight characters of a rev (short revs are not padded) plus an ellipsis.
fn short_rev(rev: &str) -> String {
    format!("{}...", rev.chars().take(8).collect::<String>())
}

/// Group URLs by repository identity, keeping only repositories with more than
/// one version.
pub fn detect_duplicates_by_repo(deps: &Deps) -> Deps {
    let mut groups: Deps = BTreeMap::new();
    for url in deps.keys() {
        groups
            .entry(extract_repo_identity(url))
            .or_default()
            .push(url.clone());
    }
    groups.retain(|_, urls| urls.len() > 1);
    groups
}

pub fn validate_output_format(format: &str) -> Result<(), String> {
    match format {
        "json" | "plain" | "pretty" => Ok(()),
        _ => Err(format!(
            "invalid output format '{format}'. Valid formats are: json, plain, pretty"
        )),
    }
}

pub fn should_fail_on_duplicates(options: &Options, deps: &Deps) -> bool {
    options.fail_if_multiple_versions && !detect_duplicates_by_repo(deps).is_empty()
}

/// Map each URL to its unique dependants, sorted.
fn dependants_of(deps: &Deps) -> Deps {
    deps.iter()
        .map(|(url, aliases)| {
            let unique: BTreeSet<&String> = aliases.iter().collect();
            (url.clone(), unique.into_iter().cloned().collect())
        })
        .collect()
}

pub fn print_dependencies(
    deps: &Deps,
    reverse_deps: &Deps,
    options: &Options,
) -> Result<(), String> {
    validate_output_format(&options.output_format)?;
    if options.quiet {
        return Ok(());
    }

    let duplicates = detect_duplicates_by_repo(deps);

    if options.output_format == "json" {
        let value = serde_json::json!({
            "dependencies": deps,
            "reverse_dependencies": reverse_deps,
            "duplicates": duplicates,
        });
        let json = serde_json::to_string_pretty(&value)
            .map_err(|err| format!("error marshaling JSON output: {err}"))?;
        println!("{json}");
        return Ok(());
    }

    let dependants = dependants_of(deps);
    match options.output_format.as_str() {
        "plain" => plain_dependencies(deps.len(), &duplicates, &dependants, options),
        _ => pretty_dependencies(deps.len(), &duplicates, &dependants, options),
    }
    Ok(())
}

pub fn print_updates(results: &UpdateResults, options: &Options) -> Result<(), String> {
    validate_output_format(&options.output_format)?;
    if options.quiet {
        return Ok(());
    }

    if options.output_format == "json" {
        let json = serde_json::to_string_pretty(results)
            .map_err(|err| format!("error marshaling JSON output: {err}"))?;
        println!("{json}");
        return Ok(());
    }

    match options.output_format.as_str() {
        "plain" => plain_updates(results),
        _ => pretty_updates(results),
    }
    Ok(())
}

fn pretty_dependencies(total: usize, duplicates: &Deps, dependants: &Deps, options: &Options) {
    let (ok, warn, err, info) = icons();

    let duplicate_repos = duplicates.len();
    let duplicate_count: usize = duplicates.values().map(|urls| urls.len() - 1).sum();

    println!(
        "{}",
        "🔍 Flint - Dependency Analysis Report"
            .fixed(12)
            .bold()
            .underline()
    );

    if total == 0 {
        println!(
            "{}",
            format!("{info} No inputs found in lockfile")
                .fixed(14)
                .bold()
        );
        println!();
        return;
    }

    println!(
        "{}",
        format!("{info} Analyzing {total} unique repositories...")
            .fixed(14)
            .bold()
    );

    if duplicate_repos == 0 {
        println!(
            "{}",
            format!("{ok} No duplicate repositories detected")
                .fixed(10)
                .bold()
        );
        println!();
        println!(
            "{}",
            "All repositories use unique versions. Your dependency tree is optimized!".fixed(8)
        );
        return;
    }

    println!(
        "{}",
        format!("{warn} Found {duplicate_repos} repositories with multiple versions ({duplicate_count} total duplicates)")
            .fixed(11)
            .bold()
    );
    println!();
    println!("{}", "📋 Detailed Analysis:".bold());
    println!();

    for (index, (identity, urls)) in duplicates.iter().enumerate() {
        let repo_name = identity.rsplit('/').next().unwrap_or(identity);
        println!("{}", format!("({}) {repo_name}", index + 1).fixed(9).bold());
        println!(
            "   {} {}{}",
            "├─".fixed(8),
            "Repository: ".bold(),
            identity.fixed(6).underline()
        );
        println!(
            "   {} {}",
            "├─".fixed(8),
            format!("Versions: {}", urls.len()).fixed(11).bold()
        );

        if options.merge {
            let used_by: BTreeSet<&String> = urls
                .iter()
                .filter_map(|url| dependants.get(url))
                .flatten()
                .collect();
            if used_by.is_empty() {
                println!("   {} {}", "└─".fixed(8), "No direct dependants".fixed(8));
            } else {
                println!(
                    "   {} {}",
                    "└─".fixed(8),
                    format!("Used by: {}", join(&used_by)).fixed(3)
                );
            }
            println!();
            continue;
        }

        for (i, url) in urls.iter().enumerate() {
            let last = i == urls.len() - 1;
            let connector = if last { "└─" } else { "├─" };
            let branch = if last { " " } else { "│" };

            println!(
                "   {} {}",
                connector.fixed(8),
                format!("Version{}", version_of(url)).fixed(13).italic()
            );

            let used_by = dependants.get(url).map(Vec::as_slice).unwrap_or_default();
            if !used_by.is_empty() {
                println!(
                    "   {}     {} {}",
                    branch.fixed(8),
                    "└─".fixed(8),
                    format!("Used by: {}", used_by.join(", ")).fixed(3)
                );
            }
            if options.verbose {
                println!(
                    "   {}     {} {}",
                    branch.fixed(8),
                    "└─".fixed(8),
                    format!("Debug: {} dependants", used_by.len()).fixed(8)
                );
            }
        }
        println!();
    }

    println!("{}", SEPARATOR.fixed(8));
    println!("{}", "📊 Summary:".bold());
    println!();
    println!(
        "{}",
        format!("{err} {duplicate_repos} repositories have duplicate versions")
            .fixed(9)
            .bold()
    );
    println!(
        "{}",
        format!("{warn} {duplicate_count} total duplicate dependencies detected")
            .fixed(11)
            .bold()
    );
    println!();
    println!("{}", format!("{info} Recommendation:").fixed(14).bold());
    println!("   Consider using 'inputs.<name>.follows' in your flake.nix to deduplicate");
    println!("   dependencies and reduce closure size.");
    println!();
    println!("{}", "   Example:".fixed(8));
    println!(
        "{}",
        "   inputs.someInput.inputs.nixpkgs.follows = \"nixpkgs\";".fixed(8)
    );
}

fn plain_dependencies(_total: usize, duplicates: &Deps, dependants: &Deps, options: &Options) {
    println!(
        "{}",
        "Dependency Analysis Report".fixed(5).bold().underline()
    );

    if duplicates.is_empty() {
        println!(
            "{}",
            "No duplicate repositories detected in the lockfile."
                .fixed(3)
                .bold()
        );
        return;
    }

    for (identity, urls) in duplicates {
        println!("{}", format!("Repository: {identity}").fixed(6).bold());

        if options.merge {
            let used_by: BTreeSet<&String> = urls
                .iter()
                .filter_map(|url| dependants.get(url))
                .flatten()
                .collect();
            if !used_by.is_empty() {
                println!("{}", format!("  Dependants: {}", join(&used_by)).fixed(2));
            }
            continue;
        }

        for url in urls {
            println!("{}", format!("  Version: {url}").fixed(4).italic());
            let used_by = dependants.get(url).map(Vec::as_slice).unwrap_or_default();
            if !used_by.is_empty() {
                println!(
                    "{}",
                    format!("    Dependants: {}", used_by.join(", ")).fixed(2)
                );
            }
            if options.verbose {
                println!(
                    "{}",
                    format!(
                        "    [Debug] {} inputs depend on this version",
                        used_by.len()
                    )
                    .fixed(2)
                );
            }
            println!();
        }
    }
}

fn pretty_updates(results: &UpdateResults) {
    let (ok, warn, err, info) = icons();
    let updates = &results.updates;

    let available = updates.iter().filter(|u| u.is_update).count();
    let errors = updates.iter().filter(|u| !u.error.is_empty()).count();

    println!(
        "{}",
        "🔄 Flint - Update Check Report"
            .fixed(12)
            .bold()
            .underline()
    );
    println!();

    if updates.is_empty() {
        println!(
            "{}",
            format!("{info} No inputs found to check").fixed(14).bold()
        );
        return;
    }

    println!(
        "{}",
        format!("{info} Checked {} inputs for updates...", updates.len())
            .fixed(14)
            .bold()
    );

    if available == 0 && errors == 0 {
        println!(
            "{}",
            format!("{ok} All inputs are up to date!").fixed(10).bold()
        );
        return;
    }

    if available > 0 {
        println!(
            "{}",
            format!("{warn} {available} updates available")
                .fixed(11)
                .bold()
        );
    }
    if errors > 0 {
        println!(
            "{}",
            format!("{err} {errors} errors encountered").fixed(9).bold()
        );
    }
    println!();
    println!("{}", "📋 Detailed Results:".bold());
    println!();

    for (i, u) in updates.iter().enumerate() {
        println!("{}. {}", i + 1, u.input_name.fixed(13).italic());

        if !u.error.is_empty() {
            println!(
                "   {} {}",
                err,
                format!("Error: {}", u.error).fixed(9).bold()
            );
        } else if u.is_update {
            println!("   {} {}", warn, "Update available".fixed(11).bold());
            println!(
                "   {} {}{}",
                "├─".fixed(8),
                "Current: ".bold(),
                short_rev(&u.current_rev).fixed(8)
            );
            println!(
                "   {} {}{}",
                "├─".fixed(8),
                "Latest:  ".bold(),
                short_rev(&u.latest_rev).fixed(10).bold()
            );
            println!(
                "   {} {}",
                "└─".fixed(8),
                u.current_url.fixed(6).underline()
            );
        } else {
            println!("   {} {}", ok, "Up to date".fixed(10).bold());
            println!(
                "   {} {}",
                "└─".fixed(8),
                short_rev(&u.current_rev).fixed(8)
            );
        }
        println!();
    }

    println!("{}", SEPARATOR.fixed(8));
    println!("{}", "📊 Summary:".bold());
    println!();

    if available > 0 {
        println!(
            "{}",
            format!("{warn} {available} inputs have updates available")
                .fixed(11)
                .bold()
        );
        println!(
            "{}",
            "Run 'nix flake update' to update all inputs, or update specific inputs:"
                .fixed(14)
                .bold()
        );
        println!("{}", "  nix flake update <input-name>".fixed(8));
        println!();
    }
    if errors > 0 {
        println!(
            "{}",
            format!("{err} {errors} inputs could not be checked")
                .fixed(9)
                .bold()
        );
        println!(
            "{}",
            "This may be due to network issues or unavailable repositories."
                .fixed(14)
                .bold()
        );
        println!();
    }
    if available == 0 && errors == 0 {
        println!(
            "{}",
            format!("{ok} All inputs are at the latest version")
                .fixed(10)
                .bold()
        );
    }
}

fn plain_updates(results: &UpdateResults) {
    let updates = &results.updates;
    let available = updates.iter().filter(|u| u.is_update).count();
    let errors = updates.iter().filter(|u| !u.error.is_empty()).count();

    println!("{}", "Update Check Report".fixed(5).bold().underline());
    println!();

    if available == 0 && errors == 0 {
        println!("{}", "All inputs are up to date.".fixed(2));
        return;
    }

    for u in updates {
        println!("{}", format!("Input: {}", u.input_name).fixed(6).bold());
        if !u.error.is_empty() {
            println!("{}", format!("  Error: {}", u.error).fixed(9).bold());
        } else if u.is_update {
            println!("{}", "  Status: Update available".fixed(2));
            println!("  Current: {}", short_rev(&u.current_rev));
            println!("  Latest:  {}", short_rev(&u.latest_rev));
            println!("  URL: {}", u.current_url);
        } else {
            println!("{}", "  Status: Up to date".fixed(2));
            println!("  Version: {}", short_rev(&u.current_rev));
        }
        println!();
    }

    if available > 0 {
        println!("{available} inputs have updates available");
    }
    if errors > 0 {
        println!("{errors} inputs could not be checked");
    }
}

/// The rev portion of a URL as ` (rev)`, or empty when there is none.
fn version_of(url: &str) -> String {
    let Some(start) = url.find("?rev=").map(|i| i + 5) else {
        return String::new();
    };
    let end = url[start..].find('&').map_or(url.len(), |i| start + i);
    if end > start {
        format!(" ({})", &url[start..end])
    } else {
        String::new()
    }
}

fn join(names: &BTreeSet<&String>) -> String {
    names
        .iter()
        .map(|s| s.as_str())
        .collect::<Vec<_>>()
        .join(", ")
}

#[cfg(test)]
mod tests {
    use super::*;

    fn deps(entries: &[(&str, &[&str])]) -> Deps {
        entries
            .iter()
            .map(|(url, aliases)| {
                (
                    url.to_string(),
                    aliases.iter().map(|a| a.to_string()).collect(),
                )
            })
            .collect()
    }

    #[test]
    fn validate_output_format_cases() {
        for good in ["json", "plain", "pretty"] {
            assert!(validate_output_format(good).is_ok());
        }
        for bad in ["invalid", "", "JSON"] {
            let err = validate_output_format(bad).unwrap_err();
            assert!(err.contains("json, plain, pretty"), "message: {err}");
        }
    }

    #[test]
    fn should_fail_only_with_flag_and_duplicates() {
        let dupes = deps(&[
            ("github:owner/repo1?rev=abc", &["node1"]),
            ("github:owner/repo1?rev=def", &["node2"]),
        ]);
        let unique = deps(&[
            ("github:owner/repo1?rev=abc", &["node1"]),
            ("github:owner/repo2?rev=def", &["node2"]),
        ]);
        let on = Options {
            fail_if_multiple_versions: true,
            ..Default::default()
        };
        let off = Options {
            fail_if_multiple_versions: false,
            ..Default::default()
        };

        assert!(should_fail_on_duplicates(&on, &dupes));
        assert!(!should_fail_on_duplicates(&off, &dupes));
        assert!(!should_fail_on_duplicates(&on, &unique));
        assert!(!should_fail_on_duplicates(&on, &BTreeMap::new()));
    }

    #[test]
    fn quiet_mode_still_validates_format() {
        let d = deps(&[("github:owner/repo?rev=abc", &["root"])]);
        let rd = deps(&[("repo", &["root"])]);

        let valid = Options {
            quiet: true,
            output_format: "json".into(),
            ..Default::default()
        };
        assert!(print_dependencies(&d, &rd, &valid).is_ok());

        let invalid = Options {
            quiet: true,
            output_format: "bogus".into(),
            ..Default::default()
        };
        let err = print_dependencies(&d, &rd, &invalid).unwrap_err();
        assert!(
            err.contains("invalid output format 'bogus'"),
            "message: {err}"
        );
    }

    #[test]
    fn detect_duplicates_groups_by_identity() {
        let d = deps(&[
            ("github:owner/repo1?rev=abc", &["n1"]),
            ("github:owner/repo1?rev=def", &["n2"]),
            ("github:owner/repo2?rev=ghi", &["n3"]),
        ]);
        let dupes = detect_duplicates_by_repo(&d);
        assert_eq!(dupes.len(), 1);
        assert_eq!(dupes["github:owner/repo1"].len(), 2);
    }

    #[test]
    fn version_of_extracts_rev() {
        assert_eq!(version_of("github:o/r?rev=abc&narHash=x"), " (abc)");
        assert_eq!(version_of("github:o/r?rev=abc"), " (abc)");
        assert_eq!(version_of("github:o/r"), "");
    }

    // Sole test that touches NO_COLOR; no other test reads or renders color, so
    // the process-wide env is never contended.
    #[test]
    fn no_color_follows_the_spec() {
        use std::env;

        unsafe { env::remove_var("NO_COLOR") };
        assert!(!no_color(), "unset means color stays on");

        unsafe { env::set_var("NO_COLOR", "") };
        assert!(!no_color(), "empty value does not disable color");

        unsafe { env::set_var("NO_COLOR", "1") };
        assert!(no_color(), "any non-empty value disables color");

        unsafe { env::remove_var("NO_COLOR") };
    }
}
