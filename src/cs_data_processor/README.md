# {json:scada} cs_data_processor.js

This process is responsible for watching updates on the "sourceDataUpdate" field object from documents on the "realtimeData" collection using a change stream. All data sources like client protocol drivers and the calculation engine are expected to update data to the "sourceDataUpdate" field. This schema keeps protocol drivers as simple as possible and standardize data processing across all sources in just one place.

Upcoming data triggers

* Calculation of linear adjustment factors (a.x+b) or digital inversion.
* Update of the main document "value", "valueString", "timeTag*", quality flags.
* Verification of digital (transition) and analog (limits violation) alarms, signalling on "alarmed" field.
* Update of the "soeData" collection when there is a source time tag for digital points.
* Data forward to PostgreSQL database historian.

This single process handles all upcoming realtime data and can be a bottleneck for the system. It is important to observe the latency and throughput on systems with high volume of upcoming data. Large delays on change stream processing may require a faster server or sharding.

The "timeTag" and "sourceDataUpdate.timeTag" timestamps can be subtracted to find the change stream latency.

Example pipeline to calculate latency (in ms)

    [{
        '$match': {
        'group1': 'a group1 name'
        }
     }, {
        '$set': {
        'diff': {
            '$subtract': [
            '$timeTag', '$sourceDataUpdate.timeTag'
            ]
        }
        }
     }, {
        '$sort': {
        'diff': -1
        }
     }]


Requires Node.js.