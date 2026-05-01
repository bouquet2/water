{
  description = "Talos Linux and Kubernetes upgrade tool";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  };

  outputs = { self, nixpkgs, ... }@inputs:
    let
      supportedSystems = ["x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin"];
      forEachSupportedSystem = nixpkgs.lib.genAttrs supportedSystems;
      pkgsForEachSystem = system: nixpkgs.legacyPackages.${system};
    in
    {
      packages = forEachSupportedSystem (system:
        let pkgs = pkgsForEachSystem system;
        in
        {
          default = self.packages.${system}.water;
          water = pkgs.buildGoModule {
            pname = "water";
            version = "unstable";
            src = self;
            go = pkgs.go_1_25;
            vendorHash = "sha256-Vd8p9HZkH5cksPRhgsjFowFy1QZolNbX2/iLHBbphD0=";
            env.CGO_ENABLED = 0;
            ldflags = ["-X main.appVersion=unstable"];
          };
        });

      devShells = forEachSupportedSystem (system:
        let pkgs = pkgsForEachSystem system;
        in
        {
          default = pkgs.mkShell {
            packages = with pkgs; [ go_1_25 gopls goreleaser git ];
            shellHook = ''
              echo "Water development environment"
              echo "Available tools: go, gopls, goreleaser, git"
            '';
          };
        });

      apps = forEachSupportedSystem (system: {
        default = {
          type = "app";
          program = "${self.packages.${system}.water}/bin/water";
        };
      });
    };
}