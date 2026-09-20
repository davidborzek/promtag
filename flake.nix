{
  description = "promtag — Prometheus rule files from Docker container labels";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forAllSystems = nixpkgs.lib.genAttrs systems;
    in
    {
      devShells = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          # Mirrors the CI toolchain (ci.yaml): gofmt/vet/test/build run on the
          # go.mod-pinned Go, lint via golangci-lint, vuln via govulncheck.
          default = pkgs.mkShell {
            packages = with pkgs; [
              go_1_26
              golangci-lint
              govulncheck
            ];

            # Keep the nixpkgs toolchain authoritative: a go.mod bump beyond
            # it must surface as an error here, not a silent toolchain download.
            GOTOOLCHAIN = "local";
          };
        }
      );
    };
}
