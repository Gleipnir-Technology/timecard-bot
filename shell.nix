with import <nixpkgs> {};
let
  timecarder = pkgs.python312Packages.buildPythonPackage rec {
    name = "timecarder";
    propagatedBuildInputs = [ pkgs.python312Packages.matrix-nio ];
    src = ./timecarder;
  };
in
mkShell {
  buildInputs = [ lefthook timecarder ];
}
