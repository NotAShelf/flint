<!-- markdownlint-disable MD033 MD059 -->

<h1 id="header" align="center">
    <pre>Flint</pre>
</h1>

<div align="center">
    <a alt="CI" href="https://github.com/NotAShelf/flint/actions">
        <img
          src="https://github.com/NotAShelf/flint/actions/workflows/go.yml/badge.svg"
          alt="Build Status"
        />
    </a>
</div>

<div align="center">
  Flint (<i>flake input linter</i>) is a simple, fast, composable utility for
  analyzing a `flake.lock` for duplicate inputs.
</div>

## Usage

```bash
Usage:
  flint [flags]

Examples:
  flint --lockfile=/path/to/flake.lock --verbose
  flint --lockfile=/path/to/flake.lock --output=json
  flint --lockfile=/path/to/flake.lock --output=plain
  flint --merge

Flags:
      --fail-if-multiple-versions   exit with error if multiple versions found
  -h, --help                        help for flint
  -l, --lockfile string             path to flake.lock (default "flake.lock")
  -m, --merge                       merge all dependants into one list for each input
  -o, --output string               output format: plain, pretty, or json (default "pretty")
  -v, --verbose                     enable verbose output
```

Flint requires a **lockfile** to analyze. By default, Flint will look into the
current directory for a `flake.lock`. If you wish to analyze another lockfile,
you must provide one with `--lockfile` using an absolute path to your
`flake.lock` that you want to analyze.

The `--verbose` option will provide just a little additional information on each
input. Flint, by design, is sufficiently verbose without this argument.

### `--fail-if-multiple-versions`

You can tell Flint to _fail_ if there are duplicate inputs by passing
`--fail-if-multiple-versions`. This is mostly useful for CI/CD purposes, or if
you want to chain Flint into other utilities or scripts.

### Output formats

Flint supports three output formats:

- **`pretty`** (default): Enhanced CI-friendly output with colors, symbols, and
  structured information
- **`plain`**: Clean, minimal output suitable for scripting and legacy systems
- **`json`**: Machine-readable JSON format for programmatic use

The default output format is **pretty**, designed to be both human-readable and
CI-friendly with clear visual hierarchy and actionable recommendations.

For legacy compatibility or when you need minimal output, use `--output=plain`.
For parsing the output programmatically, use `--output=json`.

## CI/CD Integration

Flint is designed to integrate seamlessly with CI/CD pipelines. Use the
`--fail-if-multiple-versions` flag to make your CI fail when duplicate
dependencies are detected. GitHub Actions is provided as an example below. You
may adapt the logic, i.e, the pipeline logic into any platform that supports
installing Nix. You may, also _build with Go_ if necessary.

### GitHub Actions

```yaml
name: Check Flake Dependencies
on: [push, pull_request]

jobs:
  check-dependencies:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout repository
        uses: actions/checkout@v4

      - name: Install Nix
        uses: cachix/install-nix-action@main # pin a versrion instead, this is an example
        with:
          nix_path: nixpkgs=channel:nixos-unstable

      - name: Check for duplicate dependencies
        run: |
          nix run github:NotAShelf/flint -- --fail-if-multiple-versions
```

### Pre-commit Hook

This is a poor example, but it'll generally work. You may chance the `nix run`
call in `entry` with just `flint <flags>` if you have it installed globally or
in your dev shell. You may also use `git-hooks.nix` to evaluate the store path
and use it directly.

Add this to your `.pre-commit-config.yaml`:

```yaml
repos:
  - repo: local
    hooks:
      - id: flint
        name: Check flake dependencies
        entry: nix run github:NotAShelf/flint#flint -- --fail-if-multiple-versions
        language: system
        files: ^flake\.(nix|lock)$
        pass_filenames: false
```

### Exit Codes

Flint indicates its status with two distinct exit codes.

- **0**: Success (no duplicates found, or duplicates found but
  `--fail-if-multiple-versions` not used)
