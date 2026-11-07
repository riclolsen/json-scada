# DOX: src/oshmi_sync — OSHMI Settings Sync

## Purpose

PHP tool that copies operator-entered settings from an OSHMI installation (notes, annotations, alarm inhibits, limits, hysteresis) into the JSON-SCADA `realtimeData` collection during migration.

## Ownership

- oshmi_sync owns the OSHMI settings migration tool

## Local Contracts

- **Language:** PHP (PDO SQLite + `mongodb/mongodb` via Composer)
- **Main entry:** `oshmi_sync.php`
- **Dependencies:** `composer.json` (`mongodb/mongodb` ^1.8), `composer install`
- **Config:** hard-coded paths to `../../conf/json-scada.json` and OSHMI SQLite files

## Work Guidance

- Clears `notes` and `annotation` for every point before writing OSHMI values
- Reads `notes.sl3` and `dumpdb.sl3`; matches points by `_id` = OSHMI point number
- The SQLite paths and database name (`json_scada`) are hard-coded and must be edited per installation
- Destructive by design: require a backup and run only while OSHMI is authoritative

## Verification

- `composer install` then `php -l oshmi_sync.php`
- Test against a copy of the database and confirm the expected points change
