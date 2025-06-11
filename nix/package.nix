{
  lib,
  buildGoModule,
  ...
}: let
  fs = lib.fileset;
  version = "0.2.0";
in
  buildGoModule {
    pname = "flint";
    inherit version;

    src = fs.toSource {
      root = ../.;
      fileset = fs.unions [
        ../cmd
        ../internal
        ../vendor
        ../go.mod
        ../go.sum
      ];
    };

    vendorHash = null;

    ldflags = ["-s" "-w" "-X main.version=${version}"];

    meta = {
      description = "Stupid simple utility for linting your flake inputs";
      license = lib.licenses.mpl20;
      mainProgram = "flint";
      maintainers = [lib.maintainers.NotAShelf];
    };
  }
