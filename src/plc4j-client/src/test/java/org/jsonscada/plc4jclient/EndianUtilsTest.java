package org.jsonscada.plc4jclient;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

import org.junit.jupiter.api.Test;

class EndianUtilsTest {

  @Test
  void swapSelection() {
    assertTrue(EndianUtils.shouldSwap("LITTLE_ENDIAN"));
    assertTrue(EndianUtils.shouldSwap("REV_ENDIAN"));
    assertFalse(EndianUtils.shouldSwap("BIG_ENDIAN"));
    assertFalse(EndianUtils.shouldSwap(""));
  }

  @Test
  void swaps() {
    assertEquals((short) 0x0201, EndianUtils.swap((short) 0x0102));
    assertEquals(0xFF00, EndianUtils.swapU16(0x00FF));
    assertEquals(0x78563412, EndianUtils.swap(0x12345678));
    assertEquals(0xFF000000L, EndianUtils.swapU32(0xFFL));
    assertEquals(1.5f, EndianUtils.swap(EndianUtils.swap(1.5f)));
    assertEquals(2.5, EndianUtils.swap(EndianUtils.swap(2.5)));
  }
}
