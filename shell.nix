# Development shell for mm-web: provides the Node.js toolchain (node + npm)
# and the Go toolchain (go), enough to run `npm install`, `npm run build`
# and the Go backend.
#
# Usage:
#   nix-shell                      # enter the shell
#   nix-shell --run 'npm run build'
#
# Requires <nixpkgs> to be resolvable (NIX_PATH or `nix-shell --impure`).
{ pkgs ? import <nixpkgs> { } }:

pkgs.mkShell {
  name = "mm-web";
  buildInputs = [
    pkgs.go
    pkgs.nodejs
  ];
}
