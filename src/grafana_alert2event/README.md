# {json:scada} grafana_alert2event.js

Grafana webhook notification channel listener that converts Grafana alerts to JSON-SCADA SOE events, beep and alarms.

## Environment variables

- _**JS_ALERT2EVENT_IP_BIND**_ - IP bind address, default="127.0.0.1". Use "0.0.0.0" to listen on all interfaces.
- _**JS_ALERT2EVENT_HTTP_PORT**_ - TCP/IP Port for listening. Default="51909".
- _**JS_ALERT2EVENT_USERNAME**_ - Username for basic Oauth credential validation. Default="grafana".
- _**JS_ALERT2EVENT_PASSWORD**_ - Password for basic Oauth credential validation. Default="grafana".
- _**JS_ALERT2EVENT_ALERTING_MSG**_ - Alerting message. Default="alerting".
- _**JS_ALERT2EVENT_OK_MSG**_ - Ok (not alerting) message. Default="ok".

## Grafana Alert Channel

A Grafana alert channel of webhook type must be created with HTTP Method **_POST_** and a URL like

- http://localhost:51910/grafana_alert2event

The parameters **_Username_** and **_Password_** must match credentials defined with environment variables for this service.

The options **_Default (send on all alerts)_** and **_Send Reminders_** are recommended to be set. The option **_Disable Resolve Message_** should be kept unset.

## Grafana Panel Alert Config

Grafana alerts must be created in a _Panel_ added to some _Dashboard_. The **_Alert_** tab appears when the _Panel_ is edited.

Some rule must be setup based on the panel query that should be directed to the JSON-SCADA PostgreSQL/TimescaleDB data source. When the conditions adjusted are met (or normalized), alert notifications are sent.

Notifications must be send (**_Send to_** parameter) to the _webhook_ alert channel previously created.

The **_Data and Error Handling_** options should be set both to **_Keep Last State_**.

The **_Message_** text will appear in Events Viewer converted on the _Description_ column.

The following tags can be defined in the Grafana Alert|Tags list field.

- _**tag**_ - Tag name of an existing measurement or a new tag name for the alert. Default="NO_TAG".
- _**priority**_ - Priority number (0=highest). Default='3'.
- _**group1**_ - Name of an existing group or a new group for the alert. Default='Grafana'.
- _**event**_ - Convert alert to SOE event if not equal to '0'. Default='1'.
- _**alarm**_ - Convert alert to alarm if not equal to '0'. Default='1'.
- _**alertingText**_ - Text to be presented in Events Viewer (on _Event_ column) when alerting. Default='alerting'.
- _**okText**_ - Text to be presented in Events Viewer (on _Event_ column) when no alerting (status ok). Default='ok'.

## See also

- https://grafana.com/docs/grafana/latest/alerting/notifications/
