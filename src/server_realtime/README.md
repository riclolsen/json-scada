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

## Data Services API

Access point : /Invoke/

Inspired by the OPC reference app https://github.com/OPCFoundation/UA-.NETStandard/tree/demo/webapi/SampleApplications/Workshop/Reference

Read Service Request
Write Service Request
Read History Service Request
Request Unique Attributes Value

## Environment Variables

* _**JS_IP_BIND**_ [String] - IP address for server to listen. Use "0.0.0.0" to listen on all interfaces. **Optional argument, default="localhost" (local host only)**.
* _**JS_HTTP_PORT**_ [Integer] - HTTP Port for server listening. **Optional argument, default=8080**.
* _**JS_GRAFANA_SERVER**_ [Integer] - HTTP URL to the Grafana server (for reverse proxy on /grafana). **Optional argument, default="http://127.0.0.1:3000"**.
* _**JS_CONFIG_FILE**_ [String] - JSON SCADA config file name. **Optional argument, default="../../conf/json-scada.json"**.

## Command line arguments

This process has no command line arguments.
