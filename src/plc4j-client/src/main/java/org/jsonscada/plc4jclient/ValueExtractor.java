/*
 * PLC4J Client - Generic PLC Protocol driver for {json:scada}
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 */

package org.jsonscada.plc4jclient;

import java.math.BigInteger;
import java.time.ZoneOffset;
import java.util.ArrayList;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import org.apache.plc4x.java.api.value.PlcValue;
import org.bson.Document;

/**
 * Converts PLC4X values to the JSON-SCADA value tuple (double, string, json, array, bad).
 * Endianness handling is consistent for all multi-byte types.
 *
 * The switch is done on PlcValueType names (strings) so the code is resilient to
 * enum-constant differences between PLC4J versions.
 */
public final class ValueExtractor {

  private ValueExtractor() {}

  public static ExtractedValue extract(
      PlcValue v, String endianness, String connName, String plc4xTagName) {
    ExtractedValue r = new ExtractedValue();
    if (v == null) {
      r.bad = true;
      return r;
    }
    boolean swap = EndianUtils.shouldSwap(endianness);
    String type = v.getPlcValueType() != null ? v.getPlcValueType().name() : "Unknown";
    try {
      switch (type) {
        case "Unknown":
        case "NULL":
          r.bad = true;
          r.valStr = type;
          break;

        case "BOOL": {
          boolean b = v.getBoolean();
          r.valDbl = b ? 1 : 0;
          r.valStr = b ? "true" : "false";
          r.valJson = r.valStr;
          logDetailed(connName, plc4xTagName, r.valStr);
          break;
        }

        case "BYTE": {
          r.valDbl = v.getInteger() & 0xFF;
          setNumInt(r);
          logDetailed(connName, plc4xTagName, r.valStr);
          break;
        }

        case "USINT": {
          r.valDbl = v.getInteger() & 0xFF;
          setNumInt(r);
          logDetailed(connName, plc4xTagName, r.valStr);
          break;
        }

        case "SINT": {
          r.valDbl = (byte) v.getInteger();
          setNumInt(r);
          logDetailed(connName, plc4xTagName, r.valStr);
          break;
        }

        case "UINT":
        case "WORD": {
          int raw = v.getInteger() & 0xFFFF;
          if (swap) {
            raw = EndianUtils.swapU16(raw);
          }
          r.valDbl = raw;
          setNumInt(r);
          logDetailed(connName, plc4xTagName, r.valStr);
          break;
        }

        case "INT": {
          short s = (short) v.getInteger();
          if (swap) {
            s = EndianUtils.swap(s);
          }
          r.valDbl = s;
          setNumInt(r);
          logDetailed(connName, plc4xTagName, r.valStr);
          break;
        }

        case "UDINT":
        case "DWORD": {
          long raw = v.getLong() & 0xFFFFFFFFL;
          if (swap) {
            raw = EndianUtils.swapU32(raw);
          }
          r.valDbl = raw;
          setNumInt(r);
          logDetailed(connName, plc4xTagName, r.valStr);
          break;
        }

        case "DINT": {
          int i = (int) v.getLong();
          if (swap) {
            i = EndianUtils.swap(i);
          }
          r.valDbl = i;
          setNumInt(r);
          logDetailed(connName, plc4xTagName, r.valStr);
          break;
        }

        case "ULINT":
        case "LWORD": {
          BigInteger bi = v.getBigInteger();
          if (swap) {
            long sw = EndianUtils.swap(bi.longValue());
            bi = new BigInteger(Long.toUnsignedString(sw));
          }
          r.valDbl = bi.doubleValue();
          r.valStr = bi.toString();
          r.valJson = bi.toString();
          logDetailed(connName, plc4xTagName, r.valStr);
          break;
        }

        case "LINT": {
          long l = v.getLong();
          if (swap) {
            l = EndianUtils.swap(l);
          }
          r.valDbl = l;
          setNumInt(r);
          logDetailed(connName, plc4xTagName, r.valStr);
          break;
        }

        case "REAL": {
          float f = v.getFloat();
          if (swap) {
            f = EndianUtils.swap(f);
          }
          r.valDbl = f;
          setNumFloat(r);
          logDetailed(connName, plc4xTagName, r.valStr);
          break;
        }

        case "LREAL": {
          double d = v.getDouble();
          if (swap) {
            d = EndianUtils.swap(d);
          }
          r.valDbl = d;
          setNumFloat(r);
          logDetailed(connName, plc4xTagName, r.valStr);
          break;
        }

        case "TIME":
        case "LTIME": {
          long ms = v.getDuration().toMillis();
          if (swap) {
            ms = EndianUtils.swap(ms);
          }
          r.valDbl = ms;
          setNumInt(r);
          r.valJson = jsonQuote(v.getDuration().toString());
          logDetailed(connName, plc4xTagName, r.valStr);
          break;
        }

        case "DATE":
        case "LDATE": {
          long ms = v.getDate().toEpochDay() * 86400000L;
          if (swap) {
            ms = EndianUtils.swap(ms);
          }
          r.valDbl = ms;
          setNumInt(r);
          r.valJson = jsonQuote(v.getDate().toString());
          logDetailed(connName, plc4xTagName, r.valStr);
          break;
        }

        case "TIME_OF_DAY":
        case "LTIME_OF_DAY": {
          long ms = v.getTime().toNanoOfDay() / 1_000_000L;
          if (swap) {
            ms = EndianUtils.swap(ms);
          }
          r.valDbl = ms;
          setNumInt(r);
          r.valJson = jsonQuote(v.getTime().toString());
          logDetailed(connName, plc4xTagName, r.valStr);
          break;
        }

        case "DATE_AND_TIME":
        case "DATE_AND_LTIME":
        case "LDATE_AND_TIME": {
          long ms = v.getDateTime().toInstant(ZoneOffset.UTC).toEpochMilli();
          if (swap) {
            ms = EndianUtils.swap(ms);
          }
          r.valDbl = ms;
          setNumInt(r);
          r.valJson = jsonQuote(v.getDateTime().toString());
          logDetailed(connName, plc4xTagName, r.valStr);
          break;
        }

        case "CHAR":
        case "WCHAR":
        case "STRING":
        case "WSTRING": {
          String s = v.getString();
          if (s == null) {
            s = "";
          }
          r.valStr = s;
          try {
            r.valDbl = Double.parseDouble(s.trim());
          } catch (NumberFormatException e) {
            r.valDbl = 0;
          }
          r.valJson = jsonQuote(s);
          logDetailed(connName, plc4xTagName, r.valStr);
          break;
        }

        case "Struct": {
          Document doc = new Document();
          Map<String, ? extends PlcValue> struct = v.getStruct();
          if (struct != null) {
            for (Map.Entry<String, ? extends PlcValue> e : struct.entrySet()) {
              doc.append(e.getKey(), toPlainJava(e.getValue()));
            }
          }
          r.valJson = doc.toJson();
          r.valStr = r.valJson;
          logDetailed(connName, plc4xTagName, r.valStr);
          break;
        }

        case "List": {
          List<? extends PlcValue> list = v.getList();
          StringBuilder sb = new StringBuilder("[");
          for (int i = 0; i < list.size(); i++) {
            ExtractedValue ev = extract(list.get(i), endianness, connName, plc4xTagName);
            if (ev.bad) {
              r.bad = true;
            }
            r.valArrDbl.add(ev.valDbl);
            if (i == 0) {
              r.valDbl = ev.valDbl;
            }
            if (i > 0) {
              sb.append(",");
            }
            sb.append(formatDouble(ev.valDbl));
          }
          sb.append("]");
          r.valJson = sb.toString();
          r.valStr = r.valJson;
          logDetailed(connName, plc4xTagName, r.valStr);
          break;
        }

        case "RAW_BYTE_ARRAY": {
          byte[] raw = v.getRaw();
          StringBuilder sb = new StringBuilder("[");
          if (raw != null) {
            for (int i = 0; i < raw.length; i++) {
              double b = raw[i] & 0xFF;
              r.valArrDbl.add(b);
              if (i == 0) {
                r.valDbl = b;
              }
              if (i > 0) {
                sb.append(",");
              }
              sb.append(formatDouble(b));
            }
          }
          sb.append("]");
          r.valJson = sb.toString();
          r.valStr = r.valJson;
          logDetailed(connName, plc4xTagName, r.valStr);
          break;
        }

        default:
          r.bad = true;
          break;
      }
    } catch (Exception e) {
      r.bad = true;
      if (Log.level >= Log.LEVEL_DETAILED) {
        Log.log(connName + ": Read result '" + plc4xTagName + "': error extracting " + type
            + " - " + e);
      }
    }
    if (Log.level >= Log.LEVEL_DETAILED && r.bad) {
      Log.log(connName + ": Read result '" + plc4xTagName + "': error reading " + type + "!");
    }
    return r;
  }

