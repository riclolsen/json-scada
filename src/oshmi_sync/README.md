# {json:scada} oshmi_sync

Copies operator-entered settings from an OSHMI installation to JSON-SCADA:
document notes, blocking annotations, alarm inhibits, limits and hysteresis.

> **Warning:** the tool first clears `notes` and `annotation` for **every** point
> in `realtimeData`, then writes the values found in the OSHMI databases. Run it
> only while migrating or while OSHMI is the authoritative source, and take a
> backup first.

## Requirements

- PHP with the `pdo_sqlite` extension.
- The MongoDB PHP library installed with Composer (`composer install` in this
  directory; it requires `mongodb/mongodb` ^1.8 and a `mongodb` PHP extension).
- Access to the OSHMI SQLite files and to the JSON-SCADA MongoDB server.

## What it does

1. Reads `../../conf/json-scada.json` for the MongoDB connection string.
2. Clears `notes` and `annotation` in the `realtimeData` collection.
3. Reads open notes from `c:/oshmi/db/notes.sl3` and writes them to `notes`.
4. Reads `c:/oshmi/db/dumpdb.sl3` and writes `annotation`, `alarmDisabled`,
   `hiLimit`, `loLimit` and `hysteresis`, matching points by `_id`
   (`POINTNUM`/`NPONTO` in OSHMI).

## Usage

    cd src/oshmi_sync
    composer install
    php oshmi_sync.php

## Configuration

The script has no command-line options. Edit these hard-coded values before
running it:

- the two SQLite paths (`c:/oshmi/db/notes.sl3`, `c:/oshmi/db/dumpdb.sl3`), which
  are Windows paths;
- the database name in `$client->json_scada`, which is fixed to `json_scada`
  rather than read from `mongoDatabaseName`;
- the conf path, if the installation is not in the standard layout.

The script prints the MongoDB driver results for each update. It converts
non-UTF-8 text from OSHMI (assumed ISO-8859-1) to UTF-8 before writing.
