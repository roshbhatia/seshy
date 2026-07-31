{
  description = "seshy: minimal multi-repo, git-worktree session manager (sy)";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      nixpkgs,
      flake-utils,
      ...
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        # Keep in step with `version` in cmd/root.go, which is a plain const
        # rather than an ldflags injection. `sy --version` reads that const, so a
        # mismatch here would only ever be visible in the store path.
        version = "4.0.0";

        seshy = pkgs.buildGoModule {
          pname = "seshy";
          inherit version;
          src = ./.;

          # `nix build` reports the correct value when this is wrong. Bump it
          # whenever go.mod or go.sum changes.
          vendorHash = "sha256-6B9O6ho4COpJy4HlkzQ0lk+ieezRO3xg9LyLHzoxYzc=";

          # No `subPackages`. It scopes the CHECK phase as well as the build, so
          # restricting it to "." made `go test` run against the root package
          # only, which has no test files: the build reported a passing test
          # phase while testing nothing. The root is the only main package, so
          # building everything still produces exactly one binary.

          # Taskfile.yml sets BINARY_NAME to `sy`, and every caller (shell
          # integration, hooks, the sysinit gate) invokes `sy`. buildGoModule
          # names the output after the module's last path segment, which is
          # `seshy`, so rename it and keep `seshy` as an alias.
          postInstall = ''
            mv "$out/bin/seshy" "$out/bin/sy"
            ln -s "$out/bin/sy" "$out/bin/seshy"
          '';

          # The unit tests shell out to `git` to build worktree fixtures, so it
          # has to be on PATH inside the sandbox. Without it every delete test
          # fails with "executable file not found".
          nativeCheckInputs = [ pkgs.git ];

          # The integration and e2e suites are Docker-driven (see test/), so the
          # sandbox cannot run them. The unit tests under cmd/ and internal/ are
          # the ones that gate a build, and they do run.
          checkFlags = [ "-skip=TestIntegration|TestE2E" ];

          meta = with pkgs.lib; {
            description = "Minimal session manager for multi-repo, worktree-based work";
            homepage = "https://github.com/roshbhatia/seshy";
            license = licenses.mit;
            mainProgram = "sy";
            platforms = platforms.unix;
          };
        };
      in
      {
        packages = {
          inherit seshy;
          default = seshy;
        };

        apps.default = flake-utils.lib.mkApp {
          drv = seshy;
          name = "sy";
        };

        devShells.default = pkgs.mkShellNoCC {
          packages = [
            pkgs.go
            pkgs.go-task
            pkgs.gopls
          ];
        };

        checks.seshy = seshy;
      }
    );
}
