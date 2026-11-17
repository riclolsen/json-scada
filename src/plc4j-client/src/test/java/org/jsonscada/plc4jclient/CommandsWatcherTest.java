package org.jsonscada.plc4jclient;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.math.BigInteger;
import org.junit.jupiter.api.Test;

class CommandsWatcherTest {

  private static Object build(String address, String asdu, double v, String s) {
    return CommandsWatcher.buildWriteValue(new ProtocolConnection(), address, asdu, v, s);
  }

  @Test
  void writesTypedValues() {
    assertEquals(true, build("coil:1:BOOL", "", 1, null));
    assertEquals((short) 255, build("holding-register:1:USINT", "", 255, null));
    assertEquals((short) -5, build("holding-register:1:INT", "", -5, null));
    assertEquals(65535, build("holding-register:1:UINT", "", 65535, null));
    assertEquals(4294967295L, build("holding-register:1:UDINT", "", 4294967295.0, null));
    assertEquals(-7, build("holding-register:1:DINT", "", -7, null));
    assertEquals(1.5f, build("holding-register:1:REAL", "", 1.5, null));
    assertEquals(2.5, build("holding-register:1:LREAL", "", 2.5, null));
    assertEquals(new BigInteger("18446744073709549568"),
        build("holding-register:1:ULINT", "", 18446744073709549568.0, null));
  }

  @Test
  void appliesEndiannessSwap() {
    assertEquals((short) 0x0201, build("holding-register:1:INT", "LITTLE_ENDIAN", 0x0102, null));
  }

  @Test
  void stringTypesUseValueString() {
    assertEquals("abc", build("holding-register:1:STRING", "", 0, "abc"));
    assertEquals("abc", build("holding-register:1:WSTRING", "", 0, "abc"));
  }

  @Test
  void outOfRangeValuesAreRefused() {
    assertThrows(UnsupportedOperationException.class,
        () -> build("holding-register:1:INT", "", 40000, null));
    assertThrows(UnsupportedOperationException.class,
        () -> build("holding-register:1:UINT", "", -1, null));
    assertThrows(UnsupportedOperationException.class,
        () -> build("holding-register:1:SINT", "", 128, null));
    assertThrows(UnsupportedOperationException.class,
        () -> build("holding-register:1:UDINT", "", 4294967296.0, null));
    assertThrows(UnsupportedOperationException.class,
        () -> build("holding-register:1:DINT", "", Double.NaN, null));
    assertThrows(UnsupportedOperationException.class,
        () -> build("holding-register:1:LINT", "", 1e19, null));
    assertThrows(UnsupportedOperationException.class,
        () -> build("holding-register:1:ULINT", "", 1.9e19, null));
    assertThrows(UnsupportedOperationException.class,
        () -> build("holding-register:1:REAL", "", 1e300, null));
  }

  @Test
  void structuredTypesAreRefused() {
    assertThrows(UnsupportedOperationException.class,
        () -> build("holding-register:1:Struct", "", 0, null));
  }

  @Test
  void detectsArrayElementAddresses() {
    assertTrue(CommandsWatcher.isArrayElementAddress("holding-register:20:INT[10][3]"));
    assertTrue(CommandsWatcher.isArrayElementAddress("holding-register:20[0..9]:INT[3]"));
    assertFalse(CommandsWatcher.isArrayElementAddress("holding-register:20:INT[10]"));
    assertFalse(CommandsWatcher.isArrayElementAddress("holding-register:20:INT"));
  }
  @Test
  void standbyAndRetiredConnectionsNeverCancelCommands() {
    CommandsWatcher watcher = new CommandsWatcher(null, java.util.List.of()) {
      @Override void commandCancel(org.bson.types.ObjectId id, String reason) {
        throw new AssertionError("inactive command was cancelled");
      }
    };
    ProtocolConnection pc = new ProtocolConnection();
    RedundancyManager.isActive = false;
    watcher.processCommand(pc, new org.bson.Document());
    RedundancyManager.isActive = true;
    pc.retired = true;
    try {
      watcher.processCommand(pc, new org.bson.Document());
    } finally {
      RedundancyManager.isActive = false;
    }
  }

  @Test
  void nonFiniteCommandsAreRejected() {
    for (String type : new String[] {"BOOL", "INT", "REAL", "LREAL"}) {
      assertThrows(UnsupportedOperationException.class,
          () -> build("x:" + type, "", Double.NaN, null));
      assertThrows(UnsupportedOperationException.class,
          () -> build("x:" + type, "", Double.POSITIVE_INFINITY, null));
    }
  }
}
