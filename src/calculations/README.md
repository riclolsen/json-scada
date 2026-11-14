# {json:scada} calculations

The _calculations_ process computes calculated points from predefined (compiled) formulas over input points called parcels. It runs on a fixed cycle (default 2 seconds) and recomputes every calculated point at each cycle.

This method is simple and efficient: it does not consume change streams and writes only changed results. The drawback is a latency of up to one cycle. The period can be changed, but it should leave some spare time in each cycle.

To calculate values with very low latency, write a custom change stream process (see [cs_custom_processor](../cs_custom_processor/README.md)) that recalculates as soon as a parcel changes, and give its points a formula code of 100000 or more, which this process ignores.

## How It Works

- The process computes the _realtimeData_ points with `origin: "calculated"` and a `formula` greater than 0. Formulas on points of other origins are ignored.
- The calculated points are read at startup and again every 30 seconds, so new or changed calculated points start computing within 30 seconds, without a restart.
- Each cycle it reads `value` and `invalid` of every parcel and computes the formulas. A result is written to the point's `sourceDataUpdate` (`valueAtSource`, `invalidAtSource`, `transientAtSource`) only when it differs from the last one written.
- The data processor ([cs_data_processor](../cs_data_processor/README.md)) finishes the update as for an acquired value: conversion (`kconv1`, `kconv2` for analogs, inversion with `kconv1 = -1` for digitals), limits, alarms, events and history.
- Parcels are read from their processed `value`, so they are in engineering units.
- A calculated point can be a parcel of another one; each level adds up to one cycle of delay.
- Only the active node of a redundant pair computes (see Process Instance Collection).

## Calculated Point Definition

A calculated point is a _realtimeData_ point with:

- `origin`: `"calculated"`.
- `formula`: a formula code from the tables below.
- `parcels`: the point keys (`_id`) of the parcels, in formula order (P1, P2, ...).
- `type`: `analog` for numeric results, `digital` for 0/1 results (see the Result column).

No protocol fields are needed. Other point fields (description, groups, unit, state texts, limits, `isEvent`, `priority`, ...) work as for any point; keep `kconv1 = 1` and `kconv2 = 0` unless the result must be converted.

```
    {
    "_id": 6240,
    "description": "KNH2~TL1 KYU2 230kV~Apparent Power-Calc",
    "formula": 3,
    "group1": "KNH2",
    "group2": "TL1 KYU2 230kV",
    "group3": "",
    "origin": "calculated",
    "parcels": [28973, 28974],
    ...
    "value": 118.39308345399448
    }

```

Rules:

- The number of parcels must fit the formula (Parcels column). Otherwise the point is skipped and never computed; the process logs the reason in `calculations.log`. Unknown formula codes are logged the same way.
- A parcel key that does not exist counts as 0, invalid, so the result is invalid.
- A point must not be its own parcel, directly or through other calculated points.
- Undefined results (for example the power factor with P = Q = 0, or the square root of a negative number) are 0, invalid.

The [MCP server](../mcp-json-scada-db/README.md) serves this formula catalog to AI assistants and checks calculated points (`get_calculations_guide`, `check_calculated_tag`, `audit_calculated_tags`, `save_calculated_tag`).

## Formulas

Quality: "invalid if any parcel is invalid" unless the table says otherwise. Results are never flagged substituted or not topical.

### General Formulas

