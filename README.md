# Flint

Flint (_flake input linter_) is a utility for analyzing a given `flake.lock` for
duplicate inputs.

## Usage

```bash
Usage: flint [options]

Options:
  -fail-if-multiple-versions
        exit with error if multiple versions found
  -lockfile string
        path to flake.lock (default "flake.lock")
  -output string
        output format: plain or json (default "plain")
  -verbose
        enable verbose output

Examples:
  flint --lockfile=/path/to/flake.lock --verbose
  flint --lockfile=/path/to/flake.lock --output=json
```

Flint requires a **lockfile** to analyze. By default, Flint will look into the
current directory for a `flake.lock`. If you wish to analyze another lockfile,
you must provide one with `-lockfile` (or `--lockfile`) using an absolute path
to your `flake.lock`.

The `-verbose` (or `--verbose`) option will provide just a little additional
information on each input. Flint, by design, is sufficiently verbose without
this argument.

### `-fail-if-multiple-versions`

You can tell Flint to _fail_ if there are duplicate inputs by passing
`-fail-if-multiple-versions`. This is mostly useful for CI/CD purposes, or if
you want to chain Flint into other utilities or scripts.

### Output formats

The default output format is **plain**, and aims to be human-readable above all.
If you wish to parse the output further, you may pass `-output=json` (or
`--output=json`) to print the full output in **json**.
