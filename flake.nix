{
  description = "AISH packaging flake";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
        };
        version = if self ? rev then self.rev else "0.0.0-dev";
        aish = pkgs.buildGoModule {
          pname = "aish";
          inherit version;
          src = ./.;
          vendorHash = pkgs.lib.fakeSha256;
          subPackages = [ "cmd/aish" ];
          ldflags = [
            "-s"
            "-w"
            "-X main._version=${version}"
          ];
        };
      in {
        packages = {
          default = aish;
          aish = aish;
        };
        apps.aish = {
          type = "app";
          program = "${aish}/bin/aish";
        };
        devShells.default = pkgs.mkShell {
          buildInputs = [
            pkgs.go
            pkgs.gofumpt
            pkgs.goimports
          ];
        };
      });
}
