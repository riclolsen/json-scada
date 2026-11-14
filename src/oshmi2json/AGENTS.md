# DOX: src/oshmi2json — OSHMI Point List Converter

## Purpose

Node.js tool that converts an OSHMI `point_list.txt` point database into JSON-SCADA import files (a mongo shell script and a CSV). Provides a migration path from OSHMI and a template for importers of other systems.

## Ownership

- oshmi2json owns the OSHMI migration tooling

## Local Contracts

- **Language:** Node.js
- **Main entry:** `oshmi2json.js`
- **Files:**
  - `oshmi2json.js` — main converter logic
  - `point_list.txt` — input point list (sample/working copy)
  - `point_calc.txt` — OSHMI calculation definitions
  - `json-scada-mongo-import.js` / `.csv` — generated output
- **Config:** Command-line-free; input/output paths and `protocolSourceConnectionNumber` are constants in `oshmi2json.js`

## Work Guidance

- Reads the OSHMI point-list format (two header lines, then whitespace-separated records)
- Generates `json-scada-mongo-import.js` (`db.realtimeData` bulk insert) and `json-scada-mongo-import.csv`
- Generates a CSV header with a duplicated `group1` column inherited from OSHMI; document or fix it when validating imports
- OSHMI cannot express all JSON-SCADA parameters; generated points need completion
- Used during migration from legacy OSHMI installations

## Verification

- Run the converter on the sample `point_list.txt`; import the generated script into a test MongoDB and check the resulting points
- `node --check oshmi2json.js` for syntax
