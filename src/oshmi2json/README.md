## OSHMI2JSON File Converter

A tool to convert OSHMI point_list.txt files to JSON-SCADA MongoDB script and CSV files.

Please notice that the OSHMI point list data can not fulfill all JSON-SCADA parameters. So it will be necessary to edit and complete information before or after importing the files.

This can serve also as a template script to create importers for data from other systems.

Input file: point_list.txt.
Input files: json-scada-mongo-import.js and json-scada-mongo-import.csv.

Just execute

    node oshmi2json.js 

The json-scada-mongo-import.js import file can be imported directly by the mongo shell.

The CSV file can be imported by the _mongoimport_ tool.

Requires Node.js.
