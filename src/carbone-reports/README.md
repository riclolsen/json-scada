# {json:scada} carbone-reports

Example report generator built on [Carbone](https://carbone.io). It reads the
point list from `realtimeData` and renders the ODS template
`point_list_template.ods` into `point_list.ods` in the current directory.

The template and this small program are a starting point: adapt the MongoDB
query and the template placeholders to produce your own reports (the Carbone
engine supports PDF, XLSX, DOCX and other formats in addition to ODS).

Requires Node.js.

## Run

    cd src/carbone-reports
    npm install
    node index.js

The program reads `../../conf/json-scada.json`; set `JS_CONFIG_FILE` to use
another config file. It has no command-line arguments and exits when the report
has been written.

## Data and template

The current query projects `_id`, `tag` and `value` from every point and passes
the resulting array to Carbone. Edit `index.js` to select other fields or to
filter points, then edit `point_list_template.ods` with the matching Carbone
placeholders.

The output (`point_list.ods`) is overwritten on each run. Run the program with
the working directory set to `src/carbone-reports`, or the template path will not
resolve.
