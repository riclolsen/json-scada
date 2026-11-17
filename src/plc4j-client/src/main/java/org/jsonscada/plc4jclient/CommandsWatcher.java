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

import com.mongodb.client.MongoCollection;
import com.mongodb.client.MongoCursor;
import com.mongodb.client.model.Aggregates;
import com.mongodb.client.model.Filters;
import com.mongodb.client.model.changestream.ChangeStreamDocument;
import com.mongodb.client.model.changestream.FullDocument;
import java.math.BigDecimal;
import java.math.BigInteger;
import java.util.Date;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.RejectedExecutionException;
import java.util.concurrent.TimeUnit;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import org.apache.plc4x.java.api.PlcConnection;
import org.apache.plc4x.java.api.messages.PlcWriteRequest;
import org.apache.plc4x.java.api.messages.PlcWriteResponse;
import org.apache.plc4x.java.api.types.PlcResponseCode;
import org.bson.Document;
import org.bson.types.ObjectId;

/**
 * Watches commandsQueue inserts via change stream and forwards commands to the
 * PLC: 10 s expiry, write value type parsed from the object address, endianness
 * pre-swap from protocolSourceASDU, delivered/cancel bookkeeping on the command
 * document. Only the active node handles commands. Commands of one connection
 * run in order on their own thread, so a slow device does not delay (and expire)
 * the commands of the other connections. The change stream is reopened after
 * errors until {@link #stop()} is called.
 */
public class CommandsWatcher implements Runnable {

  private static final long WRITE_TIMEOUT_SECONDS = 15;
  private static final long RESTART_DELAY_MS = 5000;
  private static final Pattern ELEMENT_SUFFIX = Pattern.compile("^(.*)\\[\\d+\\]$");

  private final MongoCollection<Document> commandsColl;
  private final List<ProtocolConnection> protocolConns;
  private final Map<Integer, ExecutorService> executors = new ConcurrentHashMap<>();
  private volatile boolean stopped = false;

  public CommandsWatcher(
      MongoCollection<Document> commandsColl, List<ProtocolConnection> protocolConns) {
    this.commandsColl = commandsColl;
    this.protocolConns = protocolConns;
  }

  /** Ends the watcher (the Mongo client must be closed too, to unblock the cursor). */
  public void stop() {
    stopped = true;
    for (ExecutorService ex : executors.values()) {
      ex.shutdownNow();
    }
  }

  @Override
  public void run() {
    while (!stopped) {
      try (MongoCursor<ChangeStreamDocument<Document>> cursor = commandsColl
          .watch(List.of(Aggregates.match(Filters.eq("operationType", "insert"))))
          .fullDocument(FullDocument.UPDATE_LOOKUP)
          .iterator()) {
        while (!stopped && cursor.hasNext()) {
          ChangeStreamDocument<Document> change = cursor.next();
          Document cmd = change.getFullDocument();
          if (cmd != null) {
            dispatch(cmd);
          }
        }
      } catch (Exception e) {
        if (!stopped) {
          Log.log("Commands - " + e);
        }
      }
      if (!stopped) {
        Log.log("Commands - change stream ended, reopening...");
        RedundancyManager.sleepMs(RESTART_DELAY_MS);
      }
    }
    Log.log("Commands - Exit change stream monitoring!");
  }

  private void dispatch(Document cmd) {
    int connNumber = ProtocolConnection.intVal(cmd, "protocolSourceConnectionNumber", -1);
    for (ProtocolConnection pc : protocolConns) {
      if (connNumber != pc.protocolConnectionNumber) {
        continue;
      }
      if (!RedundancyManager.isActive) {
        // the active node delivers it; cancelling here would clobber its result
        return;
      }
      try {
        executors
            .computeIfAbsent(connNumber, k -> Executors.newSingleThreadExecutor(r -> {
              Thread t = new Thread(r, "commands-conn-" + k);
              t.setDaemon(true);
              return t;
            }))
            .execute(() -> {
              try {
                processCommand(pc, cmd);
              } catch (Exception e) {
                Log.log("Commands - error processing command - " + e);
              }
            });
      } catch (RejectedExecutionException e) {
        // watcher stopped
      }
      return;
    }
  }

