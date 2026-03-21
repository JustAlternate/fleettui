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
            version = "1.2.0";

            src = ./.;

            vendorHash = "sha256-dGTjOHXuYItOGR+oTwWlEEdyToVXCVIK7s1hTiROLpw=";

            meta = with pkgs.lib; {
              description = "A TUI for managing and monitoring server fleets";
              homepage = "https://github.com/JustAlternate/fleettui";
              license = licenses.mit;
              maintainers = [ "loicw@justalternate.com" ];
            };
          };
        };

        apps = {
          default = {
            type = "app";
            program = "${self.packages.${system}.default}/bin/fleettui";
          };
        };
      }
    );
}
