# {json:scada} OSHMI2JSON File Converter

Converts an OSHMI `point_list.txt` file into JSON-SCADA point import files (a
mongo shell script and a CSV file). It is intended as a migration helper and as
a template for importers of data from other systems.

Requires Node.js.

## Input

`point_list.txt` must be in the OSHMI point-list format: a header line, a second
ignored line, then one whitespace-separated record per point. The converter
reads the point key, address, tag, type (`D` digital or `A` analog), message or
unit, alarm/equipment/info/origin/special codes, RTU address, IEC ASDU type,
conversion constants, supervision, alarm state, priority, default value and the
`group1~group2~description` field.

The OSHMI point list cannot express every JSON-SCADA parameter, so the generated
files need editing and completion before or after import.

## Output

Both files are written in the current working directory:

| File | Use |
|---|---|
| `json-scada-mongo-import.js` | mongo shell script that bulk-inserts into `realtimeData` |
| `json-scada-mongo-import.csv` | CSV for `mongoimport` |

The mongo shell script starts with `// use json_scada`; uncomment/change it to
the target database, or pass the database on the shell command line.

## Usage

Edit `point_list.txt` if needed, then run:

    cd src/oshmi2json
    npm install
    node oshmi2json.js

Import the result with either tool:

    mongosh "mongodb://localhost:27017/json_scada" json-scada-mongo-import.js

    mongoimport --type=csv --headerline --db json_scada \
      --collection realtimeData json-scada-mongo-import.csv

The source connection number is set by the `protocolSourceConnectionNumber`
variable at the top of `oshmi2json.js` (default `1`); adjust it to the
`protocolConnectionNumber` that will receive the data.

Import the result before starting the protocol driver or the data processor, and
review the generated points afterwards: limits, command relations and
protocol-specific fields often need completion. The CSV has a duplicated
`group1` column in the header (inherited from the OSHMI exporter); fix the
header if your import tool is strict.

## Files

- `oshmi2json.js` — converter
- `point_list.txt` — input sample/working copy
- `point_calc.txt` — calculation definitions used by the OSHMI source system
- `json-scada-mongo-import.js` / `.csv` — last generated output
