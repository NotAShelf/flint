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

## Understanding Output

Lets assume you are analyzing a simple `flake.lock` with the plain output
format.

```bash
$ flint --lockfile ./flake.lock --output=plain
```

You will get something of this sort:

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
    # Your flake pulling nixpkgs
    nixpkgs.url = "github:NixOS/nixpkgs";

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

```bash
Dependency Analysis Report
No duplicate inputs detected in the repositories analyzed.
```

![xckd](https://imgs.xkcd.com/comics/manuals.png)

## License

Flint is licensed under Mozilla Public License 2.0, please see
[LICENSE](LICENSE) for more details.
