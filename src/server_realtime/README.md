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

Inspired by the OPC-UA material below:

* https://prototyping.opcfoundation.org
* https://www.youtube.com/watch?v=fiuamY0DzLM
* https://reference.opcfoundation.org
* https://github.com/OPCFoundation/UA-.NETStandard/tree/demo/webapi/SampleApplications/Workshop/Reference

### Read Service

This service is used to read realtime values from the MongoDB server. It is necessary to specify the nodes (data points) to be read by the numeric __id_ key or by the _tag_ string key.

Reference documentation https://reference.opcfoundation.org/Core/docs/Part4/5.10.2/.

Read Request: call the Post method for the /Invoke access point with the following JSON structure:

    {
        "ServiceId": 629,
        "Body": {
            "RequestHeader": {
            "Timestamp": "2020-09-17T12:40:47.373Z",
            "RequestHandle": 54027318,
            "TimeoutHint": 1500,
            "ReturnDiagnostics": 2,
            "AuthenticationToken": null
            },
            "MaxAge": 0,
            "TimestampsToReturn": 2,
            "NodesToRead": [
            {
                "NodeId": { "IdType": 0, "Id": 6620, "Namespace": 2 },
                "AttributeId": 13
            },
            {
                "NodeId": { "IdType": 1, "Id": "KAW2AL-27XCBR5231", "Namespace": 2 },
                "AttributeId": 13
            }
            ]
        }
    }

Response example:

    {
        "NamespaceUris": [
            "urn:opcf-apps-01:UA:Quickstarts:ReferenceServer",
            "http://opcfoundation.org/Quickstarts/ReferenceApplications",
            "http://opcfoundation.org/UA/Diagnostics"
        ],
        "ServerUris": [],
        "ServiceId": 632,
        "Body": {
            "ResponseHeader": {
            "RequestHandle": 54027318,
            "Timestamp": "2020-09-17T12:40:48.305Z",
            "ServiceDiagnostics": { "LocalizedText": 0 },
            "StringTable": [
                "Good",
                "The operation completed successfully.",
                "Query time: 12 ms"
            ],
            "ServiceResult": 0
            },
            "Results": [
            {
                "StatusCode": 0,
                "NodeId": { "IdType": 1, "Id": "KAW2BC1--RBLK", "Namespace": 2 },
                "Value": { "Type": 1, "Body": false, "Quality": 0 },
                "_Properties": {
                    "_id": 6620,
                    "valueString": "NORMAL",
                    "alarmed": true,
                    "annotation": "",
                    "origin": "supervised"
                },
                "SourceTimestamp": "2020-09-17T12:40:36.168Z"
            },
            {
                "StatusCode": 0,
                "NodeId": { "IdType": 1, "Id": "KAW2AL-27XCBR5231", "Namespace": 2 },
                "Value": { "Type": 1, "Body": true, "Quality": 0 },
                "_Properties": {
                    "_id": 3279,
                    "valueString": "ON",
                    "alarmed": false,
                    "annotation": "",
                    "origin": "supervised"
                },
                "SourceTimestamp": "2020-09-14T17:51:12.547Z"
            }
            ]
        }
    }

The non OPC-UA standard __Properties_ object from the response has extended attributes for the point. 

### Write Service

This service is used to send commands from the web UI to the server. 

Reference documentation: https://reference.opcfoundation.org/v104/Core/docs/Part4/5.10.4/.

Write Request: call the Post method for the /Invoke access point with the following JSON structure:

    {
        "ServiceId": 671,
        "Body": {
        "RequestHeader": {
            "Timestamp": "2020-09-17T13:35:30.186Z",
            "RequestHandle": 94046531,
            "TimeoutHint": 1500,
            "ReturnDiagnostics": 2,
            "AuthenticationToken": null
        },
        "NodesToWrite": [
            {
            "NodeId": { "IdType": 0, "Id": 64083, "Namespace": 2 },
            "AttributeId": 13,
            "Value": { "Type": 11, "Body": 1 }
            }
        ]
    }


Response:

    {
        "NamespaceUris": [
            "urn:opcf-apps-01:UA:Quickstarts:ReferenceServer",
            "http://opcfoundation.org/Quickstarts/ReferenceApplications",
            "http://opcfoundation.org/UA/Diagnostics"
        ],
        "ServerUris": [],
        "ServiceId": 674,
        "Body": {
            "ResponseHeader": {
            "RequestHandle": 94046531,
            "Timestamp": "2020-09-17T13:35:31.115Z",
            "ServiceDiagnostics": { "LocalizedText": 0 },
            "StringTable": [],
            "ServiceResult": 0
            },
        "Results": [0],
        "_CommandHandles": ["5f6366233fabf37f071097e9"]
        }
    }

Next, use read requests (with AttributeId: 12) to monitor command feedback, using the __CommandHandles_ value.

    {
        "ServiceId": 629,
        "Body": {
        "RequestHeader": {
            "Timestamp": "2020-09-17T13:35:30.396Z",
            "RequestHandle": 78655446,
            "TimeoutHint": 1250,
            "ReturnDiagnostics": 2,
            "AuthenticationToken": null
        },
        "MaxAge": 0,
        "NodesToRead": [
            {
            "NodeId": { "IdType": 0, "Id": 64083, "Namespace": 2 },
            "AttributeId": 12,
            "ClientHandle": "5f6366233fabf37f071097e9"
            }
        ]
        }
    }

Response as defined in https://reference.opcfoundation.org/Core/docs/Part4/7.20.2/

    {
        "NamespaceUris": [
            "urn:opcf-apps-01:UA:Quickstarts:ReferenceServer",
            "http://opcfoundation.org/Quickstarts/ReferenceApplications",
            "http://opcfoundation.org/UA/Diagnostics"
        ],
        "ServerUris": [],
        "ServiceId": 809,
        "Body": {
            "ResponseHeader": {
            "RequestHandle": 78655446,
            "Timestamp": "2020-09-17T13:35:31.325Z",
            "ServiceDiagnostics": { "LocalizedText": 0 },
            "StringTable": [],
            "ServiceResult": 0
            },
            "MonitoredItems": [
            {
                "ClientHandle": "5f6366233fabf37f071097e9",
                "Value": { "Value": 1, "StatusCode": 2159149056 },
                "NodeId": {
                "IdType": 1,
                "Id": "KAW2TR2-0XCBR5206----K",
                "Namespace": 2
                }
            }
            ]
        }
    }

When the StatusCode for the command is 0 (Good) the command was acknowledged ok.

### Read History Service Request
### Request Unique Attributes Value

## Environment Variables

* _**JS_IP_BIND**_ [String] - IP address for server to listen. Use "0.0.0.0" to listen on all interfaces. **Default="localhost" (local host only)**.
* _**JS_HTTP_PORT**_ [Integer] - HTTP Port for server listening. **Default=8080**.
* _**JS_GRAFANA_SERVER**_ [Integer] - HTTP URL to the Grafana server (for reverse proxy on /grafana). **Default="http://127.0.0.1:3000"**.
* _**JS_CONFIG_FILE**_ [String] - JSON SCADA config file name. **Default="../../conf/json-scada.json"**.

## Command line arguments

This process has no command line arguments.
