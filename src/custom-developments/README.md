# Custom Developments Example Templates

Use the example templates to create your own custom developments.

1.  Duplicate the template folder and rename it to your own development name.
2.  Use an AI-enabled text editor edit the project files. We recommend using WindSurf, VS Code with Cline or Copilot extension, Cursor.
3.  Ask the AI assistant to help you write the code to accomplish your development goals.
4.  Import the APIs from the library where you need them.
    ```typescript
    import * as scadaOpcApi from '../lib/scadaOpcApi'  ```
5.  Mention the APIs in src/lib/scadaOpcApi.ts as the source of the data. Mention the tag names you want to retrieve. Mention the following API calls to retrieve data from the SCADA:

        ```typescript
        // Get the group1 (station names) list
        scadaOpcApi.getGroup1List(): Promise<string[]>

        // Get realtime data from tag names list
        scadaOpcApi.readRealTimeData(
            variables: string[]
        ): Promise<DataPoint[]>

        // Get realtime filtered data by group1, group2 and alarmed status
        scadaOpcApi.getRealtimeFilteredData(
            group1Filter: string,
            group2Filter: string,
            onlyAlarms: boolean
        ): Promise<DataPoint[]>

        // Get historical data for a tag
        scadaOpcApi.getHistoricalData(
            tag: string,
            timeBegin: Date,
            timeEnd: Date | null | undefined
        ): Promise<HistoricalData[]> 

        // Get Sequence of Events (SOE) data
        scadaOpcApi.getSoeData (
            group1Filter: string[],
            useSourceTime: boolean,
            aggregate: number,
            limit: number,
            timeBegin: any,
            timeEnd: any
        ): Promise<SoeData[]> 

        // Issue a command for a tag
        scadaOpcApi.issueCommand(
            commandTag: string,
            value: number
        ): Promise<string>
        ```

6. Build the project and run it.

    ```bash
    npm run build
    ```
7. Restart the realtime server to create a new route for the project.

    ```bash
    sudo supervisorctl restart server_realtime_auth
    ```

    ```cmd
    net restart server_realtime_auth
    ```
8. Access the app in your browser under 'Custom Developments', reload the page to see the changes.

9. Do changes to the code and repeat steps 6 and 8 to see the results.

## Example 1: Basic Bar Graph

This example shows how to create a bar graph with fixed data points. It uses the scadaOpcApi.readRealTimeData() API to retrieve data from the SCADA.

It has been built with Astro and React, with help from the WindSurf editor.

## Example 2: Advanced Dashboard

This example shows how to create a futuristic dashboard with dynamic chosen data points. It has a tree view to select the data points, bar graph, arc graphs, and historical plots.

It has been built with Astro and React, with help from the WindSurf editor.

## Example 3: Transformer with Command

This example shows a transformer drawing with measurement boxes and tap changer buttons.

It has been built with Astro and React, with help from the WindSurf editor.