| Code | Name | Expression | Parcels | Result | Quality and notes |
|---|---|---|---|---|---|
| 1 | Current from P, Q and voltage | `(1000/sqrt(3)) * sqrt(P1^2 + P2^2) / P3   (0 when P3 <= 0)` | exactly 3 | analog | invalid if any parcel is invalid. P1 = active power (MW), P2 = reactive power (Mvar), P3 = line voltage (kV); result in A |
| 2 | Power factor | `P1 / sqrt(P1^2 + P2^2)` | exactly 2 | analog | invalid if any parcel is invalid. P1 = active power, P2 = reactive power. When P1 = P2 = 0 the result is undefined: 0 with the invalid flag |
| 3 | Apparent power from P and Q | `sqrt(P1^2 + P2^2)` | exactly 2 | analog | invalid if any parcel is invalid. P1 = active power (MW), P2 = reactive power (Mvar); result in MVA |
| 4 | Sum | `P1 + P2 + ... + Pn` | 1 or more | analog | invalid if any parcel is invalid |
| 5 | Square root | `sqrt(P1)` | exactly 1 | analog | invalid if any parcel is invalid. A negative P1 gives 0 with the invalid flag |
| 6 | Logical AND | `1 if every parcel != 0, else 0` | 1 or more | digital | invalid if any parcel is invalid |
| 7 | Logical OR | `1 if any parcel != 0, else 0` | 1 or more | digital | invalid if any parcel is invalid |
| 8 | Timer (Unix time) | `current Unix time in seconds` | 0 or more | analog | never invalid. Needs no parcels (any given are ignored); the value changes every second, so it is written on every cycle |
| 9 | Apparent power from current and voltage | `P1 * P2 * sqrt(3) / 1000` | exactly 2 | analog | invalid if any parcel is invalid. P1 = current (A), P2 = line voltage (kV); result in MVA |
| 10 | Negative sum | `-(P1 + P2 + ... + Pn)` | 1 or more | analog | invalid if any parcel is invalid |
| 14 | Scale | `(P1 * 10) / 6` | exactly 1 | analog | invalid if any parcel is invalid |
| 15 | Difference | `P1 - P2` | exactly 2 | analog | invalid if any parcel is invalid |
| 16 | Weighted sum | `P1 - P2 - P3` | exactly 3 | analog | invalid if any parcel is invalid |
| 17 | Weighted sum | `P1 - P2 - P3 - P4` | exactly 4 | analog | invalid if any parcel is invalid |
| 18 | Weighted sum | `P1 - P2 - P3 - P4 - P5 - P6` | exactly 6 | analog | invalid if any parcel is invalid |
| 19 | Weighted sum | `P1 + P2 - P3` | exactly 3 | analog | invalid if any parcel is invalid |
| 20 | Weighted sum | `P1 + P2 + P3 - P4 - P5 - P6` | exactly 6 | analog | invalid if any parcel is invalid |
| 21 | Weighted sum | `P1 + P2 + P3 + P4 - P5 - P6 - P7 - P8 - P9` | exactly 9 | analog | invalid if any parcel is invalid |
| 22 | Weighted sum | `P1 + P2 + P3 + P4 + P5 + P6 - P7 - P8 - P9` | exactly 9 | analog | invalid if any parcel is invalid |
| 23 | Weighted sum | `P1 + P2 + P3 + P4 + P5 + P6 + P7 + P8 + P9 + P10 - P11` | exactly 11 | analog | invalid if any parcel is invalid |
| 24 | Weighted sum | `P1 + P2 + P3 - P4 - P5 - P6 - P7 - P8 - P9` | exactly 9 | analog | invalid if any parcel is invalid |
| 26 | Band selector | `P2, but 0 when abs(P1) > 1.4 and 1 when abs(P1) <= 0.5` | exactly 2 | any | invalid if any parcel is invalid |
| 50 | Choice of valid parcel (backup sources) | `value of the first valid parcel in the list; P1 when none is valid` | 1 or more | any | invalid only when no parcel is valid. List the preferred source first, then the backups. 50 and 51 are identical |
| 51 | Choice of valid parcel (backup sources) | `value of the first valid parcel in the list; P1 when none is valid` | 1 or more | any | invalid only when no parcel is valid. List the preferred source first, then the backups. 50 and 51 are identical |
| 52 | Any parcel valid | `1 if any parcel is valid, else 0` | 1 or more | digital | never invalid |
| 53 | Span (max - min) | `max(P1..Pn) - min(P1..Pn)` | 1 or more | analog | invalid if any parcel is invalid. Give at least two parcels; with one the result is always 0 |
| 54 | Double point from two single points | `P1 != 0 and P2 == 0 -> 1 (ON); P1 == 0 and P2 != 0 -> 0 (OFF); P1 == P2 -> invalid + transient` | exactly 2 | digital | invalid (and transient) when both inputs are equal, or when any parcel is invalid. P1 = ON/closed indication, P2 = OFF/open indication |
| 55 | Division | `P1 / P2   (P2 = 0: 0 when P1 = 0, else +/- max float)` | exactly 2 | analog | invalid if any parcel is invalid |
| 200 | Weighted sum | `P1 - P2 - P3 - P4 - P5 - P6 - P7 - P8` | exactly 8 | analog | invalid if any parcel is invalid |
| 201 | Weighted sum | `P1 + P2 + P3 + P4 + P5 + P6 + P7 + P8 - P9 - P10 - P11` | exactly 11 | analog | invalid if any parcel is invalid |
| 202 | Minutes and seconds | `(P1 * 60) + P2` | exactly 2 | analog | invalid if any parcel is invalid |
| 203 | Selector | `P1 != 0 ? P3 : P2` | exactly 3 | any | quality of the selected parcel (P1 quality is ignored) |
| 204 | Half | `P1 / 2` | exactly 1 | analog | invalid if any parcel is invalid |
| 205 | Weighted sum | `P1 + P2 - P3 - P4 - P5` | exactly 5 | analog | invalid if any parcel is invalid |
| 206 | Weighted sum | `P1 - P2 - P3 - P4 - P5 - P6 - P7 - P8 - P9` | exactly 9 | analog | invalid if any parcel is invalid |
| 207 | Weighted sum | `P1 + P2 - P3 - P4 - P5 - P6` | exactly 6 | analog | invalid if any parcel is invalid |
| 208 | Weighted sum | `P1 - P2 - P3 - P4 - P5` | exactly 5 | analog | invalid if any parcel is invalid |
| 209 | Weighted sum | `P1 + P2 - P3 - P4 - P5 - P6 - P7 - P8 - P9` | exactly 9 | analog | invalid if any parcel is invalid |
| 210 | Weighted sum | `P1 - P2 - P3 - P4 - P5 - P6 - P7` | exactly 7 | analog | invalid if any parcel is invalid |
| 211 | Weighted sum | `P1 + P2 + P3 + P4 + P5 - P6 - P7 - P8 - P9` | exactly 9 | analog | invalid if any parcel is invalid |
| 213 | Weighted sum | `P1 + P2 + P3 - P4` | exactly 4 | analog | invalid if any parcel is invalid |
| 214 | Weighted sum | `P1 + P2 + P3 + P4 - P5` | exactly 5 | analog | invalid if any parcel is invalid |
| 215 | Weighted sum | `P1 + P2 - P3 - P4` | exactly 4 | analog | invalid if any parcel is invalid |
| 216 | Range check | `1 if P2 <= P1 <= P3, else 0` | exactly 3 | digital | quality of P1 only (the limits P2, P3 are not checked) |
| 217 | Weighted sum | `P1 + P2 + P3 + P4 + P5 - P6` | exactly 6 | analog | invalid if any parcel is invalid |
| 218 | Range check | `1 if P2 < P1 <= P3, else 0` | exactly 3 | digital | quality of P1 only (the limits P2, P3 are not checked) |
| 219 | Range check | `1 if P2 <= P1 < P3, else 0` | exactly 3 | digital | quality of P1 only (the limits P2, P3 are not checked) |
| 220 | Minimum of two | `min(P1, P2)` | exactly 2 | analog | invalid if any parcel is invalid |
| 221 | Maximum of two | `max(P1, P2)` | exactly 2 | analog | invalid if any parcel is invalid |
| 222 | Weighted sum | `P1 + P2 + P3 - P4 - P5 - P6 - P7` | exactly 7 | analog | invalid if any parcel is invalid |
| 223 | Weighted sum | `P1 + P2 + P3 - P4 - P5 - P6 - P7 - P8` | exactly 8 | analog | invalid if any parcel is invalid |
| 224 | Weighted sum | `P1 + P2 + P3 + P4 + P5 + P6 - P7` | exactly 7 | analog | invalid if any parcel is invalid |
| 226 | Weighted sum | `P1 + P2 - P3 - P4 - P5 - P6 - P7 - P8` | exactly 8 | analog | invalid if any parcel is invalid |
| 227 | Weighted sum | `P1 + P2 + P3 - P4 - P5` | exactly 5 | analog | invalid if any parcel is invalid |
| 228 | Weighted sum | `P1 + P2 + P3 - P4 - P5 - P6 - P7 - P8 - P9 - P10 - P11 - P12` | exactly 12 | analog | invalid if any parcel is invalid |
| 229 | Weighted sum | `P1 + P2 + P3 + P4 + P5 + P6 + P7 + P8 + P9 - P10 - P11 - P12` | exactly 12 | analog | invalid if any parcel is invalid |
| 230 | Weighted sum | `P1 + P2 + P3 + P4 + P5 + P6 + P7 + P8 + P9 - P10` | exactly 10 | analog | invalid if any parcel is invalid |
| 231 | Negative part | `P1 > 0 ? 0 : P1` | exactly 1 | analog | invalid if any parcel is invalid |
| 232 | Weighted sum | `P1 + P2 + P3 + P4 + P5 - P6 - P7 - P8` | exactly 8 | analog | invalid if any parcel is invalid |
| 233 | Weighted sum | `P1 + P2 + P3 + P4 + P5 + P6 + P7 - P8 - P9 - P10 - P11` | exactly 11 | analog | invalid if any parcel is invalid |