  /** Recursive conversion of a PlcValue to a plain Java object for BSON/JSON embedding. */
  static Object toPlainJava(PlcValue v) {
    if (v == null || v.getPlcValueType() == null) {
      return null;
    }
    switch (v.getPlcValueType().name()) {
      case "BOOL":
        return v.getBoolean();
      case "BYTE":
      case "USINT":
      case "SINT":
      case "UINT":
      case "WORD":
      case "INT":
        return v.getInteger();
      case "UDINT":
      case "DWORD":
      case "DINT":
      case "LINT":
        return v.getLong();
      case "ULINT":
      case "LWORD":
        return v.getBigInteger().doubleValue();
      case "REAL":
        return (double) v.getFloat();
      case "LREAL":
        return v.getDouble();
      case "CHAR":
      case "WCHAR":
      case "STRING":
      case "WSTRING":
        return v.getString();
      case "Struct": {
        Document doc = new Document();
        Map<String, ? extends PlcValue> struct = v.getStruct();
        if (struct != null) {
          for (Map.Entry<String, ? extends PlcValue> e : struct.entrySet()) {
            doc.append(e.getKey(), toPlainJava(e.getValue()));
          }
        }
        return doc;
      }
      case "List": {
        List<Object> out = new ArrayList<>();
        for (PlcValue item : v.getList()) {
          out.add(toPlainJava(item));
        }
        return out;
      }
      default:
        try {
          return v.getString();
        } catch (Exception e) {
          return null;
        }
    }
  }

