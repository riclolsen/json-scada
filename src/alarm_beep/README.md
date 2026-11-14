# {json:scada} alarm_beep

Plays an audible alarm on the computer running the process. It is meant to run on
operator workstations (one process per workstation), not on the server, and only
needs network access to the MongoDB server.

Requires Node.js and a working sound device.

## How it works

The process reads the reserved point with `_id = -1` in the `realtimeData`
collection once per second:

- `value` different from `0` means alarms are present, and a sound is played
  every 3 seconds.
- `beepType` selects the sound: it is a 1-based index into the WAV list (`1` is
  the first file, `2` the second, and so on; the list cycles). Values of `2` or
  more also make the beep repeat three times faster (every second). With the
  default single non-Windows WAV the number has no audible effect.
- The point is seeded by `mongo_seed` as `_System.Status.AlarmBeep`
  (`_id = -1`). Keep it if you rebuild the point database, or create an
  equivalent document with `_id = -1`.

On Windows the sound is played by
`platform-windows\sounder.exe` using WAV files from `C:\Windows\Media`. On other
platforms the process calls `aplay` (install `alsa-utils` on Debian/Ubuntu) and
plays `/usr/share/sounds/linuxmint-gdm.wav`. Edit the `WavFilesWin` /
`WavFilesNonWin` arrays in `alarm_beep.js` to use other files.

No change streams are used, so a standalone MongoDB server is enough. If the
database connection is lost the beep stops until the connection is restored.

## Run

    cd src/alarm_beep
    npm install
    node alarm_beep.js

The process reads `../../conf/json-scada.json`. Override the path with
`JS_CONFIG_FILE`; set the instance and log level with `JS_ALARM_BEEP_INSTANCE`
and `JS_ALARM_BEEP_LOGLEVEL` (command-line arguments are not read by this tool).

A sample Supervisor program is not provided for this tool. On Windows the
installer registers it as a service with manual start
(`JSON_SCADA_alarm_beep`); on Linux start it from the operator's desktop session
or add your own Supervisor/systemd entry. Since the sound must reach the
operator, run it on the workstation rather than alongside the database.
