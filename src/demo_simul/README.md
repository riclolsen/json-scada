## Demo Simulator

Simulates the electrical process behind the {json:scada} demo database: it
publishes coherent measurements, answers supervisory commands and produces
protection events, exactly as a real protocol driver would.

Requires Node.js.

    cd src/demo_simul
    npm install
    node index.js

The simulator writes `sourceDataUpdate` documents into `realtimeData` (the same
integration point used by the protocol drivers), so `cs_data_processor` converts
the values, evaluates alarms and generates SOE events normally. It also keeps
the `IEC60870-5-104` driver instance alive and consumes `commandsQueue`.

### The model

The topology is discovered from the database itself at startup (and reloaded
periodically), using the demo tag convention `SSSSMMMMMPPPP…` — 4 characters of
station, 5 of module/bay, then the IEC 61850-like point code — together with
`group1`/`group2` and the point `unit`. Points are grouped into **bays**, bays
into **buses** (station + nominal voltage read from `group2`) and transformer
windings into **transformer elements**.

Points of `CIM-EMS` and `UNDEFINED` connections are left alone (not simulated, not revalidated, their commands
not answered): they belong to the CIM-EMS driver and to external simulators such as the Svedala demo field
simulator (`python -m cim_ems.demo_field`, see `src/cim-ems/README.md`).

Everything is anchored on `valueDefault`, so the simulation always looks like
the seeded demo database, only alive and self-consistent:

| Quantity | Point code | How it is simulated |
| --- | --- | --- |
| Active power | `MTWT` | daily load curve × per-bay stochastic load × energization |
| Reactive power | `MTVR` | follows P with a drifting power factor; capacitor banks follow V² |
| Apparent power | `MTVA` | `sqrt(P² + Q²)` of the bay |
| Current | `MAPH` | `1000/sqrt(3) · S / V` with a small per-phase unbalance |
| Voltage | `MVPP` | per-bus per-unit voltage with load droop, tap boost and fault dips |
| Frequency | `MFHZ` | one system frequency for the whole model, disturbed by load ramps |
| Tap position | `YTAP` | integer, moved by commands or by the AVR when `ATCC` is INCLUDED |
| Oil / winding temp. | `YIMT` / `YHPT` | first-order thermal model driven by loading² and ambient |
| Ambient temperature | `ZTMP` | daily temperature curve |
| Aux. services | `ZVAC` / `ZVDC` | AC follows the station voltage, DC is battery-stable |
| Fault distance | `RFLO` | written when a fault occurs on that line |

Coupled behavior worth watching in the viewers:

- **Breakers matter.** Opening an `XCBR` (by command or by protection) zeroes
  the P, Q and current of its bay; closing it brings the load back through a
  cold-load-pickup ramp.
- **Transformers balance.** For two-winding transformers metered on both sides
  the outgoing winding is derived from the incoming one minus no-load and load
  losses, so losses always have the right sign (~0.6 % at rated load).
- **Tap changers move voltage.** A raise/lower command, or the AVR, shifts the
  low-voltage bus by 0.625 % per step; when several transformers feed the same
  bus, the bus follows their average tap.
- **Short circuits run a full sequence.** Protection pickup and fault current,
  trip and voltage dip, fault location, auto-reclosing when the recloser is
  INCLUDED, lockout when the fault is permanent and later restoration.
- **Annunciation alarms** are raised on real alarm points and reset themselves,
  instead of randomly flipping every digital point.

At startup the simulator also repairs the seeded database: breakers of bays that
carry power are closed, and protection/reclosing signals are put back at rest.

### Configuration

All tunables can be overridden with `JS_DEMO_SIMUL_`-prefixed environment
variables (see `sim-config.js` for the full list and defaults):

| Variable | Default | Meaning |
| --- | --- | --- |
| `JS_DEMO_SIMUL_STEP_PERIOD_MS` | 2000 | simulation step and publish period |
| `JS_DEMO_SIMUL_LOAD_CYCLE_MINUTES` | 60 | real minutes for one simulated 24 h load curve (0 = wall clock) |
| `JS_DEMO_SIMUL_ANALOG_DEADBAND` | 0.002 | publish deadband, fraction of the point default |
| `JS_DEMO_SIMUL_FORCE_REFRESH_SECONDS` | 300 | republish unchanged points at least this often |
| `JS_DEMO_SIMUL_ENABLE_FAULTS` | true | enable short circuits |
| `JS_DEMO_SIMUL_FAULT_RATE_PER_HOUR` | 6 | expected faults per simulated hour |
| `JS_DEMO_SIMUL_ENABLE_ALARMS` | true | enable annunciation alarms |
| `JS_DEMO_SIMUL_ALARM_RATE_PER_HOUR` | 30 | expected alarms per hour |
| `JS_DEMO_SIMUL_ENABLE_TAP_AVR` | true | automatic voltage regulation on transformers with `ATCC` INCLUDED |
| `JS_DEMO_SIMUL_VOLTAGE_DROOP` | 0.03 | p.u. voltage drop at full load |
| `JS_DEMO_SIMUL_MODEL_REFRESH_MINUTES` | 10 | reload the point list (picks up new tags) |
| `JS_DEMO_SIMUL_LOGLEVEL` | 1 | 2 shows each step, breaker moves and AVR operations |

Example, a fast and eventful demo:

    JS_DEMO_SIMUL_LOAD_CYCLE_MINUTES=10 JS_DEMO_SIMUL_FAULT_RATE_PER_HOUR=60 node index.js
