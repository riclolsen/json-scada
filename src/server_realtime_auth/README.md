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

### Alarm List Service

`POST /Invoke/auth/listAlarms` (signed-in user, token cookie) answers the alarm list of the Alarms Viewer as `{ "error": false, "alarms": [], "limited": false }` (`limited` true when the row limit was reached). Body: `mode` (`"realtime"` or `"historical"`), `timeBegin`, `timeEnd` (ISO dates, historical), `group1` (station list), `priorityMax`, `text`, `unackedOnly`, `limit`, with the same meaning and limits as the GraphQL _alarms_ query. Each alarm carries the _alarmsData_ fields, `_id` as a string.

To acknowledge one alarm, send the WriteRequest alarm acknowledgement (AckOneAlarm) of its point with `{ "_Properties": { "alarm_id": "<_id>" } }` as the written value properties; without `alarm_id` every unacknowledged alarm of the point is acknowledged. The point's _alarmed_ flag is cleared when it has no unacknowledged alarm left.

### Read History Service Request

At most 1000 NodesToRead; each tag is read once, and at most 200000 values are returned in all (a repeated node counts again).

### Request Unique Attributes Value

## GraphQL API

Access point: /apollo

An Apollo GraphQL server providing functionality similar to the OPC-like /Invoke API, with a typed, introspectable schema. Authentication uses the same JWT token from the /Invoke/auth/signin service, sent either as the _x-access-token_ HTTP header or cookie. User rights (RBAC) are enforced exactly like in the /Invoke API: group1 (station) access restriction lists, command rights per station, and per-property write rights.

One request may have at most 50 top-level fields (aliases and fragments included) and ask for at most 500000 rows in all (the limits of its list fields add up); a query nested deeper than 10 levels is refused.

The schema is self-documenting: use introspection or point any GraphQL client (Apollo Sandbox, Insomnia, Postman, etc.) to the /apollo endpoint.

### Queries

- _**serverInfo**_ - Server name/version and database connectivity status.
- _**me**_ - Authenticated username and combined role rights.
- _**tags(filter, limit, skip, sortBy, sortDesc)**_ - Flexible realtime points query. Filter by tag list, point key list, group1/group2/group3, type, origin, alarmed, invalid, frozen, substituted and more; substring search on tag name and description; pagination and sorting.
- _**tagsCount(filter)**_ - Count of points matching a filter.
- _**tag(tag) / tagById(id)**_ - Single point lookup by tag name or numeric point key.
- _**groups1(limit, skip) / groups2(group1, limit, skip)**_ - Distinct station/bay names with point counts.
- _**activeAlarms(group1, group2, limit)**_ - Currently alarmed or out-of-normal points (like the persistentAlarms filter of the /Invoke API).
- _**alarms(mode, timeBegin, timeEnd, group1List, priorityMax, text, unackedOnly, limit)**_ - The alarm list (_alarmsData_, one entry per alarm occurrence). Mode REALTIME (default): the alarms not acknowledged or still active, newest first (default 5000). Mode HISTORICAL: every alarm raised from _timeBegin_ to _timeEnd_ (default the last 24 h), oldest first (default 10000, at most 100000). _text_ is a case-insensitive substring of tag, description, alarm text or station (at most 200 characters). Restricted to the user's stations.
- _**soeEvents(filter, limit, ascending)**_ - Sequence of Events query with time range, station list, priority and tag filters; optional per-tag aggregation with counts; optional filtering/sorting by source timestamp.
- _**historicalData(tags, timeBegin, timeEnd, limit)**_ - Historical values from the PostgreSQL/TimescaleDB historian, restricted to points the user can access; _limit_ is the maximum number of values per tag (at most 50000, 500000 in one answer).
- _**commandStatus(commandHandle)**_ - Track command acknowledgment (PENDING/ACK_OK/ACK_FAIL/CANCELLED). A user sees its own commands; other commands only for points in a station (group1) it can see, special "$$" commands of others only when not restricted to some stations, admins all (others answer null, as for an unknown handle; the same applies to the /Invoke/ command acknowledgement read).
- _**protocolDriverInstances(limit, skip) / protocolConnections(limit, skip)**_ - Protocol configuration (credentials and certificates are never exposed).
- _**users(limit, skip) / roles(limit, skip) / systemSettings / userActions(filter, limit, skip)**_ - Administrative queries (require admin rights). Configuration and group lists default to 20000 entries. _userActions_ is the audit trail of user commands and changes.

