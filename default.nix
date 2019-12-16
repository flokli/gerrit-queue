let
  pkgs = import <nixpkgs> {};
in
  pkgs.callPackage ./gerrit-queue.nix {}
