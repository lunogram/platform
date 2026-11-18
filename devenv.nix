{ pkgs, lib, config, ... }:
let
  # devenv moves a service to the next free port when its default is taken, so
  # the connection settings follow the allocated ports instead of the defaults
  # in etc/lunogram.dev.yaml. They are exported to the processes only: in the
  # shell they would override the configuration the Go tests expect.
  port = process: name: toString config.processes.${process}.ports.${name}.value;
  postgres = database: "postgres://postgres:postgrespw@127.0.0.1:${port "postgres" "main"}/${database}?sslmode=disable";
  exports = vars: lib.concatLines (lib.mapAttrsToList (name: value: "export ${name}=${lib.escapeShellArg value}") vars);
in
{
  languages.go.enable = true;
  languages.javascript = {
    enable = true;
    package = pkgs.nodejs_24;
    # CI's PNPM_VERSION major. nixpkgs' default pnpm 12 fails on the
    # onlyBuiltDependencies in console/pnpm-workspace.yaml (ERR_PNPM_IGNORED_BUILDS).
    pnpm = {
      enable = true;
      package = pkgs.pnpm_10;
    };
  };

  packages = [ pkgs.tinygo ];

  services.postgres = {
    enable = true;
    listen_addresses = "127.0.0.1";
    # The databases and the postgres role come from docker/postgres/init.sql,
    # which runs on every start, not from initdb.
    createDatabase = false;
  };
  services.redis.enable = true;
  services.nats = {
    enable = true;
    jetstream.enable = true;
  };
  services.mailpit.enable = true;

  tasks = {
    # Creates whatever databases a pull added; a no-op otherwise.
    "lunogram:db" = {
      exec = ''psql --dbname postgres -v ON_ERROR_STOP=1 -f "$DEVENV_ROOT/docker/postgres/init.sql"'';
      after = [ "devenv:processes:postgres" ];
    };
    "console:install" = {
      # Tasks have no TTY to confirm replacing a node_modules another pnpm made.
      exec = "pnpm install --frozen-lockfile --config.confirmModulesPurge=false";
      cwd = "${config.devenv.root}/console";
      execIfModified = [ "console/package.json" "console/pnpm-lock.yaml" ];
      before = [ "devenv:enterShell" ];
    };
  };

  processes = {
    lunogram = {
      exec = ''
        ${exports {
          CONFIG_FILE = "${config.devenv.root}/etc/lunogram.dev.yaml";
          POSTGRES_MANAGEMENT_URI = postgres "management";
          POSTGRES_SUBJECTS_URI = postgres "subjects";
          POSTGRES_JOURNEY_URI = postgres "journey";
          RBAC_POSTGRES_URI = postgres "rbac";
          REDIS_ADDRESS = "redis://127.0.0.1:${port "redis" "main"}";
          NATS_URL = "nats://127.0.0.1:${port "nats" "main"}";
          MAIL_SMTP_PORT = port "mailpit" "smtp";
        }}
        exec go run ./cmd/lunogram
      '';
      after = [
        "lunogram:db"
        "devenv:processes:redis"
        "devenv:processes:nats"
      ];
    };
    console = {
      exec = ''
        export VITE_PROXY_URL=http://localhost:8080
        exec pnpm dev
      '';
      cwd = "${config.devenv.root}/console";
    };
  };
}
