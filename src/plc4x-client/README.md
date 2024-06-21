# {json:scada} plc4x-client.go

A generic PLC client driver for JSON-SCADA. Based on the Apache PLC4X/plc4go project.

    https://github.com/apache/plc4x

## Process Command Line Arguments And Environment Variables

This driver has the following command line arguments and equivalent environment variables.

* _**1st arg. - Instance Number**_ [Integer] - Instance number to be executed. **Optional argument, default=1**. Env. variable: **JS_CALCULATIONS_INSTANCE**.
* _**2nd arg. - Log. Level**_ [Integer] - Log level (0=minimum,1=basic,2=detailed,3=debug). **Optional argument, default=1**. Env. variable: **JS_CALCULATIONS_LOGLEVEL**.
* _**3th arg. - Config File Path/Name**_ [String] - Path/name of the JSON-SCADA config file. **Optional argument, default="../conf/json-scada.json"**. Env. variable: **JS_CONFIG_FILE**.

Command line args take precedence over environment variables.

## Process Instance Collection

A _processInstance_ entry will be created with defaults if one is not found. It can be used to configure some parameters and limit nodes allowed to run instances.

See also 

* [Schema Documentation](../../docs/schema.md) 

