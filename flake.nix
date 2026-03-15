{
  description = "FleetTUI - A TUI for managing and monitoring server fleets";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages = {
          default = pkgs.buildGoModule {
            pname = "fleettui";
            version = "0.1.0";

            src = ./.;

            vendorHash = "sha256-GgS3itX4+/GsdP2dCZCYtMqql9lY/kddicYVQQ+NcoE=";

            meta = with pkgs.lib; {
              description = "A TUI for managing and monitoring server fleets";
              homepage = "https://github.com/JustAlternate/fleettui";
              license = licenses.mit;
              maintainers = [ ];
            };
          };
        };

        apps = {
          default = {
            type = "app";
            program = "${self.packages.${system}.default}/bin/fleettui";
          };
        };

        devShells = {
          default = pkgs.mkShell {
            packages = with pkgs; [
              go
              gopls
              gotools
              go-tools
            ];
          };
        };
      }
    );
}
