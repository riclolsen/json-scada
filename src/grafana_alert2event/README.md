# {json:scada} grafana_alert2event.js

Grafana webhook listener that converts Grafana alert notifications to JSON-SCADA SOE events, beep and alarms.

It accepts the Grafana Alerting webhook payload (Grafana 8+ unified alerting, current API). The legacy (pre-Grafana 8) webhook notification payload is still accepted for backward compatibility.

Requires Node.js.

## Run

    cd src/grafana_alert2event
    npm install
    node grafana_alert2event.js [config file]

The process reads `../../conf/json-scada.json` by default; pass a path as the
first argument or set `JS_CONFIG_FILE`.

## Environment variables

- _**JS_ALERT2EVENT_IP_BIND**_ - IP bind address, default="127.0.0.1". Use "0.0.0.0" to listen on all interfaces.
- _**JS_ALERT2EVENT_HTTP_PORT**_ - TCP/IP Port for listening. Default="51910".
- _**JS_ALERT2EVENT_USERNAME**_ - Username for HTTP Basic Authentication. Default="grafana".
- _**JS_ALERT2EVENT_PASSWORD**_ - Password for HTTP Basic Authentication. Default="grafana".
- _**JS_ALERT2EVENT_ALERTING_MSG**_ - Alerting message. Default="alerting".
- _**JS_ALERT2EVENT_OK_MSG**_ - Ok (not alerting) message. Default="ok".

## Grafana Contact Point (webhook)

In Grafana, create a contact point with the _Webhook_ integration:

1. Navigate to **Alerting → Contact points**.
2. Click **+ Add contact point** and give it a name (e.g. "JSON-SCADA Events").
3. Select **Webhook** as the integration.
4. Set the **URL** to something like

   - http://localhost:51910/grafana_alert2event

5. In **Optional settings**, set the **HTTP Method** to **POST** and fill in **Basic Authentication Username** and **Basic Authentication Password** matching the credentials defined with the environment variables for this service.
6. Save the contact point.

The option **_Disable resolved message_** should be kept unset, so that events are also produced when alerts return to normal.

Then route alerts to this contact point using the **Notification policies** (or select the contact point directly in the alert rule). To get periodic reminders while an alert keeps firing, adjust the **Repeat interval** of the notification policy.

## Grafana Alert Rule Config

Create alert rules in **Alerting → Alert rules** (or from a dashboard panel via the **Alert** tab). Base the rule query on the JSON-SCADA PostgreSQL/TimescaleDB data source. When the rule fires (or resolves), notifications are sent to the webhook contact point and converted to JSON-SCADA events/alarms.

The **Summary** annotation text (fallback: **Description** annotation) will appear in the Events Viewer on the _Description_ column.

The following custom **labels** can be defined in the alert rule (Grafana Alerting labels replace the tags of the legacy alerting system):

- _**tag**_ - Tag name of an existing measurement or a new tag name for the alert. Default=alert rule name (label _alertname_).
- _**priority**_ - Priority number (0=highest). Default='3'.
- _**group1**_ - Name of an existing group or a new group for the alert. Default='Grafana'.
- _**event**_ - Convert alert to SOE event if not equal to '0'. Default='1'.
- _**alarm**_ - Convert alert to alarm/beep if not equal to '0'. Default='1'.
- _**alertingText**_ - Text to be presented in Events Viewer (on _Event_ column) when alerting (firing). Default='alerting'.
- _**okText**_ - Text to be presented in Events Viewer (on _Event_ column) when not alerting (resolved). Default='ok'.

Each alert instance in a grouped notification is converted individually. The _startsAt_/_endsAt_ timestamps of the alert are used as the source timestamp of the event.

## See also

- https://grafana.com/docs/grafana/latest/alerting/configure-notifications/manage-contact-points/integrations/webhook-notifier/
- https://grafana.com/docs/grafana/latest/alerting/alerting-rules/