  void processCommand(ProtocolConnection pc, Document cmd) {
    if (stopped || pc.retired || !RedundancyManager.isActive) {
      return;
    }
    int connNumber = pc.protocolConnectionNumber;
    ObjectId id = cmd.getObjectId("_id");
    String tag = cmd.getString("tag");
    double value = ProtocolConnection.dblVal(cmd, "value", 0);
    Log.log("Commands - Command received on connection " + connNumber + ", " + tag + " "
        + value);

    // test for time expired, if too old command (> 10s) then cancel it
    Date timeTag = cmd.get("timeTag") instanceof Date dt ? dt : null;
    if (timeTag == null || System.currentTimeMillis() - timeTag.getTime() > 10_000) {
      Log.log("Commands - Command expired ");
      commandCancel(id, "expired");
      return;
    }

    if (stopped || pc.retired || !RedundancyManager.isActive) {
      return;
    }

    if (!pc.commandsEnabled) {
      Log.log("Commands - Commands disabled on connection " + pc.name);
      commandCancel(id, "commands disabled");
      return;
    }

    PlcConnection conn = pc.plcConn;
    if (conn == null || !conn.isConnected()) {
      Log.log("Commands - PLC not connected on " + pc.name);
      commandCancel(id, "PLC not connected");
      return;
    }

    String address = cmd.getString("protocolSourceObjectAddress");
    if (address == null || address.isEmpty()) {
      commandCancel(id, "no object address");
      return;
    }
    if (isArrayElementAddress(address)) {
      Log.log("Commands - Command canceled! " + id + " array element address " + address);
      commandCancel(id, "commands on array elements are not supported");
      return;
    }
    String asdu = cmd.getString("protocolSourceASDU");
    String valueString = cmd.getString("valueString");

    Object writeValue;
    try {
      writeValue = buildWriteValue(pc, address, asdu, value, valueString);
    } catch (UnsupportedOperationException e) {
      commandCancel(id, e.getMessage());
      Log.log("Commands - Command canceled! " + id + " " + e.getMessage());
      return;
    }

    try {
      // tag name stays the configured address; the address itself may need the
      // 1.0.0 array notation (see TopicParser.toPlc4xAddress)
      PlcWriteRequest wrReq = conn.writeRequestBuilder()
          .addTagAddress(address, TopicParser.toPlc4xAddress(address), writeValue)
          .build();
      if (stopped || pc.retired || !RedundancyManager.isActive || pc.plcConn != conn) {
        return;
      }
      var future = wrReq.execute();
      PlcWriteResponse wrResp;
      try {
        wrResp = future.get(WRITE_TIMEOUT_SECONDS, TimeUnit.SECONDS);
      } catch (InterruptedException e) {
        future.cancel(true);
        Thread.currentThread().interrupt();
        return;
      } catch (java.util.concurrent.TimeoutException e) {
        future.cancel(true);
        throw e;
      }
      PlcResponseCode rc = wrResp.getResponseCode(address);
      if (rc != PlcResponseCode.OK) {
        commandCancel(id, rc != null ? rc.name() : "write error");
        Log.log("Commands - Command error executing! " + id + " " + rc);
        return;
      }
    } catch (Exception e) {
      commandCancel(id, e.getMessage() != null ? e.getMessage() : e.toString());
      Log.log("Commands - Command error executing! " + id + " " + e);
      return;
    }

    Log.log("Commands - Command executed successfully! " + id);
    commandDelivered(id);
  }

  /**
   * True for "ARRAY_ADDRESS[i]", the key of one element of an array topic: the
   * element has no address of its own that PLC4X can write to.
   */
  static boolean isArrayElementAddress(String address) {
    Matcher m = ELEMENT_SUFFIX.matcher(address);
    return m.matches() && TopicParser.arrayLength(m.group(1)) != 0;
  }

  private static void requireRange(double value, double min, double max, String type) {
    if (Double.isNaN(value) || value < min || value > max) {
      throw new UnsupportedOperationException("value out of range for " + type);
    }
  }

