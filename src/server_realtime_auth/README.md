## Realtime HTTP/JSON Data Server (with user authentication/RBAC/JWT)

This Node.js/Express module serves JSON-SCADA realtime data, the Admin UI API
(`/Invoke`), and the GraphQL endpoint for the web-based interface.

It can also serve the HTML files from the `src/AdminUI/dist` folder.

The server reads `../../conf/json-scada.json` (override with `JS_CONFIG_FILE`)
and listens on `127.0.0.1:8080` by default. Set `JS_IP_BIND` and `JS_HTTP_PORT`
to change the bind address and port.

    cd src/server_realtime_auth
    npm install
    node index.js

It is possible to route access Grafana on "/grafana" path by adjusting the _JS_GRAFANA_SERVER_ environment variable.

It is recommended to apply a reverse proxy (Nginx) on top of this service to serve securely (https) to clients on external networks. For best scalability static files should be served directly via Nginx or Apache, and redirecting _/Invoke_ (API calls) to this service.

This module also provides user authentication and role-based access control (RBAC) using JWT tokens and optional LDAP authentication.

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
        root /home/username/json-scada/src/AdminUI/dist;
    }

## Data Services API

Access point : /Invoke/

Inspired by the OPC-UA material below:

- https://prototyping.opcfoundation.org
- https://www.youtube.com/watch?v=fiuamY0DzLM
- https://reference.opcfoundation.org
- https://github.com/OPCFoundation/UA-.NETStandard/tree/demo/webapi/SampleApplications/Workshop/Reference

### Read Service

This service is used to read realtime values from the MongoDB server. It is necessary to specify the nodes (data points) to be read by the numeric \__id_ key or by the _tag_ string key.

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

The non OPC-UA standard \__Properties_ object from the response has extended attributes for the point.

### Write Service

This service is used to send commands from the web UI to the server.

Reference documentation: https://reference.opcfoundation.org/Core/docs/Part4/5.10.4/.

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

Next, use read requests (with AttributeId: 12) to monitor command feedback, using the \__CommandHandles_ value.

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

## GraphQL API

Access point: /apollo

An Apollo GraphQL server providing functionality similar to the OPC-like /Invoke API, with a typed, introspectable schema. Authentication uses the same JWT token from the /Invoke/auth/signin service, sent either as the _x-access-token_ HTTP header or cookie. User rights (RBAC) are enforced exactly like in the /Invoke API: group1 (station) access restriction lists, command rights per station, and per-property write rights.

The schema is self-documenting: use introspection or point any GraphQL client (Apollo Sandbox, Insomnia, Postman, etc.) to the /apollo endpoint.

### Queries

- _**serverInfo**_ - Server name/version and database connectivity status.
- _**me**_ - Authenticated username and combined role rights.
- _**tags(filter, limit, skip, sortBy, sortDesc)**_ - Flexible realtime points query. Filter by tag list, point key list, group1/group2/group3, type, origin, alarmed, invalid, frozen, substituted and more; substring search on tag name and description; pagination and sorting.
- _**tagsCount(filter)**_ - Count of points matching a filter.
- _**tag(tag) / tagById(id)**_ - Single point lookup by tag name or numeric point key.
- _**groups1 / groups2(group1)**_ - Distinct station/bay names with point counts.
- _**activeAlarms(group1, group2, limit)**_ - Currently alarmed or out-of-normal points (like the persistentAlarms filter of the /Invoke API).
- _**soeEvents(filter, limit, ascending)**_ - Sequence of Events query with time range, station list, priority and tag filters; optional per-tag aggregation with counts; optional filtering/sorting by source timestamp.
- _**historicalData(tags, timeBegin, timeEnd, limit)**_ - Historical values from the PostgreSQL/TimescaleDB historian, restricted to points the user can access.
- _**commandStatus(commandHandle)**_ - Track command acknowledgment (PENDING/ACK_OK/ACK_FAIL/CANCELLED).
- _**protocolDriverInstances / protocolConnections**_ - Protocol configuration (credentials and certificates are never exposed).
- _**users / roles / systemSettings / userActions(filter, limit, skip)**_ - Administrative queries (require admin rights). _userActions_ is the audit trail of user commands and changes.