### Mutations

- _**issueCommand(tagOrId, value, valueString)**_ - Issue a command (control). Requires the _sendCommands_ right and permission for the point's group1 (station). Returns a _commandHandle_ to track acknowledgment via the _commandStatus_ query. Command tags beginning with "$$" are queued directly without a point lookup. Commands are logged to the SOE list and to the user actions audit trail.
- _**ackEvents(action, tag, eventId)**_ - Acknowledge or remove SOE events (ACK_ONE_EVENT, ACK_POINT_EVENTS, ACK_ALL_EVENTS, REMOVE_ONE_EVENT, REMOVE_POINT_EVENTS, REMOVE_ALL_EVENTS). Requires the _ackEvents_ right.
- _**ackAlarms(action, tagOrId)**_ - Acknowledge alarms or silence beep (ACK_ONE_ALARM: every unacknowledged alarm of the point, ACK_ALL_ALARMS, SILENCE_BEEP). Acknowledges the _alarmsData_ entries and clears the point's _alarmed_ summary flag. Requires the _ackAlarms_ right.
- _**updateTagProperties(tagOrId, properties)**_ - Update point annotation, notes, limits, alarm disabling, or substitute (manually enter) the value. Each property requires its respective user right (_enterAnnotations_, _enterNotes_, _enterLimits_, _disableAlarms_, _substituteValues_). Annotations are at most 2000 characters, notes 10000, the hysteresis is not below zero (the same rules apply to /Invoke/ property writes, where a refused write answers _BadTypeMismatch_ and changes nothing). Changes are audited and generate SOE events where applicable.

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
- _**mime**_ [String] - Mime type for the HTTP header, accepted only for passive content (`image/png`, `image/jpeg`, `image/gif`, `image/webp`, `image/bmp`, `image/avif`, `video/*`, `audio/*`, `text/plain`, `text/csv`, `application/pdf`, `application/json`, `application/octet-stream`); otherwise, and when not specified, the type comes from the file extension. HTML, SVG, XML and script files are served with `Content-Security-Policy: sandbox` (they cannot use the session of this site). **Optional parameter**
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
- _**viewSecEvents**_ [Boolean] - right to open the Security Events viewer (`POST /Invoke/auth/listSecEvents`, the `secEvents` collection). Not implied by _isAdmin_.
- _**group1List**_ [Array of String] - right to access only a limited set of data (from a group1 list). An empty list means no restrictions on groups.
- _**displayList**_ [Array of String] - right to access only a limited set of displays. An empty list means no restrictions on displays.
- _**maxSessionDays**_ [Double] - time in days for the maximum session period (after this, the JWT access token expires).

To each user can be attributed a set of roles. Each right in each user role are combined to be the less restrictive (except for arrays). The combination is a logical OR for booleans, maximum value for numbers and union for arrays. When arrays are combined, an empty array can be combined with a non-empty list leaving to a more restrictive result.

### Environment Variables

