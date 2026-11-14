# DOX: src/alarm_beep — Alarm Audio Notification

## Purpose

Node.js service that reads the state of the reserved `realtimeData` point `_id = -1` and plays an audio notification on the node where it runs. Provides audible alerts for SCADA alarms.

## Ownership

- alarm_beep owns the alarm audio notification system

## Local Contracts

- **Language:** Node.js
- **Main entry:** `alarm_beep.js`
- **Pattern:** standard Node.js app (`app-defs.js`, `load-config.js`, `simple-logger.js`)
- **Config:** `conf/json-scada.json` (`JS_CONFIG_FILE`); `JS_ALARM_BEEP_INSTANCE` / `JS_ALARM_BEEP_LOGLEVEL`

## Work Guidance

- Polls `realtimeData` for `_id = -1` once per second; `value != 0` means alarms are present
- Plays a WAV every 3 s (every 1 s when `beepType >= 2`); `beepType` is a 1-based index into the WAV list
- Windows: `platform-windows/sounder.exe`; other platforms: `aplay` and a WAV path configured in `alarm_beep.js`
- The sound device is on the operator workstation, so run it there, not on the database server
- Does not use change streams; a standalone MongoDB server is sufficient

## Verification

- `npm install` — dependencies install cleanly
- Test with alarm-producing data and verify audio playback
