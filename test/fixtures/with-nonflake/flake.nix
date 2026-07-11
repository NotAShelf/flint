{
  description = "Test flake with a non-flake input";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

    # A non-flake, git-type input, exercised by flint's non-flake code path.
    direnv-nvim = {
      url = "git+https://github.com/notashelf/direnv.nvim";
      flake = false;
    };
  };

  # Outputs are irrelevant; this flake exists only to produce a lockfile.
  outputs = _: {};
}