  /**
   * Chooses the Java value to write based on the data type present in the
   * PLC4X address (after the address separator), applying the endianness
   * pre-swap requested in protocolSourceASDU. Values outside the range of the
   * data type are refused (UnsupportedOperationException) instead of wrapping.
   */
  static Object buildWriteValue(
      ProtocolConnection pc, String address, String asdu, double value, String valueString) {
    String addr = address.toUpperCase(Locale.ROOT);
    String sep = pc.addrSeparator;
    String endian = asdu != null ? asdu.toUpperCase(Locale.ROOT).trim() : "";
    boolean swap = EndianUtils.shouldSwap(endian);

    if (!Double.isFinite(value) && !TopicParser.inferType(address, sep).equals("string")) {
      throw new UnsupportedOperationException("non-finite command value");
    }
    if (addr.contains(sep + "BOOL")) {
      return value != 0;
    }
    if (addr.contains(sep + "BYTE") || addr.contains(sep + "USINT")
        || addr.contains(sep + "SUINT")) {
      requireRange(value, 0, 255, "BYTE/USINT");
      return (short) ((long) value & 0xFF);
    }
    if (addr.contains(sep + "SINT")) {
      requireRange(value, -128, 127, "SINT");
      return (byte) value;
    }
    if (addr.contains(sep + "UINT") || addr.contains(sep + "WORD")) {
      requireRange(value, 0, 65535, "UINT/WORD");
      int u16 = (int) value & 0xFFFF;
      if (swap) {
        u16 = EndianUtils.swapU16(u16);
      }
      return u16;
    }
    if (addr.contains(sep + "INT")) {
      requireRange(value, Short.MIN_VALUE, Short.MAX_VALUE, "INT");
      short s = (short) value;
      if (swap) {
        s = EndianUtils.swap(s);
      }
      return s;
    }
    if (addr.contains(sep + "UDINT") || addr.contains(sep + "DWORD")) {
      requireRange(value, 0, 4294967295.0, "UDINT/DWORD");
      long u32 = (long) value & 0xFFFFFFFFL;
      if (swap) {
        u32 = EndianUtils.swapU32(u32);
      }
      return u32;
    }
    if (addr.contains(sep + "DINT")) {
      requireRange(value, Integer.MIN_VALUE, Integer.MAX_VALUE, "DINT");
      int i32 = (int) value;
      if (swap) {
        i32 = EndianUtils.swap(i32);
      }
      return i32;
    }
    if (addr.contains(sep + "ULINT") || addr.contains(sep + "LWORD")) {
      // 2^64 is the first value out of range (exactly representable as double)
      if (Double.isNaN(value) || value < 0 || value >= 18446744073709551616.0) {
        throw new UnsupportedOperationException("value out of range for ULINT/LWORD");
      }
      BigInteger bi = new BigDecimal(value).toBigInteger();
      if (swap) {
        bi = new BigInteger(Long.toUnsignedString(EndianUtils.swap(bi.longValue())));
      }
      return bi;
    }
    if (addr.contains(sep + "LINT")) {
      // 2^63 is the first value out of range (exactly representable as double)
      if (Double.isNaN(value) || value < -9223372036854775808.0
          || value >= 9223372036854775808.0) {
        throw new UnsupportedOperationException("value out of range for LINT");
      }
      long l = (long) value;
      if (swap) {
        l = EndianUtils.swap(l);
      }
      return l;
    }
    if (addr.contains(sep + "REAL")) {
      if (!Double.isNaN(value) && !Double.isInfinite(value)
          && Math.abs(value) > Float.MAX_VALUE) {
        throw new UnsupportedOperationException("value out of range for REAL");
      }
      float f = (float) value;
      if (swap) {
        f = EndianUtils.swap(f);
      }
      return f;
    }
    if (addr.contains(sep + "LREAL")) {
      double d = value;
      if (swap) {
        d = EndianUtils.swap(d);
      }
      return d;
    }
    if (addr.contains(sep + "STRING") || addr.contains(sep + "CHAR")
        || addr.contains(sep + "WSTRING") || addr.contains(sep + "WCHAR")) {
      return valueString != null ? valueString : "";
    }
    if (addr.contains(sep + "STRUCT") || addr.contains(sep + "LIST")
        || addr.contains(sep + "RAW_BYTE_ARRAY")) {
      throw new UnsupportedOperationException("unsupported value type for command");
    }
    return value;
  }

  void commandCancel(ObjectId id, String cancelReason) {
    try {
      commandsColl.updateOne(
          Filters.eq("_id", id),
          new Document("$set", new Document("cancelReason", cancelReason)));
    } catch (Exception e) {
      Log.log(e.toString());
      Log.log("Mongodb - Can not write update to command on mongo!");
    }
  }

  void commandDelivered(ObjectId id) {
    try {
      commandsColl.updateOne(
          Filters.eq("_id", id),
          new Document("$set", new Document()
              .append("delivered", true)
              .append("ack", true)
              .append("ackTimeTag", new Date())));
    } catch (Exception e) {
      Log.log(e.toString());
      Log.log("Mongodb - Can not write update to command on mongo!");
    }
  }
}
