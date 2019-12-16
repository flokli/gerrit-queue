{ lib, buildGoModule, statik }:
buildGoModule {
  name = "gerrit-queue";

  # TODO: filter out all the files from .gitignore
  src = lib.cleanSource ./.;

  # TODO: either avoid or automate the generation of that hash everytime
  #       the go.sum file changes
  modSha256 = "0kwbj736mlq0n2pnbi93ry20ndijd3cw5mli6ky1zkf5618gddk1";

  preBuild = ''
    go generate ./...
  '';

  # create a static binary
  CGO_ENABLED = 0;
}
