package org.jsonscada.plc4jclient;

import static org.junit.jupiter.api.Assertions.*;

import java.lang.reflect.Proxy;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;
import org.apache.plc4x.java.api.PlcConnection;
import org.apache.plc4x.java.api.messages.PlcReadRequest;
import org.apache.plc4x.java.api.messages.PlcReadResponse;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.Test;

class ScanTaskTest {
  @AfterEach
  void reset() { RedundancyManager.isActive = false; }

  @Test
  void timedOutRequestIsCancelledAndStaleTaskCannotReadReplacement() {
    CompletableFuture<PlcReadResponse> future = new CompletableFuture<>() {
      @Override
      public PlcReadResponse get(long timeout, TimeUnit unit) throws TimeoutException {
        throw new TimeoutException("device hung");
      }
    };
    PlcConnection conn = (PlcConnection) Proxy.newProxyInstance(
        PlcConnection.class.getClassLoader(), new Class<?>[] {PlcConnection.class},
        (p, m, a) -> m.getReturnType() == boolean.class ? true : null);
    PlcReadRequest request = (PlcReadRequest) Proxy.newProxyInstance(
        PlcReadRequest.class.getClassLoader(), new Class<?>[] {PlcReadRequest.class},
        (p, m, a) -> {
          if (m.getName().equals("execute")) { return future; }
          return null;
        });
    ProtocolConnection pc = new ProtocolConnection(); pc.plcConn = conn;
    RedundancyManager.isActive = true;
    ScanTask task = new ScanTask(pc, new MongoWriter(new ConfigData(), 1), conn, request);
    assertFalse(task.execRead());
    assertTrue(future.isCancelled());
    pc.plcConn = null;
    assertFalse(task.execRead());
  }
}