### Mutations

- _**issueCommand(tagOrId, value, valueString)**_ - Issue a command (control). Requires the _sendCommands_ right and permission for the point's group1 (station). Returns a _commandHandle_ to track acknowledgment via the _commandStatus_ query. Command tags beginning with "$$" are queued directly without a point lookup. Commands are logged to the SOE list and to the user actions audit trail.
- _**ackEvents(action, tag, eventId)**_ - Acknowledge or remove SOE events (ACK_ONE_EVENT, ACK_POINT_EVENTS, ACK_ALL_EVENTS, REMOVE_ONE_EVENT, REMOVE_POINT_EVENTS, REMOVE_ALL_EVENTS). Requires the _ackEvents_ right.
- _**ackAlarms(action, tagOrId)**_ - Acknowledge alarms or silence beep (ACK_ONE_ALARM, ACK_ALL_ALARMS, SILENCE_BEEP). Requires the _ackAlarms_ right.
- _**updateTagProperties(tagOrId, properties)**_ - Update point annotation, notes, limits, alarm disabling, or substitute (manually enter) the value. Each property requires its respective user right (_enterAnnotations_, _enterNotes_, _enterLimits_, _disableAlarms_, _substituteValues_). Changes are audited and generate SOE events where applicable.

### Examples

Query realtime data (POST to /apollo with the x-access-token header/cookie):

    {
      tags(filter: { group1: "KAW2", type: "analog" }, limit: 100) {
        tag
        value
        valueString
        invalid
        alarmed
        timeTag
        description
        unit
      }
    }

Query events and history:

    {
      soeEvents(filter: { priorityLte: 3, timeBegin: "2026-07-18T00:00:00Z" }) {
        eventId
        tag
        eventText
        timeTagAtSource
        ack
      }
      historicalData(tags: ["KAW2KPR21------A"], timeBegin: "2026-07-18T00:00:00Z") {
        tag
        values { value invalid timeTag }
      }
    }

Issue a command and check its acknowledgment:

    mutation {
      issueCommand(tagOrId: "KAW2AL-21XCBR5238----KCmd", value: 1) {
        ok
        commandHandle
      }
    }

    {
      commandStatus(commandHandle: "68f0...") {
        status
        ackTimeTag
        cancelReason
      }
    }

Acknowledge alarms and update properties:

    mutation {
      ackAlarms(action: ACK_ALL_ALARMS) { ok matchedCount }
      updateTagProperties(
        tagOrId: "KAW2KPR21------A"
        properties: { annotation: "under maintenance", hiLimit: 90 }
      ) { ok }
    }

### Environment Variables

- _**JS_GRAPHQL_AP**_ [String] - Access point (path) where the GraphQL API is mounted. **Default="/apollo"**.

## File Services API

Access point : /getFile

This API can be used to retrieve files stored in MongoDB Gridfs. Files can be manipulated using the _mongofiles_ tool.

- https://docs.mongodb.com/database-tools/mongofiles/

The MQTT client driver can be used to subscribe and save files published to MQTT topics on a broker. In this case, the file name is the MQTT topic name.

Parameters:

- _**name**_ [String] - File name (for files saved by the MQTT driver, use the MQTT topic name). **Required parameter**
- _**bucket**_ [String] - Gridfs bucket name. Default is "fs". **Optional parameter**
- _**mime**_ [String] - Mime type for the HTTP header. If not specified, it will be guessed based on the file extension. **Optional parameter**
- _**refresh**_ [Integer] - Time interval in seconds to reload page automatically (using the refresh HTTP header). **Optional parameter**

Examples:

    /getFile?name=topicRoot/subtopic/document.pdf&bucket=fs
    /getFile?name=cameras/camera1/image.jpg&mime=image/jpg&refresh=5

## User Authentication / Authorization (RBAC)

