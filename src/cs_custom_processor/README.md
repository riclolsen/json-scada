# {json:scada} cs\_custom\_processor

This process can be customized for special data processing on mongodb changes.
Can be used as a template for other developments using the **jsonscada** Typescript submodule.

Requires Node.js.

## Build and run

    cd src/cs_custom_processor
    npm install
    npm run build
    npm start                 # node dist/cs_custom_processor.js
    npm run dev               # run the TypeScript directly with tsx

Pass the usual instance, log level and config file arguments after `npm start --`
(or to `node dist/cs_custom_processor.js`); the defaults match the other
processes. `npm run clean` removes `dist/`.

## Customization of Processing

Custom processing can be

* CYCLIC - At regular adjustable intervals.
* BY EXCEPTION - By change on any mongodb collection (by exception).
* BY EXTERNAL SOURCE - By external events (requires nodejs coding, no example provided).

Check the *customized\_module.ts* file for examples of cyclic and by exception processing.
The *cs\_custom\_processor.ts* should not be edited, it provides MongoDB connection handling and redundancy control.

## Process Command Line Arguments And Environment Variables

This process has the following command line arguments and equivalent environment variables.

* ***1st arg. - Instance Number*** \[Integer] - Instance number to be executed. **Optional argument, default=1**. Env. variable: **JS\_CSCUSTOMPROC\_INSTANCE**.
* ***2nd arg. - Log. Level*** \[Integer] - Log level (0=minimum,1=basic,2=detailed,3=debug). **Optional argument, default=1**. Env. variable: **JS\_CSCUSTOMPROC\_LOGLEVEL**.
* ***3rd arg. - Config File Path/Name*** \[String] - Path/name of the JSON-SCADA config file. **Optional argument, default="../../conf/json-scada.json"**. Env. variable: **JS\_CONFIG\_FILE**.

Command line args take precedence over environment variables.

## Process Instance Collection

A *processInstance* entry will be created with defaults if one is not found. It can be used to configure some parameters, control redundancy, and limit nodes allowed to run instances.

See also

* [Schema Documentation](../../docs/schema.md)
