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

import java.util.Locale;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/**
 * Parses connection "topics" entries: "TAG_NAME|PLC4X_ADDRESS|ENDIANNESS"
 * (2nd and 3rd parts optional). Same conventions as the Go plc4x-client.
 *
 * <p>Array addresses accept both notations: the legacy count after the type,
 * "holding-register:20:INT[10]" (what the Go plc4x-client and PLC4X up to 0.13.x
 * use, and what existing JSON-SCADA configurations contain), and the PLC4X 1.0.0
 * range before the type, "holding-register:20[0..9]:INT". The legacy form is
 * translated before it reaches PLC4X while the configured text stays the PLC4X
 * tag name, so realtimeData tags keep matching and the same connection
 * configuration works with either driver executable.
 */
public final class TopicParser {

  /** Legacy array notation: everything ending in ":TYPE[count]". */
  private static final Pattern LEGACY_ARRAY_PATTERN =
      Pattern.compile("^(.*):([A-Za-z_][A-Za-z0-9_]*)\\[(\\d+)\\]$");
  /** PLC4X 1.0.0 array notation: "[lower..upper]" before the type. */
  private static final Pattern RANGE_ARRAY_PATTERN =
      Pattern.compile("\\[(\\d+)\\.\\.(\\d+)\\]");

  public static class ParsedTopic {
    public String jsTagName = "";
    /** Address exactly as configured: PLC4X tag name and realtimeData key. */
    public String address = "";
    /** Address handed to PLC4X (legacy array notation translated to 1.0.0). */
    public String plc4xAddress = "";
    public String endianness = "";
    public int arrayLength = 0; // 0 = not an array address
    public String jsType = "analog";
  }

  private TopicParser() {}

  public static ParsedTopic parse(String topic, String addrSeparator) {
    ParsedTopic pt = new ParsedTopic();
    String[] parts = topic.split("\\|");
    if (parts.length > 2) {
      pt.jsTagName = parts[0];
      pt.address = parts[1];
      pt.endianness = parts[2].toUpperCase(Locale.ROOT).trim();
    } else if (parts.length > 1) {
      pt.jsTagName = parts[0];
      pt.address = parts[1];
    } else {
      pt.address = topic;
    }
    pt.jsType = inferType(pt.address, addrSeparator);
    pt.plc4xAddress = toPlc4xAddress(pt.address);
    pt.arrayLength = arrayLength(pt.address);
    return pt;
  }

  /**
   * Number of elements of an array address, 0 when scalar and -1 when the array
   * notation cannot be parsed.
   */
  public static int arrayLength(String address) {
    Matcher legacy = LEGACY_ARRAY_PATTERN.matcher(address);
    if (legacy.matches()) {
      try {
        return Integer.parseInt(legacy.group(3));
      } catch (NumberFormatException e) {
        return -1;
      }
    }
    Matcher range = RANGE_ARRAY_PATTERN.matcher(address);
    if (range.find()) {
      try {
        int lower = Integer.parseInt(range.group(1));
        int upper = Integer.parseInt(range.group(2));
        return upper >= lower ? upper - lower + 1 : -1;
      } catch (NumberFormatException e) {
        return -1;
      }
    }
    return 0;
  }

  /**
   * Translates the legacy ":TYPE[count]" array notation to the "[0..count-1]:TYPE"
   * form required from PLC4X 1.0.0 on. Any other address is returned unchanged.
   */
  public static String toPlc4xAddress(String address) {
    Matcher legacy = LEGACY_ARRAY_PATTERN.matcher(address);
    if (!legacy.matches()) {
      return address;
    }
    int count;
    try {
      count = Integer.parseInt(legacy.group(3));
    } catch (NumberFormatException e) {
      return address;
    }
    if (count < 1) {
      return address;
    }
    return legacy.group(1) + "[0.." + (count - 1) + "]:" + legacy.group(2);
  }

  /**
   * JSON-SCADA tag type inferred from the data type inside the PLC4X address.
   * (The Go driver compares the uppercased address against mixed-case "Struct",
   * which never matches — fixed here by uppercasing both sides.)
   */
  public static String inferType(String address, String addrSeparator) {
    String addr = address.toUpperCase(Locale.ROOT);
    if (addr.contains(addrSeparator + "BOOL")) {
      return "digital";
    }
    if (addr.contains(addrSeparator + "STRING") || addr.contains(addrSeparator + "CHAR")) {
      return "string";
    }
    if (addr.contains(addrSeparator + "STRUCT")
        || addr.contains(addrSeparator + "LIST")
        || addr.contains(addrSeparator + "RAW_BYTE_ARRAY")) {
      return "json";
    }
    return "analog";
  }

  /** Address separator used for data-type detection, by URL scheme. */
  public static String addrSeparatorForScheme(String scheme) {
    switch (scheme) {
      case "knxnet-ip":
        return "/";
      case "opcua":
        return ";";
      default:
        return ":";
    }
  }
}
