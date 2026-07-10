{
  lib,
  buildGoModule,
  ...
}:
buildGoModule (finalAttrs: {
  pname = "flint";
  version = "0.3.0";

  src = let
    fs = lib.fileset;
    s = ../.;
  in
    fs.toSource {
      root = s;
      fileset = fs.unions [
        ../cmd
        ../internal
        ../vendor
        ../main.go
        ../go.mod
        ../go.sum
      ];
    };

  vendorHash = null;
  ldflags = ["-s" "-w" "-X main.version=${finalAttrs.version}"];

  meta = {
    description = "Stupid simple utility for linting your flake inputs";
    homepage = "https://github.com/notashelf/flint";
    license = lib.licenses.mpl20;
    mainProgram = "flint";
    maintainers = [lib.maintainers.NotAShelf];
  };
})
