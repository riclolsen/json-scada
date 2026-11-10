# DOX: src/camera-onvif — ONVIF Camera Control

## Purpose

Node.js protocol driver for ONVIF-compatible IP cameras and plain RTSP sources. Provides camera PTZ control (via commandsQueue tags like `$$CAM001$$relativeMove$$x`), periodic JPEG snapshot capture, and MPEG1/JSMpeg WebSocket streaming (RTSP → ffmpeg → WebSocket) for browser viewing.

## Ownership

- camera-onvif owns the ONVIF camera integration

## Local Contracts

- **Language:** Node.js (>=18, uses global fetch)
- **Main entry:** `index.js`
- **Structure:**
  - `index.js` — main application logic (Mongo reconnect loop, camera lifecycle with retry, command handling with acks, snapshot fetch with basic/digest auth)
  - `redundancy.js` — standard active/standby redundancy control (same module as other Node drivers)
  - `app-defs.js` — driver name (`ONVIF`), env prefix (`JS_ONVIF_`), version
  - `load-config.js` — configuration loader
  - `simple-logger.js` — logging utility
  - `camera.html` — web-based camera viewer (JSMpeg player + PTZ controls; canonical copies in `src/AdminUI/public` and `src/AdminUI/dist` must be kept in sync)
  - `jsmpeg.min.js` — JSMPEG decoder for video streaming
  - `ffmpeg.exe` — FFmpeg bundled for Windows (Linux uses PATH; override with `JS_ONVIF_FFMPEG_PATH`)
  - `snapshots/` — runtime output folder for periodic snapshots (`<connection name>.jpg`)
- **Config:** `conf/json-scada.json` + `protocolDriverInstances`/`protocolConnections` collections (see README.md for all connection parameters)
- **Connection stats:** `stats.isConnected` = stream running (`conn.stream !== null`); the stream is torn down on any camera failure

## Work Guidance

- Supports ONVIF Profile S cameras for PTZ control and media streaming; `rtsp://` endpoints are streaming-only (no PTZ/snapshots)
- Streams via node-rtsp-stream: ffmpeg transcodes RTSP to MPEG1-TS pushed over a WebSocket per connection (port from `ipAddressLocalBind`); nginx templates proxy `/camNNN` → port 9NNN
- PTZ commands mapped from commandsQueue tags; acks written back (`delivered`/`ack`/`resultDescription`)
- Supports multiple cameras simultaneously; only the redundancy-active node runs streams

## Verification

- `npm install` — dependencies install cleanly
- `node --check index.js` — syntax check
- Test with an ONVIF-compatible camera on the network (or any RTSP URL for the streaming path)
- Verify camera.html loads in browser and displays video
