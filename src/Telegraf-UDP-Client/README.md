# Telegraf UDP JSON Client Driver

This client driver listen on a UDP port for JSON formatted data coming from Telegraf sources.

Telegraf is a incredibly powerful tool that can collect data from diverse sources like application metrics (MongoDB, Nginx, etc.) and protocols like SNMP, OPC-UA, MQTT and Modbus.

Telegraf works in **monitoring direction only**, i.e. **this driver can not send controls**.

Telegraf must be configured for UDP output with JSON format (socket_writer output).

    [[outputs.socket_writer]]
    address = "udp://127.0.0.1:51920"
    data_format = "json"

See Telegraf documentation for more information.
https://github.com/influxdata/telegraf

