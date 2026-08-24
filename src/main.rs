mod analyze;
mod lock;
mod output;
mod updates;

use pound::Parse;
use std::path::PathBuf;
use std::process::ExitCode;

use lock::FlakeLock;
use output::Options;

/// Analyze flake.lock for duplicate inputs and check updates.
#[derive(Parse)]
#[pound(name = "flint")]
struct Args {
    /// Path to flake.lock.
    #[pound(short = 'l', long, default = "flake.lock")]
    lockfile: String,

    /// Enable verbose output.
    #[pound(short, long)]
    verbose: bool,

    /// Exit with error if multiple versions found.
    #[pound(long)]
    fail_if_multiple_versions: bool,

    /// Output format: plain, pretty, or json.
    #[pound(short = 'o', long, default = "pretty")]
    output: String,

    /// Merge all dependants into one list for each input.
    #[pound(short, long)]
    merge: bool,

    /// Suppress all non-error output.
    #[pound(short, long)]
    quiet: bool,

    /// Check for available updates for flake inputs.
    #[pound(short = 'u', long)]
    check_updates: bool,

    /// Print version information.
    #[pound(long)]
    version: bool,
}

fn main() -> ExitCode {
    let args = Args::parse();

    if args.version {
        println!("flint version {}", env!("CARGO_PKG_VERSION"));
        return ExitCode::SUCCESS;
    }

    if output::no_color() {
        yansi::disable();
    }

    // A directory argument means "look for flake.lock inside it".
    let mut lockfile = PathBuf::from(&args.lockfile);
    if lockfile.is_dir() {
        lockfile.push("flake.lock");
    }

    let data = match std::fs::read_to_string(&lockfile) {
        Ok(data) => data,
        Err(err) => {
            eprintln!("error reading {}: {err}", lockfile.display());
            return ExitCode::FAILURE;
        }
    };

    let lock: FlakeLock = match serde_json::from_str(&data) {
        Ok(lock) => lock,
        Err(err) => {
            eprintln!("error decoding flake.lock: {err}");
            return ExitCode::FAILURE;
        }
    };

    let options = Options {
        output_format: args.output,
        verbose: args.verbose,
        merge: args.merge,
        fail_if_multiple_versions: args.fail_if_multiple_versions,
        quiet: args.quiet,
    };

    if args.check_updates {
        let results = match updates::check_updates(&lock, options.verbose) {
            Ok(results) => results,
            Err(err) => {
                eprintln!("error checking updates: {err}");
                return ExitCode::FAILURE;
            }
        };
        if let Err(err) = output::print_updates(&results, &options) {
            eprintln!("Error: {err}");
            return ExitCode::FAILURE;
        }
        return ExitCode::SUCCESS;
    }

    let relations = analyze::analyze_flake(&lock);
    if let Err(err) = output::print_dependencies(&relations.deps, &relations.reverse_deps, &options)
    {
        eprintln!("Error: {err}");
        return ExitCode::FAILURE;
    }

    if output::should_fail_on_duplicates(&options, &relations.deps) {
        return ExitCode::FAILURE;
    }
    ExitCode::SUCCESS
}
