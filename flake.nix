{
  description = "Development shell for openwrt-singbox";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  inputs.systems.url = "github:nix-systems/default";
  inputs.flake-utils = {
    url = "github:numtide/flake-utils";
    inputs.systems.follows = "systems";
  };

  outputs =
    { nixpkgs, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            h
            binutils
            bzip2
            coreutils
            direnv
            file
            findutils
            gawk
            gcc
            gettext
            git
            gnumake
            go
            gnutar
            gzip
            iproute2
            ncurses
            openssh
            patch
            perl
            pkg-config
            python3
            qemu
            rsync
            unzip
            util-linux
            which
            wget
            xz
            zstd
          ];
          shellHook = ''
            if [ -f .env ]; then
              export $(grep -v '^#' .env | xargs)
            fi
          '';
        };
      }
    );
}