- _**JS_IP_BIND**_ [String] - IP address for server to listen. Use "0.0.0.0" to listen on all interfaces. **Default="localhost" (local host only)**.
- _**JS_HTTP_PORT**_ [Integer] - HTTP Port for server listening. **Default=8080**.
- _**JS_GRAFANA_SERVER**_ [Integer] - HTTP URL to the Grafana server (for reverse proxy on /grafana). **Default="http://127.0.0.1:3000"**.
- _**JS_CONFIG_FILE**_ [String] - JSON SCADA config file name. **Default="../../conf/json-scada.json"**.
- _**JS_TRUSTED_PROXIES**_ [String] - Comma separated addresses of reverse proxies whose _X-Real-IP_ / _X-Forwarded-For_ headers are believed (client address for the sign-in throttling, the audit trail and the command originator). **Default="127.0.0.1,::1,::ffff:127.0.0.1"** (a proxy on this host). Exact addresses only (no ranges). A proxy on another host (or another container) must be listed, written as the server sees it (e.g. `::ffff:10.0.0.5` for an IPv4 peer on a dual-stack socket); otherwise every user appears to come from the proxy address and 5 failed sign-ins of anyone lock that user name for every client of the proxy. Never list an address that clients can connect from directly.
- _**JS_ALLOW_FRAMING**_ [Boolean] - Use "true" to let other sites show the pages of this server in frames. **Default="false"**: pages are sent with _X-Frame-Options: SAMEORIGIN_ (the proxied apps such as Grafana keep their own policy). All responses carry _X-Content-Type-Options: nosniff_ and _Referrer-Policy: same-origin_.
- _**JS_JWT_SECRET**_ [String] - Signing key for the JWT session tokens. Must be unique per installation and kept private: whoever knows it can forge a valid admin token. **When not set, a random secret is generated at each start (a warning is logged): sessions then end when the server restarts. The value in ./app/config/auth.config.js is public (it is in the repository) and is never used for signing.** The docker demo generates one in demo-docker/.env (see demo-docker/generate-env.sh), the Windows services get one from platform-windows/create_services.bat and the supervisor setup from platform-ubuntu/start_server_realtime_auth.sh.
- _**JS_SERVER_REALTIME_AUTH_READ_FROM_SECONDARY**_ [String] - Use "TRUE" to change the preferred read to a secondary MongoDB server. By default all read operations are directed to the primary server.

#### Access control notes

