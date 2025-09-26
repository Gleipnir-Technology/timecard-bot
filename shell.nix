with import <nixpkgs> {};
with pkgs.python3Packages;

buildPythonPackage rec {
  name = "timecarder";
  src = ./timecarder;
  propagatedBuildInputs = [ pkgs.python312Packages.matrix-nio ];
}
