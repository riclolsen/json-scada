# {json:scada} grafana_alert2event.js

Grafana webhook notification channel listener that converts alerts to JSON-SCADA SOE events.

* https://grafana.com/docs/grafana/latest/alerting/notifications/

The following tags can be defined in the Grafana Alert|Tags list field.
* tag - Tag name of an existing measurement or a new tag name for the alert. Default="NO_TAG".
* priority - Priority number (0=highest). Default='3'.
* group1 - Name of an existing group or a new group for the alert. Default='Grafana'.
* event - Convert alert to SOE event if not equal to '0'. Default='1'.
* alarm - Convert alert to alarm if not equal to '0'. Default='1'.

## Environment variables

* JS_ALERT2EVENT_IP_BIND - IP bind address, default="127.0.0.1". Use "0.0.0.0" to listen on all interfaces.
* JS_ALERT2EVENT_HTTP_PORT - TCP/IP Port for listening. Default="51909".
* JS_ALERT2EVENT_USERNAME - Username for basic Oauth credential validation. Default="grafana".
* JS_ALERT2EVENT_PASSWORD - Password for basic Oauth credential validation. Default="grafana".
* JS_ALERT2EVENT_ALERTING_MSG - Alerting message. Default="alerting".
* JS_ALERT2EVENT_OK_MSG - Ok (not alerting) message. Default="ok".