- **1**: Failure (duplicates found and `--fail-if-multiple-versions` flag used,
  or other error)

### Combining with Other Formats

You can combine the CI flag with different output formats for various use cases:

```bash
# Pretty output with CI failure
flint --fail-if-multiple-versions --output=pretty

# JSON output for parsing + CI failure
flint --fail-if-multiple-versions --output=json

# Minimal output with CI failure
flint --fail-if-multiple-versions --output=plain
```

## Understanding Output

### Pretty Output (Default)

The default **pretty** output format provides enhanced, CI-friendly formatting:

```bash
$ flint --lockfile ./flake.lock
# => This will return a pretty result
```

With duplicates, you'll see:

```bash
🔍 Flint - Dependency Analysis Report
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

ℹ Analyzing 3 unique inputs...
⚠ Found 1 inputs with multiple versions (2 total duplicates)

📋 Detailed Analysis:

(1) nixpkgs
   ├─ URL: github:NixOS/nixpkgs
   ├─ Repeats: 2
   ├─ Alias: nixpkgs
   │     └─ Used by: input1
   └─ Alias: nixpkgs_2
         └─ Used by: input2, root

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📊 Summary:

✗ 1 inputs have duplicate versions
⚠ 2 total duplicate dependencies detected

ℹ Recommendation:
   Consider using 'inputs.<name>.follows' in your flake.nix to deduplicate
   dependencies and reduce closure size.

   Example:
   inputs.someInput.inputs.nixpkgs.follows = "nixpkgs";
```

### Plain Output

For minimal, script-friendly output, use `--output=plain`:

```bash
$ flint --lockfile ./flake.lock --output=plain
# => This will return a plain, compact result
```

Output:

```bash
Dependency Analysis Report
Input: github:NixOS/nixpkgs
  Alias: nixpkgs
    Dependants: input1
  Alias: nixpkgs_2
    Dependants: input2
  Alias: nixpkgs_3
    Dependants: root
```

This means that you have two inputs, **input1** and **input2**, pulling separate
instances of nixpkgs in addition to `root`, which means nixpkgs is also pulled
by your current flake.

```nix
# flake.nix
{
  inputs = {
    # Your flake pulling nixpkgs
    nixpkgs.url = "github:NixOS/nixpkgs";

    # Inputs pulling their own instances of nixpkgs
    input1.url = "github:foo/bar";
    input2.url = "github:foo/baz";
  };
}
```

You would clear duplicates by `follow`ing the root nixpkgs URL.

```nix
# flake.nix
{
  inputs = {
    # Your flake pulling nixpkgs. Arguments such as 'rev' or 'ref' are
    # not relevant to Flint's use-case.
    nixpkgs.url = "github:NixOS/nixpkgs?ref=nixos-unstable";

    # Inputs pulling their own instances of nixpkgs
    input1 = {
      url = "github:foo/bar";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    input2 = {
      url = "github:foo/baz";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };
}
```

Running Flint again after locking your flake with `nix flake lock` would return:

**Pretty output:**

```bash
🔍 Flint - Dependency Analysis Report
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

ℹ Analyzing 1 unique inputs...
✓ No duplicate inputs detected

All inputs use unique versions. Your dependency tree is optimized!
```

**Plain output:**

```bash
Dependency Analysis Report
No duplicate inputs detected in the repositories analyzed.
```

<p align="center">
  <img src="https://imgs.xkcd.com/comics/manuals.png" alt="Mandatory xkcd comic">
</p>

## Hacking

Clone the repository and run `nix develop`. A `.envrc` is provided for Direnv
users. Otherwise you will need Go installed. Flint does not have any build-time
dependencies.

## License

This project is made available under Mozilla Public License (MPL) version 2.0.
See [LICENSE](LICENSE) for more details on the exact conditions. An online copy
is provided [here](https://www.mozilla.org/en-US/MPL/2.0/).

<div align="right">
  <a href="#doc-begin">Back to the Top</a>
  <br/>
</div>

<!-- markdownlint-enable MD033 -->
