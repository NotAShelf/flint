{
  mkShell,
  cargo,
  rustc,
  clippy,
  rustfmt,
  rust-analyzer,
  pkg-config,
  libgit2,
  openssl,
}:
mkShell {
  name = "flint";

  strictDeps = true;
  nativeBuildInputs = [
    cargo
    rustc
    clippy
    (rustfmt.override {asNightly = true;})
    rust-analyzer
    pkg-config
  ];

  buildInputs = [
    libgit2.dev
    openssl.dev
  ];

  # Link against the system libgit2 rather than building the vendored copy.
  env.LIBGIT2_NO_VENDOR = "1";
}
