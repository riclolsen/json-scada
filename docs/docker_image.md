# Docker Image

This is an all-in-one docker image for JSON-SCADA based on Ubuntu 24.04.

This is a full executable docker demo for the system. It can be reconfigured and customized for your needs.

It includes

- MongoDB Community as the core database server.
- PostgreSQL/TimescaleDB for time series historian.
- Grafana for dashboards.
- IEC 60870-5-104 Client that connects to the online demo for data acquisition.
- IEC 60870-5-104 Server listening on the localhost.
- DNP3 Server.
- Modbus client and server (Go).
- OPC UA Server.
- OPC UA Client.
- MQTT Sparkplug B.
- Metabase.
- And more.

## Run Instructions

Be sure to have the ports free. The ports are: 80, 9000, 4840, 2404 and 20000. Use sudo if needed.

    sudo docker pull ricardolo/json-scada:latest
    sudo docker run -p 80:80 -p 9000:9000 -p 4840:4840 -p 2404:2404 -p 20000:20000 -d --name=json_scada ricardolo/json-scada:latest
    # open http://127.0.0.1:80 on a browser

Or with Podman

    podman pull docker.io/ricardolo/json-scada:latest
    # redirect port 80 to an upper port as podman doesn't have root access by default
    podman run -p 8090:80 -p 9000:9000 -p 4840:4840 -p 2404:2404 -p 20000:20000 -d --name=json_scada docker.io/ricardolo/json-scada:latest
    # open http://127.0.0.1:8090 on a browser

## Access Instructions

Wait until images are pulled, the databases are seeded and the protocol communication begins.

Open http://127.0.0.1 on a browser (like Chrome, Safari or Firefox).

Login credentials are user="admin" and password="jsonscada".

To manage processes, access the Supervisor web interface at http://127.0.0.1:9000 (same credentials as above).

# Image build instructions

Clone the whole repository on the host computer.

    git clone --recurse-submodules https://github.com/riclolsen/json-scada --config core.autocrlf=input
    
Go to the json-scada folder to create the image.

    cd json-scada
    sudo docker build --pull --no-cache -t json-scada:latest -f Dockerfile .

