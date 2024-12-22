{
  self,
  lib,
  buildGoModule,
  ...
}:
buildGoModule {
  pname = "flint";
  version = "0.1.0";

  src = builtins.path {
    path = self;
    name = "flint-src";
  };

  vendorHash = "sha256-HVynSqH/6GnPutzEsASWAMd7H1veE1ps+MpADKKJEmU=";

  ldflags = ["-s" "-w"];

  meta = {
    description = "Stupid simple utility for linting your flake inputs";
    license = lib.licenses.mpl20;
    mainProgram = "flint";
    maintainers = [lib.maintainers.NotAShelf];
  };
}
