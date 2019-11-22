let
  pkgs = (import (builtins.fetchTarball {
    url = "https://github.com/NixOS/nixpkgs/archive/5de728659b412bcf7d18316a4b71d9a6e447f460.tar.gz";
    sha256 = "1bdykda8k8gl2vcp36g27xf3437ig098yrhjp0hclv7sn6dp2w1l";
  })) {};
in
  pkgs.mkShell {
    buildInputs = [
      pkgs.go_1_12
      pkgs.statik
    ];
  }
