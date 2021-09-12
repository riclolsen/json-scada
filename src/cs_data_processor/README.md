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

## Process Command Line Arguments And Environment Variables

This driver has the following command line arguments and equivalent environment variables.

* _**1st arg. - Instance Number**_ [Integer] - Instance number to be executed. **Optional argument, default=1**. Env. variable: **JS_CSDATAPROC_INSTANCE**.
* _**2nd arg. - Log. Level**_ [Integer] - Log level (0=minimum,1=basic,2=detailed,3=debug). **Optional argument, default=1**. Env. variable: **JS_CSDATAPROC_LOGLEVEL**.
* _**3rd arg. - Config File Path/Name**_ [String] - Path/name of the JSON-SCADA config file. **Optional argument, default="../conf/json-scada.json"**. Env. variable: **JS_CONFIG_FILE**.

Env. variable: **JS_CSDATAPROC_DIVIDE_EXP** [String] - A JSON expression to divide change stream processing, will be part of the pipeline for the change stream. E.g: JS_CSDATAPROC_DIVIDE_EXP={ "fullDocument._id": { "$mod": [ 2, 1 ] } } or JS_CSDATAPROC_DIVIDE_EXP={"fullDocument.type":"analog"}. Use this to break change stream processing in multiple processes to reduce peak latency. This method can be paired with sharding for best results.
Env. variable: **JS_CSDATAPROC_READ_FROM_SECONDARY** [String] - Use "TRUE" to change the preferred read to a secondary MongoDB server. By default all read operations are directed to the primary server.

Command line args take precedence over environment variables.

## Process Instance Collection

A _processInstance_ entry will be created with defaults if one is not found. It can be used to configure some parameters and limit nodes allowed to run instances.

See also 

* [Schema Documentation](../../docs/schema.md) 
