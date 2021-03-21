# {json:scada} cs_custom_processor.js

This process can be customized for special data processing on mongodb changes.

Requires Node.js.

## Customization of Processing

Custom processing can be 

* CYCLIC - At regular adjustable intervals.
* BY EXCEPTION - By change on any mongodb collection (by exception).
* BY EXTERNAL SOURCE - By external events (requires nodejs coding, no example provided).

Check the _customized_module.js_ file for examples of cyclic and by exception processing.
The _cs_custom_processor.js_ should not be edited, it provides MongoDB connection handling and redundancy control.

## Process Command Line Arguments And Environment Variables

This process has the following command line arguments and equivalent environment variables.

* _**1st arg. - Instance Number**_ [Integer] - Instance number to be executed. **Optional argument, default=1**. Env. variable: **JS_CSCUSTOMPROC_INSTANCE**.
* _**2nd arg. - Log. Level**_ [Integer] - Log level (0=minimum,1=basic,2=detailed,3=debug). **Optional argument, default=1**. Env. variable: **JS_CSCUSTOMPROC_LOGLEVEL**.
* _**3rd arg. - Config File Path/Name**_ [String] - Path/name of the JSON-SCADA config file. **Optional argument, default="../conf/json-scada.json"**. Env. variable: **JS_CONFIG_FILE**.

Command line args take precedence over environment variables.

## Process Instance Collection

A _processInstance_ entry will be created with defaults if one is not found. It can be used to configure some parameters and limit nodes allowed to run instances.

See also 

* [Schema Documentation](../../docs/schema.md) 
