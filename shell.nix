{ pkgs ? import <nixpkgs> {} }:

pkgs.mkShell {
  packages = with pkgs; [
    gcc
    gnumake
    go
    pkg-config
    sqlite
  ];

  shellHook = ''
    export CGO_ENABLED=1
  '';
}
