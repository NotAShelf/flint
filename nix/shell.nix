{
  mkShell,
  go,
  gopls,
  delve,
}:
mkShell {
  name = "flint";
  packages = [
    go
    gopls
    delve
  ];
}