  private static void setNumInt(ExtractedValue r) {
    r.valStr = String.format(Locale.US, "%.0f", r.valDbl);
    r.valJson = r.valStr;
  }

  private static void setNumFloat(ExtractedValue r) {
    if (Double.isNaN(r.valDbl) || Double.isInfinite(r.valDbl)) {
      // NaN/Infinity are not valid JSON nor usable analog values: report as bad
      r.valStr = String.valueOf(r.valDbl);
      r.valDbl = 0;
      r.valJson = "null";
      r.bad = true;
      return;
    }
    r.valStr = String.format(Locale.US, "%f", r.valDbl);
    r.valJson = String.valueOf(r.valDbl);
  }

  static String formatDouble(double d) {
    if (d == Math.floor(d) && !Double.isInfinite(d)) {
      return String.format(Locale.US, "%.0f", d);
    }
    return String.valueOf(d);
  }

  /** Minimal JSON string quoting/escaping. */
  static String jsonQuote(String s) {
    StringBuilder sb = new StringBuilder("\"");
    for (int i = 0; i < s.length(); i++) {
      char c = s.charAt(i);
      switch (c) {
        case '"':
          sb.append("\\\"");
          break;
        case '\\':
          sb.append("\\\\");
          break;
        case '\n':
          sb.append("\\n");
          break;
        case '\r':
          sb.append("\\r");
          break;
        case '\t':
          sb.append("\\t");
          break;
        default:
          if (c < 0x20) {
            sb.append(String.format("\\u%04x", (int) c));
          } else {
            sb.append(c);
          }
      }
    }
    sb.append("\"");
    return sb.toString();
  }

  private static void logDetailed(String connName, String tagName, String valStr) {
    if (Log.level >= Log.LEVEL_DETAILED) {
      Log.log(connName + ": Read result '" + tagName + "': '" + valStr + "'");
    }
  }
}
