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

/**
 * Byte-order adjustments for values already decoded by PLC4X.
 *
 * PLC4X delivers values interpreted with the protocol default byte order
 * (big-endian for Modbus). "LITTLE_ENDIAN" means the device actually stores
 * little-endian words, so the bytes must be swapped; "REV_ENDIAN" is an
 * unconditional reverse; "BIG_ENDIAN" or empty is a no-op. This is applied
 * consistently to all multi-byte types.
 */
public final class EndianUtils {

  private EndianUtils() {}

  public static boolean shouldSwap(String endianness) {
    return "LITTLE_ENDIAN".equals(endianness) || "REV_ENDIAN".equals(endianness);
  }

  public static short swap(short v) {
    return Short.reverseBytes(v);
  }

  public static int swapU16(int v) {
    return Short.toUnsignedInt(Short.reverseBytes((short) v));
  }

  public static int swap(int v) {
    return Integer.reverseBytes(v);
  }

  public static long swapU32(long v) {
    return Integer.toUnsignedLong(Integer.reverseBytes((int) v));
  }

  public static long swap(long v) {
    return Long.reverseBytes(v);
  }

  public static float swap(float v) {
    return Float.intBitsToFloat(Integer.reverseBytes(Float.floatToRawIntBits(v)));
  }

  public static double swap(double v) {
    return Double.longBitsToDouble(Long.reverseBytes(Double.doubleToRawLongBits(v)));
  }
}