### Site-Specific Formulas

Kept for existing projects. Prefer the general formulas for new points.

| Code | Name | Expression | Parcels | Result | Quality and notes |
|---|---|---|---|---|---|
| 11 | Weighted sum | `P1 + P2 + P3 + P4 + P5 + P6 + P7 + P8 + P9 + P10 + P11 + P12 + P13 + P14 + P15 + P16 + P17 + P18 + P19 + P20 + P21 + P22 + P23 + 0.72*P24` | exactly 24 | analog | invalid if any parcel is invalid |
| 13 | Weighted sum | `P1 + P2 + P3 + P4 + P5 + P6 + P7 + P8 - P9 + P10 + P11 + P12 + 0.52*P13 + P14 + P15 - P16 - P17 + P18 + P19 + P20 + P21 + P22 - P23 - P24 + P25 + P26 + P27 + P28 + P29 + P30 + P31 + P32 + P33 + 0.2*P34 - 1` | exactly 34 | analog | invalid if any parcel is invalid |
| 225 | Constant 1 | `1` | 0 to 1 | digital | never invalid. The parcel, if any, is ignored |
| 500 | Logical OR | `1 if any parcel != 0, else 0` | 1 or more | digital | invalid if any parcel is invalid. Same as formula 7 |
| 680 | Weighted sum | `P1 + P2 + P3 - P4 - P5` | exactly 5 | analog | invalid if any parcel is invalid |
| 5000 | Weighted sum | `P1 + P2 + P3 - P4 - 0.6*P5 + P6` | exactly 6 | analog | invalid if any parcel is invalid |
| 5022 | Weighted sum | `P1 + 0.65*P2` | exactly 2 | analog | invalid if any parcel is invalid |
| 5041 | Weighted sum | `P1 + P2 - P3 + P4 - P5` | exactly 5 | analog | invalid if any parcel is invalid |
| 5043 | Weighted sum | `P1 + P2 + P3 + P4 - P5 - P6 + P7 - P8 - P9` | exactly 9 | analog | invalid if any parcel is invalid |
| 5172 | Weighted sum | `-0.72*P1` | exactly 1 | analog | invalid if any parcel is invalid |
| 5173 | Weighted sum | `0.38*P1` | exactly 1 | analog | invalid if any parcel is invalid |
| 5174 | Weighted sum | `0.14*P1` | exactly 1 | analog | invalid if any parcel is invalid |
| 5406 | Weighted sum | `-P1 + P2 + P3 + P4 + P5 + P6 - P7 - P8 + P9 + P10 + P11` | exactly 11 | analog | invalid if any parcel is invalid |
| 5711 | Weighted sum | `-P1 - P2 - P3 - P4` | exactly 4 | analog | invalid if any parcel is invalid |
| 7374 | Weighted sum | `P1 - P2 - P3 - P4 - P5 - P6 - P7 - P8 - P9 - P10 - P11 - P12` | exactly 12 | analog | invalid if any parcel is invalid |
| 8055 | Weighted sum | `P1 + P2 - P3 - P4 - P5 - P6 - P7 - P8` | exactly 8 | analog | invalid if any parcel is invalid |
| 25673 | Weighted sum | `P1 - P2` | exactly 2 | analog | invalid if any parcel is invalid. Same as formula 15 |

