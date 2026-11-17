package org.jsonscada.plc4jclient;

import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertSame;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.lang.reflect.Proxy;
import java.util.concurrent.atomic.AtomicInteger;
import org.apache.plc4x.java.api.PlcConnection;
import org.junit.jupiter.api.Test;

class ProtocolConnectionTest {

  private static PlcConnection fake(AtomicInteger closes) {
    return (PlcConnection) Proxy.newProxyInstance(
        PlcConnection.class.getClassLoader(), new Class<?>[] {PlcConnection.class},
        (proxy, method, args) -> {
          if (method.getName().equals("close")) {
            closes.incrementAndGet();
          }
          return method.getReturnType() == boolean.class ? Boolean.TRUE : null;
        });
  }

  @Test
  void closingAStaleConnectionKeepsTheNewOne() {
    AtomicInteger closes = new AtomicInteger();
    ProtocolConnection pc = new ProtocolConnection();
    PlcConnection oldConn = fake(closes);
    PlcConnection newConn = fake(closes);
    pc.plcConn = newConn;
    pc.close(oldConn);
    assertSame(newConn, pc.plcConn);
    assertTrue(closes.get() == 1);
  }

  @Test
  void retireClosesAndMarks() {
    AtomicInteger closes = new AtomicInteger();
    ProtocolConnection pc = new ProtocolConnection();
    pc.plcConn = fake(closes);
    pc.retire();
    assertNull(pc.plcConn);
    assertTrue(pc.retired);
    assertTrue(closes.get() == 1);
  }
}