When enabled, the user roles (Role Based Access Control - RBAC) and rights are configured in the collections _roles_ and _users_ in the MongoDB database.

### _roles_ Collection Schema

- _**name**_ [String] - role name.
- _**isAdmin**_ [Boolean] - right to create, alter and delete users and roles.
- _**changePassword**_ [Boolean] - right to change its own password.
- _**sendCommands**_ [Boolean] - right to send commands (controls) via protocol.
- _**enterAnnotations**_ [Boolean] - right to create blocking annotations.
- _**enterNotes**_ [Boolean] - right to create documental annotations.
- _**enterManuals**_ [Boolean] - right to alter manual values.
- _**enterLimits**_ [Boolean] - right to set limits for analog points.
- _**substituteValues**_ [Boolean] - right to substitute supervised values.
- _**ackEvents**_ [Boolean] - right to acknowledge and eliminate events.
- _**ackAlarms**_ [Boolean] - right to acknowledge and eliminate alarms.
- _**disableAlarms**_ [Boolean] - right to disable/enable alarms for points.
- _**group1List**_ [Array of String] - right to access only a limited set of data (from a group1 list). An empty list means no restrictions on groups.
- _**displayList**_ [Array of String] - right to access only a limited set of displays. An empty list means no restrictions on displays.
- _**maxSessionDays**_ [Double] - time in days for the maximum session period (after this, the JWT access token expires).

To each user can be attributed a set of roles. Each right in each user role are combined to be the less restrictive (except for arrays). The combination is a logical OR for booleans, maximum value for numbers and union for arrays. When arrays are combined, an empty array can be combined with a non-empty list leaving to a more restrictive result.

### Environment Variables

- _**JS_IP_BIND**_ [String] - IP address for server to listen. Use "0.0.0.0" to listen on all interfaces. **Default="localhost" (local host only)**.
- _**JS_HTTP_PORT**_ [Integer] - HTTP Port for server listening. **Default=8080**.
- _**JS_GRAFANA_SERVER**_ [Integer] - HTTP URL to the Grafana server (for reverse proxy on /grafana). **Default="http://127.0.0.1:3000"**.
- _**JS_CONFIG_FILE**_ [String] - JSON SCADA config file name. **Default="../../conf/json-scada.json"**.
- _**JS_JWT_SECRET**_ [String] - Signing key for the JWT session tokens. Must be unique per installation and kept private: whoever knows it can forge a valid admin token. **Default=value defined in ./app/config/auth.config.js, which is public (it is in the repository) and MUST be overridden in any real installation**. The docker demo generates one in demo-docker/.env (see demo-docker/generate-env.sh), the Windows services get one from platform-windows/create_services.bat and the supervisor setup from platform-ubuntu-2404/start_server_realtime_auth.sh.
- _**JS_SERVER_REALTIME_AUTH_READ_FROM_SECONDARY**_ [String] - Use "TRUE" to change the preferred read to a secondary MongoDB server. By default all read operations are directed to the primary server.

#### LDAP Authentication Configuration

LDAP can be configured by editing the file ./app/config/auth.config.js or by setting the following environment variables. The environment variables have precedence over the configuration file.

Notes on how the login name is used:

- The value substituted for `{{username}}` is escaped per RFC 4515 before it is placed in the search filter, and per RFC 4514 when it is used to build a DN, so a login name cannot alter the shape of the filter or of the DN. Every occurrence of the placeholder is replaced, so filters naming it more than once work as written.
- A login attempt with an empty password is rejected before any bind is issued. A simple bind carrying a zero-length password is an *unauthenticated* bind (RFC 4513 5.1.2) that many servers answer with success, which would otherwise let any existing account in without a password.
- A search filter that matches more than one entry is rejected instead of authenticating the first match, so an ambiguous filter cannot be used to log in as another user.
- Prefer an `ldaps://` URL. With a plain `ldap://` URL, credentials cross the network unprotected and the TLS options below are not applied.