### Other Codes

- Not implemented (points using them are never computed): 12, 25, 27 to 49, 56 to 199, 212, and every other code below 100000 not listed above.
- 100000 and above: reserved for user processes; ignored by this process.

### Changes in Version 0.2.0

- Formulas 50 and 51 take the first valid parcel, as documented (earlier versions took the last).
- Formula 5022 applies the 0.65 factor to P2 and formula 13 subtracts P23 and P24, as documented (earlier versions computed P1 + P2, and cancelled P23 and P24).
- Only points with `origin: "calculated"` are computed (earlier versions computed any point with a formula).
- Calculated points are reloaded every 30 seconds (earlier versions read them only at startup).
- Missing parcels make the result invalid (earlier versions read them as 0, valid); undefined results are 0, invalid.
- Points whose parcel count does not fit are logged and skipped (earlier versions could crash on formulas 50, 51 and 54 without parcels).
- Fractional calculation periods are honored.

## Compilation

This module should be compiled with the Golang compiler (Go 1.26 or later).

    cd src/calculations
    go build -o ../../bin/calculations

The executable must be in the `bin/` directory of the installation (or a symlink
there) so that, by default, it finds the config file at `../conf/json-scada.json`.
Alternatively pass the config file path as the fourth argument.

The organization of the project files should resemble the structure below.

