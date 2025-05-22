# To learn more about how to use Nix to configure your environment
# see: https://developers.google.com/idx/guides/customize-idx-env
{ pkgs, ... }: {
  # Which nixpkgs channel to use.
  channel = "stable-23.11"; # or "unstable"

  # Use https://search.nixos.org/packages to find packages
  packages = [   
    pkgs.sudo
    pkgs.postgresql_17
    pkgs.postgresql17Packages.timescaledb
    pkgs.postgresql17Packages.timescaledb_toolkit
    pkgs.util-linux.bin
    pkgs.dotnet-sdk_8
    pkgs.vscode-extensions.ms-dotnettools.csharp
    pkgs.nuget-to-nix
    pkgs.icu
    pkgs.gcc
    pkgs.gnumake
    pkgs.cmake
    pkgs.go
    pkgs.python311
    pkgs.python311Packages.pip
    pkgs.python311Packages.supervisor
    pkgs.nodejs_20
    pkgs.openjdk_headless
    pkgs.grafana
    pkgs.telegraf
    pkgs.nginx
    pkgs.wineWowPackages.full
    pkgs.libpcap
    pkgs.mongodb
    pkgs.mongodb-tools
    pkgs.mongosh
    # pkgs.nodePackages.nodemon
  ];

  #services.postgres = {
  #  extensions = [
  #    "timescaledb"
  #    "timescaledb_toolkit"
  #    "pgvector"
  #  ];
  #  enable = true;
  #};
  services.docker.enable = true;

  # Sets environment variables in the workspace
  env = { 
    DOTNET_ROOT = "/nix/store/3frycb58lshfmnkjv1rvqqz1s7wyvvck-dotnet-sdk-8.0.101";
  };
  idx = {
    # Search for the extensions you want on https://open-vsx.org/ and use "publisher.id"
    extensions = [
      # "vscodevim.vim"
      "esbenp.prettier-vscode"
    ];

    # Enable previews
    previews = {
      enable = true;
      previews = {
        # web = {
        #   # Example: run "npm run dev" with PORT set to IDX's defined port for previews,
        #   # and show it in IDX's web preview panel
        #   command = ["npm" "run" "dev"];
        #   manager = "web";
        #   env = {
        #     # Environment variables to set for your server
        #     PORT = "$PORT";
        #   };
        # };
      };
    };

    # Workspace lifecycle hooks
    workspace = {
      # Runs when a workspace is first created
      onCreate = {
        init-mongodb = "
          rm -rf ~/.emu/avd
          rm -rf ~/.androidsdkroot/* &&
          mkdir -p ~/mongodb/var/lib/mongo/ && 
          mkdir -p ~/mongodb/var/log/mongodb/ && 
          mongod -f ~/json-scada/platform-nix-idx/mongod.conf && 
          mongosh json_scada < ~/json-scada/mongo_seed/a_rs-init.js && 
          mongosh json_scada < ~/json-scada/mongo_seed/b_create-db.js &&
          mongoimport --db json_scada --collection protocolDriverInstances --type json --file ~/json-scada/demo-docker/mongo_seed/files/demo_instances.json &&
          mongoimport --db json_scada --collection protocolConnections --type json --file ~/json-scada/platform-nix-idx/demo_connections.json &&
          mongoimport --db json_scada --collection protocolConnections --type json --file ~/json-scada/demo-docker/mongo_seed/files/demo_connections_linux.json &&
          mongoimport --db json_scada --collection realtimeData --type json --file ~/json-scada/demo-docker/mongo_seed/files/demo_data.json &&
          mongoimport --db json_scada --collection processInstances --type json --file ~/json-scada/demo-docker/mongo_seed/files/demo_process_instances.json &&
          mongoimport --db json_scada --collection roles --type json --file ~/json-scada/demo-docker/mongo_seed/files/demo_roles.json &&
          mongoimport --db json_scada --collection users --type json --file ~/json-scada/demo-docker/mongo_seed/files/demo_users.json
        ";
        init-postgresql = "
          mkdir -p ~/json-scada/grafana/data &&
          mkdir -p ~/json-scada/grafana/logs &&
          mkdir -p ~/json-scada/grafana/plugins &&
          mkdir -p ~/json-scada/log &&
          mkdir -p ~/postgres &&
          initdb -D ~/postgres &&
          cp ~/json-scada/platform-nix-idx/postgresql.conf ~/postgres/postgresql.conf &&
          cp ~/json-scada/platform-nix-idx/pg_hba.conf ~/postgres/pg_hba.conf &&
          /usr/bin/pg_ctl -D ~/postgres start >/dev/null 2>&1 &&
          /usr/bin/createuser -h localhost -s postgres ;
          psql -U postgres -w -h localhost -f ~/json-scada/sql/create_tables.sql template1 &&
          psql -U postgres -w -h localhost -f ~/json-scada/sql/metabaseappdb.sql metabaseappdb &&
          psql -U postgres -w -h localhost -f ~/json-scada/sql/grafanaappdb.sql grafanaappdb
        ";
        build-jsonscada = "
          cd ~/json-scada/platform-nix-idx &&
          sh ./build.sh
        ";
      };
      # Runs when the workspace is (re)started
      onStart = {
        # Example: start a background task to watch and re-build backend code
        # watch-backend = "npm run watch-backend";
        start-mongodb = "/usr/bin/mongod -f ~/json-scada/platform-nix-idx/mongod.conf";
        start-postgresql = "/usr/bin/pg_ctl -D ~/postgres start >/dev/null 2>&1";
        start-grafana = "grafana server target --config ~/json-scada/platform-nix-idx/grafana.ini --homepath /nix/store/454jp6ww3nr2k7jxfp4il4a3l9kq0l3h-grafana-10.2.8/share/grafana/ >/dev/null 2>&1 &";
        start-supervisor = "supervisord -c ~/json-scada/platform-nix-idx/supervisord.conf";
      };
    };
  };
}
