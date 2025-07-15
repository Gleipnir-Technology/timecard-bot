{ pkgs ? import <nixpkgs> { } }:

let
  flake = import ./flake.nix;
  flakeOutputs = flake.outputs {
    self = flake;
    nixpkgs = pkgs;
    # Add any other flake inputs that your flake.nix requires
  };
in
flakeOutputs.packages.${pkgs.system}.default