- _**JS_LDAP_ENABLED**_ [Boolean] - Use "true" to enable LDAP authentication. **Default="false"**.
- _**JS_LDAP_URL**_ [String] - LDAP server URL. **E.g."ldap://localhost:389"**.
- _**JS_LDAP_BIND_DN**_ [String] - LDAP bind DN. **E.g."cn=read-only-admin,dc=example,dc=com"**.
- _**JS_LDAP_BIND_CREDENTIALS**_ [String] - LDAP bind password. **E.g."secret"**.
- _**JS_LDAP_SEARCH_BASE**_ [String] - LDAP search base for users. **E.g."dc=example,dc=com"**.
- _**JS_LDAP_SEARCH_FILTER**_ [String] - LDAP search filter. **E.g."(uid={{username}})" or "(|(sAMAccountName={{username}})(cn={{username}}))"**.
- _**JS_LDAP_ATTRIBUTES_USERNAME**_ [String] - LDAP attribute for username. **E.g."uid" or "sAMAccountName"**.
- _**JS_LDAP_ATTRIBUTES_EMAIL**_ [String] - LDAP attribute for email. **E.g."mail"**.
- _**JS_LDAP_ATTRIBUTES_DISPLAYNAME**_ [String] - LDAP attribute for display name. **E.g."cn"**.
- _**JS_LDAP_GROUP_SEARCH_BASE**_ [String] - LDAP group search base. **E.g."ou=JSON-SCADA,dc=ad,dc=gpfs,dc=net"**.
- _**JS_LDAP_GROUP_MAPPING**_ [String] - LDAP group mapping as a JSON object. **E.g.'{"ou=mathematicians,dc=example,dc=com":"admin","ou=scientists,dc=example,dc=com":"user"}'**.
- _**JS_LDAP_TLS_REJECT_UNAUTHORIZED**_ [Boolean] - LDAP TLS reject unauthorized. **Default="true"**.
- _**JS_LDAP_TLS_CA**_ [String] - LDAP TLS CA file location. **E.g."/etc/ssl/certs/ca-certificates.crt"**.
- _**JS_LDAP_TLS_CERT**_ [String] - LDAP TLS cert  file location. **E.g."/etc/ssl/certs/client-cert.pem"**.
- _**JS_LDAP_TLS_KEY**_ [String] - LDAP TLS key file location. **E.g."/etc/ssl/private/client-key.pem"**.
- _**JS_LDAP_TLS_PASSPHRASE**_ [String] - LDAP TLS passphrase. **E.g."secret"**.
- _**JS_LDAP_TLS_PFX**_ [String] - LDAP TLS PFX file location. **E.g."/etc/ssl/certs/client.pfx"**.
- _**JS_LDAP_TLS_CRL**_ [String] - LDAP TLS CRL file location. **E.g."/etc/ssl/certs/crl.pem"**.
- _**JS_LDAP_TLS_CIPHERS**_ [String] - LDAP TLS ciphers. **E.g."TLS_AES_128_GCM_SHA256"**.
- _**JS_LDAP_TLS_SECURE_PROTOCOL**_ [String] - LDAP TLS secure protocol. **E.g."TLSv1_2_method"**.
- _**JS_LDAP_TLS_MIN_VERSION**_ [String] - LDAP TLS min version. **E.g."TLSv1.2"**.
- _**JS_LDAP_TLS_MAX_VERSION**_ [String] - LDAP TLS max version. **E.g."TLSv1.3"**.

#### PostgreSQL Historian Environment Variables

For connection to the PostgreSQL historian, it is possible to use the standard _Libpq_ environment variables.

- https://www.postgresql.org/docs/current/libpq-envars.html

### Command line arguments

This server takes no command line arguments. User authentication/authorization is always enforced.

## Tool to create users and change password via command line

Use this tool to create new users or change passwords using the command line.

Usage:

    node updateUser.js username password [email] [config file name]

Username and password arguments are mandatory, email and config file are optional.

If the user is not found in the database it will be created a new user. This new user will not be able to login until an administrator assign him some role.
