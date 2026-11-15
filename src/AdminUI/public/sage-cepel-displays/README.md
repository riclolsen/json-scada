# SAGE Displays 

SAGE (from CEPEL) is a SCADA/EMS package very popular in the brazilian power sector.

JSON-SCADA is designed to be used side-by-side with SAGE or as a replacement in some cases.

SAGE allows to export displays as HTML with embedded SVG code. This export folder can be expanded in the json-scada/src/AdminUI/public/sage-cepel-displays/ location (copied to `dist/` by the AdminUI build, served at `/sage-cepel-displays/`). The displays will updated for visualization only with no interactivity.

This will work provided the tag names are the same in both systems.

The animation script is located at json-scada/src/AdminUI/public/scripts/main.js (self-contained, no other scripts needed).

Directly in this script it is possible to change the data update period (default = 3s).

Pages here should be served with charset="iso-8859-1" headers to display correctly extended characters.

