## Realtime HTTP/JSON Data Server

This NodeJS/Express module can serve JSON-SCADA realtime data for the web-based interface.

It can also server the HTML files from the src/htdocs folder.

It is possible to access Grafana on "/grafana" path adjusting the _JS_GRAFANA_SERVER_ environment variable.

It is recommended to apply a reverse proxy (Nginx) on top of this service to serve securely to client on external networks. For best scalability static files should be served directly via Nginx or Apache, redirecting _/grafana_ to the Grafana server and _/Invoke_ to this Node.js service.

### Example Nginx config as a reverse proxy

    # data API
    location /Invoke/ {
        proxy_set_header   X-Forwarded-For $remote_addr;
        proxy_set_header   Host $http_host;
        proxy_pass         http://127.0.0.1:8080/Invoke/;
    }

    # Grafana server
    location /grafana/ {
        proxy_set_header   X-Forwarded-For $remote_addr;
        proxy_set_header   Host $http_host;
        proxy_pass         http://127.0.0.1:3000/;
    }

    # Supervisor web access, if desired
    location /supervisor/ {
        proxy_set_header   X-Forwarded-For $remote_addr;
        proxy_set_header   Host $http_host;
        proxy_pass         http://127.0.0.1:9000/;
    }

    # Static files
    location / {
        root /home/username/json-scada/src/htdocs;
    }

## Data Services API, File Services API

See https://github.com/riclolsen/json-scada/tree/master/src/server_realtime_auth#read-service.

## Environment Variables

- _**JS_IP_BIND**_ [String] - IP address for server to listen. Use "0.0.0.0" to listen on all interfaces. **Default="localhost" (local host only)**.
- _**JS_HTTP_PORT**_ [Integer] - HTTP Port for server listening. **Default=8080**.
- _**JS_GRAFANA_SERVER**_ [Integer] - HTTP URL to the Grafana server (for reverse proxy on /grafana). **Default="http://127.0.0.1:3000"**.
- _**JS_CONFIG_FILE**_ [String] - JSON SCADA config file name. **Default="../../conf/json-scada.json"**.
- _**JS_READ_FROM_SECONDARY**_ [String] - Use "TRUE" to change the preferred read to a secondary MongoDB server. By default all read operations are directed to the primary server.

For connection to the PostgreSQL historian, it is possible to use the standard _Libpq_ environment variables.

- https://www.postgresql.org/docs/current/libpq-envars.html

## Command line arguments

This process has no command line arguments.
