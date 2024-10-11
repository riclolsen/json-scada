# To learn more about how to use Nix to configure your environment
# see: https://developers.google.com/idx/guides/customize-idx-env
{ pkgs, ... }: {
  # Which nixpkgs channel to use.
  channel = "stable-23.11"; # or "unstable"

  # Use https://search.nixos.org/packages to find packages
  packages = [   
    pkgs.postgresql_15_jit
    pkgs.postgresql15Packages.timescaledb
    pkgs.postgresql15Packages.timescaledb_toolkit
    pkgs.util-linux.bin
    pkgs.dotnet-sdk_8
    pkgs.vscode-extensions.ms-dotnettools.csharp
    pkgs.nuget-to-nix
    pkgs.icu
    pkgs.gcc
    pkgs.gnumake
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
  env = { };
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
          mkdir -p ~/mongodb/var/lib/mongo/ && 
          mkdir -p ~/mongodb/var/log/mongodb/ && 
          mongod -f ~/json-scada/platform-nix-idx/mongod.conf && 
          mongosh json_scada < ~/json-scada/mongo_seed/a_rs-init.js && 
          mongosh json_scada < ~/json-scada/mongo_seed/b_create-db.js &&
          mongoimport --db json_scada --collection protocolDriverInstances --type json --file ~/json-scada/demo-docker/mongo_seed/files/demo_instances.json &&
          mongoimport --db json_scada --collection protocolConnections --type json --file ~/json-scada/demo-docker/mongo_seed/files/demo_connections_linux.json &&
          mongoimport --db json_scada --collection realtimeData --type json --file ~/json-scada/demo-docker/mongo_seed/files/demo_data.json &&
          mongoimport --db json_scada --collection processInstances --type json --file ~/json-scada/demo-docker/mongo_seed/files/demo_process_instances.json &&
          mongoimport --db json_scada --collection users --type json --file ~/json-scada/demo-docker/mongo_seed/files/demo_users.json &&
          mongosh json_scada --eval \"db.realtimeData.updateMany({_id:{$gt:0}},{$set:{dbId:'demo'}})\" 
        ";
        init-postgresql = "
          mkdir -p ~/postgres &&
          initdb -D ~/postgres &&
          cp ~/json-scada/platform-nix-idx/postgresql.conf ~/postgres/postgresql.conf &&
          cp ~/json-scada/platform-nix-idx/pg_hba.conf ~/postgres/pg_hba.conf &&
          /usr/bin/pg_ctl -D /home/user/postgres start &&
          /usr/bin/createuser -h localhost -s postgres &&
          psql -U postgres -w -h localhost -f ~/json-scada/sql/create_tables.sql template1 &&
          psql -U postgres -w -h localhost -f ~/json-scada/sql/metabaseappdb.sql metabaseappdb &&
          psql -U postgres -w -h localhost -f ~/json-scada/sql/grafanaappdb.sql grafanaappdb
        ";
        build-jsonscada = "cd ~/json-scada/platform-linux && ./build.sh";
      };
      # Runs when the workspace is (re)started
      onStart = {
        # Example: start a background task to watch and re-build backend code
        # watch-backend = "npm run watch-backend";
        start-mongodb = "/usr/bin/mongod -f ~/json-scada/platform-nix-idx/mongod.conf";
        start-supervisor = "(supervisord -c ~/json-scada/platform-nix-idx/supervisord.conf &)";
      };
    };
  };
}