- The session token only identifies the user. Rights are read from the database (cached for 10 seconds, immediately refreshed after a change of users or roles made through this server, and always fresh for commands and admin routes): a deleted user is refused and a role change applies without waiting for the token to expire. A sign-out revokes the token (also a kept copy, see the _revokedTokens_ collection) and a password change or a new user name ends all the other sessions of the user. While the database is unavailable, recently cached rights are still used (up to 1 minute for commands and admin routes, 10 minutes otherwise), then requests are answered as unavailable; sign-outs made meanwhile are stored when the database is back. Sessions last the largest _maxSessionDays_ of the user's roles, at least one hour; LDAP users at most _JS_LDAP_MAX_SESSION_HOURS_ (the directory is consulted only at sign-in).
- Passwords set through this server (user creation/update, password change) need at least 8 characters. Hashes made with a lower bcrypt cost by older versions are upgraded at the next successful sign-in.
- No change through this server can leave the installation without a user holding an _isAdmin_ role (role update, role removal from a user, user deletion), and the `admin` user can be neither renamed nor deleted.
- The client cookie _json-scada-user_ carries the user's flags and _displayList_; when the list would not fit in a cookie it is left out (`rights.displayListTruncated: true`) and clients read it from `GET /Invoke/auth/userDisplays` (`{ "displayList": [] }`, an empty list means all displays).
- Tag names may not begin with a number (or a signed number): `createTag`/`updateTag` refuse them, because a point id that is a number or a digit string is read as a point key.
- The audit trail (_userActions_) never stores secrets: any field named like a password, passphrase, secret, token, credential or api key is replaced by `***`. Display saves are audited with name and size only, sign-outs only for a valid session.
- While MongoDB is unavailable, users whose rights are cached keep working; others get HTTP 503 with `unavailable: true` (the AdminUI keeps the session) or the OPC status _BadServerNotConnected_.
- Sign-in is throttled: 5 failed attempts of the same client address for the same user name lock that pair for 15 minutes, and 30 failed attempts of one client address (any user names) lock that address for 15 minutes (kept in memory). At most 8 password checks run at once, 2 per client address; more simultaneous sign-ins get HTTP 503 (try again).
- A local user (not created by LDAP) is checked against its stored password only, also when LDAP is enabled: its password is never sent to the directory, and an LDAP login can never take over a local account of the same name (it is refused). An LDAP user is checked by the directory only: a password stored for it is never accepted. Failures are audited as _signinFailed_ user actions and the answer does not tell whether the user exists. The client address is the connection's, unless the connection comes from a trusted proxy (_JS_TRUSTED_PROXIES_), whose _X-Real-IP_ / _X-Forwarded-For_ is then used.
- Users restricted to some stations (_group1List_) only get the rows of their points from _/queryJSON_, and the displays of their _displayList_ from _/svg_. Special `$$` commands need `$$` in _group1CommandList_ when the user is restricted by _group1List_ or _group1CommandList_.
- Commands (`/Invoke/` and GraphQL) are accepted only for points with origin `command`, in a station the user may command (its _group1CommandList_, or else its _group1List_; both when both are given), and never while the command point or its supervised point (_supervisedOfCommand_) is blocked by the operator (_commandBlocked_ true or a non-empty blocking _annotation_). A numeric command must carry a finite number. Command points, user rights, revoked tokens and token versions are always read from the MongoDB primary, also with _JS_SERVER_REALTIME_AUTH_READ_FROM_SECONDARY_.
- Uploads (project import, CIM-EMS model packages) are written to temporary files in the system temp folder (`json-scada-uploads`), not kept in memory, and removed when the request ends. A project import is refused when its _users.json_/_roles.json_ are missing or no user in it holds an admin role (the import replaces all users and roles).
- A user may have at most 20 CIM-EMS requests waiting for the driver (HTTP 429 beyond).
- Errors of admin routes are answered as short texts (invalid field, duplicate value, file error); the details go to the log. Log lines never start with request data: line breaks in a message continue on an indented line.
- GraphQL (_/apollo_): the token is checked before the body is read, queries nested deeper than 10 levels are refused (schema introspection, `__schema`/`__type`, has a limit of 30 of its own), and the protocol connection and driver instance queries are for admin users.
- The Node-RED editor (_/nodered_), n8n (_/n8n_), the log viewers (_/log-io_, _/supervisor_) and their websocket channels are for admin users only. The session token (_x-access-token_ cookie or header) is removed from every request forwarded to a proxied app (Grafana, Metabase, Node-RED, n8n, log viewers); they identify the user by _X-WebAuth-User_.
- The custom developments (_/custom-developments_) are for signed-in users; without a session the browser is sent to _/login_.
- The session cookies are _SameSite=Lax_ (and _Secure_ when the request came over https, also behind a proxy sending _X-Forwarded-Proto_, as the HTTPS nginx templates do: `proxy_set_header X-Forwarded-Proto $scheme;`). Creating, updating and deleting tags, connections and driver instances accept only POST.
- Request bodies are limited to 2 MB, except for the project import, CIM-EMS model upload/requests and display save routes (100 MB, token checked before the body is read).

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
- _**JS_LDAP_MAX_SESSION_HOURS**_ [Number] - Longest session of an LDAP user, in hours. The directory is consulted only at sign-in, so a disabled account or a changed group keeps working until its session ends. **Default=12**.

#### PostgreSQL Historian Environment Variables

For connection to the PostgreSQL historian, it is possible to use the standard _Libpq_ environment variables.

- https://www.postgresql.org/docs/current/libpq-envars.html

- _**JS_PG_CONNECT_TIMEOUT_MS**_ [Integer] - Time limit to open a historian connection. **Default=5000**.
- _**JS_PG_STATEMENT_TIMEOUT_MS**_ [Integer] - Time limit of a historian query (the client gives up 5 s later). **Default=30000**.

### Command line arguments

This server takes no command line arguments. User authentication/authorization is always enforced.

## Tool to create users and change password via command line

Use this tool to create new users or change passwords using the command line.

Usage:

    node updateUser.js username password [email] [config file name]

Username and password arguments are mandatory, email and config file are optional.

If the user is not found in the database it will be created a new user. This new user will not be able to login until an administrator assign him some role.
