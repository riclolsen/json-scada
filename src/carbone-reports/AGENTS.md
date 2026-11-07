# DOX: src/carbone-reports — Carbone Report Generator

## Purpose

Node.js example report generator using the Carbone report engine. Reads the point list from MongoDB and renders an ODS template into an ODS file.

## Ownership

- carbone-reports owns the Carbone report generation example

## Local Contracts

- **Language:** Node.js
- **Main entry:** `index.js`
- **Structure:**
  - `index.js` — report generation logic
  - `app-defs.js` — application definitions
  - `load-config.js` — configuration loader
  - `simple-logger.js` — logging utility
  - `point_list_template.ods` — Carbone template
- **Config:** JSON-SCADA config file (`JS_CONFIG_FILE`)
- **Output:** `point_list.ods`

## Work Guidance

- Uses Carbone.js templating to merge MongoDB data with the ODS template
- The example reads `_id`, `tag` and `value` from every `realtimeData` point
- Carbone can render other formats, but this example writes ODS only
- It is a one-shot program, not a long-running service

## Verification

- `npm install` then `node index.js`; confirm `point_list.ods` is created and opens
