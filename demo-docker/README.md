# Docker Demo

This is a full executable docker demo for the system.

It includes

- MongoDB Community as the core database server.
- PostgreSQL/TimescaleDB for time series historian.
- Grafana for dashboards.
- IEC 60870-5-104 Client that connects to the online demo for data acquisition.
- IEC 60870-5-104 Server listening on the localhost (port 2404).
- OPC-UA Client connected to public demo servers, and OPC-UA Server (port 4840).
- DNP3 Server (port 20000) and a DNP3 Client polling it.
- IEC 61850 MMS Server (port 102) and ICCP (TASE.2) Server (docker network only).
- MQTT Sparkplug B client, Telegraf listener, n8n integration.
- Calculations processor.
- Change stream realtime data processor.
- A Node/Express webserver app for user interface.
- Role based access control and admin management UI.

To run this demo, a docker runtime is needed with the docker compose command available. Git is also needed to extract the repository.

It can run on any Linux x64 or Windows 10 x64 (use Docker/WSL2 on Windows 10 version 2004 for best performance on this platform).

Clone the whole repository on the host computer.

    git clone https://github.com/riclolsen/json-scada.git  --config core.autocrlf=input

Go to the compile-docker folder to create the binaries and install the Node.js modules.

    cd json-scada/compile-docker
    docker compose up

Wait until the compilation process finishes. The protocol drivers and the calculations
process are Go static binaries published to `demo-docker/bin`, run on a plain Alpine image.

Go to the demo-docker folder and create the environment file. This generates a
random JWT signing secret that is unique to your installation.

    cd ../demo-docker
    ./generate-env.sh

On Windows, use PowerShell instead.

    .\generate-env.ps1

Then run the system.

    docker compose -f docker-compose.yaml up

The stack refuses to start while `JS_JWT_SECRET` is undefined. See the
[Security Notes](#security-notes) below.

## Access Instructions

Wait until images are pulled, the databases are seeded and the protocol communication begins.

Open http://127.0.0.1/login/login.html on a browser (Chrome, Safari or Firefox).

Login credentials are user="admin" and password="jsonscada".

This demo will connect to the online demo using IEC60870-5-104.

Online demo: http://vmi233205.contaboserver.net:8080/.

When the system docker demo and the online demo are connected, both systems will show the same data and will reflect identically the results of commands.

The MongoDB and PostgreSQL are configured for unauthenticated access, default server ports are exported to the main host, bound to `127.0.0.1` (this can be changed in the _.env_ and _docker-compose.yaml_ files, see the Security Notes below).

The docker demo also provides an IEC60870-5-104 server port (127.0.0.1:2404, originator address 1) that you can connect to using some IEC60870-5-104 client tool.

Suggestions of free IEC60870-5-104 clients

- https://github.com/riclolsen/qtester104
- http://the-vinci.com/vinci-software.
- https://www.mz-automation.de/communication-protocols/iec-60870-5-104-test-tool/

It also provides a DNP3 server port (127.0.0.1:20000, local address 2, remote address 1) and an
IEC 61850 MMS server port (127.0.0.1:102).

## Security Notes

This demo is meant to run on a trusted machine. Two settings in the `.env` file
control how exposed it is.

- **`JS_JWT_SECRET`** — the key used to sign the web server session tokens. It
  has no default: `generate-env.sh` / `generate-env.ps1` create a random one,
  and Docker Compose stops with an error if the variable is empty. Never reuse
  a secret published anywhere (including the placeholder in
  `src/server_realtime_auth/app/config/auth.config.js`) and never commit your
  `.env`: whoever knows the secret can mint a valid admin token and thereby
  issue commands to field devices, read server files and restart processes,
  without ever knowing a password. To roll the secret over, run
  `./generate-env.sh --force` (or `.\generate-env.ps1 -Force`) and recreate the
  containers; everyone will have to log in again.
- **`JS_BIND_ADDR`** — the host address the published ports are bound to,
  `127.0.0.1` by default, so the demo is reachable only from the local machine.

The MongoDB and PostgreSQL are configured for unauthenticated access, and the
IEC 60870-5-104, DNP3, IEC 61850 and OPC-UA server ports accept any peer. Publishing them
on a reachable interface (`JS_BIND_ADDR=0.0.0.0`) hands the whole database and
the control of the simulated field devices to anyone who can reach the host.
For an installation that is not a local demo, put the web server behind TLS,
enable authentication on both databases and restrict the protocol ports.
