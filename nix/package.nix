{
  lib,
  rustPlatform,
  pkg-config,
  libgit2,
  openssl,
  zlib,
  ...
}:
rustPlatform.buildRustPackage (finalAttrs: {
  pname = "flint";
  version = "0.3.0";

  src = let
    fs = lib.fileset;
    s = ../.;
  in
    fs.toSource {
      root = s;
      fileset = fs.unions [
        ../Cargo.toml
        ../Cargo.lock
        ../src
      ];
    };

  cargoLock.lockFile = ../Cargo.lock;

  nativeBuildInputs = [pkg-config];
  buildInputs = [libgit2 openssl zlib];

  # Link against the system libgit2 rather than building the vendored copy.
  env.LIBGIT2_NO_VENDOR = "1";

  meta = {
    description = "Stupid simple utility for linting your flake inputs";
    homepage = "https://github.com/notashelf/flint";
    license = lib.licenses.mpl20;
    mainProgram = "flint";
    maintainers = [lib.maintainers.NotAShelf];
  };
})