```
json-scada-dir/                          # main folder
  bin/calculations                       # executable
  conf/json-scada.json                   # config file
  src/calculations/calculations.go       # cycle, MongoDB access, redundancy
  src/calculations/formulas.go           # formula catalog
```

## Process Command Line Arguments And Environment Variables

This driver has the following command line arguments and equivalent environment variables.

- _**1st arg. - Instance Number**_ [Integer] - Instance number to be executed. **Optional argument, default=1**. Env. variable: **JS_CALCULATIONS_INSTANCE**.
- _**2nd arg. - Log. Level**_ [Integer] - Log level (0=minimum,1=basic,2=detailed,3=debug). **Optional argument, default=1**. Env. variable: **JS_CALCULATIONS_LOGLEVEL**.
- _**3rd arg. - Period of Calculation**_ [String] - Time period of calculation in seconds. **Optional argument, default=2.0**. Env. variable: **JS_CALCULATIONS_PERIOD**.
- _**4th arg. - Config File Path/Name**_ [String] - Path/name of the JSON-SCADA config file. **Optional argument, default="../conf/json-scada.json"**. Env. variable: **JS_CONFIG_FILE**.

Command line args take precedence over environment variables.

## Process Instance Collection

A _processInstance_ entry will be created with defaults if one is not found. It can be used to configure some parameters and limit nodes allowed to run instances. Its `logLevel` and `periodOfCalculation` fields raise the values given on the command line (a lower value is ignored).

See also

- [Schema Documentation](../../docs/schema.md)
