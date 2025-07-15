# file: default.nix
let
  sources = import ./nix/sources.nix;
  pkgs = import sources.nixpkgs { };
  # Let all API attributes like "poetry2nix.mkPoetryApplication"
  # use the packages and versions (python3, poetry etc.) from our pinned nixpkgs above
  # under the hood:
  poetry2nix = import sources.poetry2nix { inherit pkgs; };
  myPythonApp = poetry2nix.mkPoetryApplication { projectDir = ./.; };
in
myPythonApp
