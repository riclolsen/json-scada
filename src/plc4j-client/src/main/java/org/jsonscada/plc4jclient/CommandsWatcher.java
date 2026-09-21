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
import java.math.BigInteger;
import java.util.Date;
import java.util.List;
import java.util.Locale;
import java.util.concurrent.TimeUnit;
import org.apache.plc4x.java.api.PlcConnection;
import org.apache.plc4x.java.api.messages.PlcWriteRequest;
import org.apache.plc4x.java.api.messages.PlcWriteResponse;
import org.apache.plc4x.java.api.types.PlcResponseCode;
import org.bson.Document;
import org.bson.types.ObjectId;

/**
 * Watches commandsQueue inserts via change stream and forwards commands to the
 * PLC, same semantics as the Go iterateCommandsChangeStream(): 10 s expiry,
 * write value type parsed from the object address, endianness pre-swap from
 * protocolSourceASDU, delivered/cancel bookkeeping on the command document.
 */
public class CommandsWatcher implements Runnable {

  private static final long WRITE_TIMEOUT_SECONDS = 15;

  private final MongoCollection<Document> commandsColl;
  private final List<ProtocolConnection> protocolConns;

  public CommandsWatcher(
      MongoCollection<Document> commandsColl, List<ProtocolConnection> protocolConns) {
    this.commandsColl = commandsColl;
    this.protocolConns = protocolConns;
  }

  @Override
  public void run() {
    try (MongoCursor<ChangeStreamDocument<Document>> cursor = commandsColl
        .watch(List.of(Aggregates.match(Filters.eq("operationType", "insert"))))
        .fullDocument(FullDocument.UPDATE_LOOKUP)
        .iterator()) {
      while (cursor.hasNext()) {
        ChangeStreamDocument<Document> change = cursor.next();
        Document cmd = change.getFullDocument();
        if (cmd != null) {
          try {
            processCommand(cmd);
          } catch (Exception e) {
            Log.log("Commands - error processing command - " + e);
          }
        }
      }
    } catch (Exception e) {
      Log.log("Commands - " + e);
    }
    Log.log("Commands - Exit change stream monitoring!");
  }

  void processCommand(Document cmd) {
    int connNumber = ProtocolConnection.intVal(cmd, "protocolSourceConnectionNumber", -1);
    for (ProtocolConnection pc : protocolConns) {
      if (connNumber != pc.protocolConnectionNumber) {
        continue;
      }
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
        break;
      }

      if (!pc.commandsEnabled) {
        Log.log("Commands - Commands disabled on connection " + pc.name);
        commandCancel(id, "commands disabled");
        break;
      }

      PlcConnection conn = pc.plcConn;
      if (conn == null || !conn.isConnected()) {
        Log.log("Commands - PLC not connected on " + pc.name);
        commandCancel(id, "PLC not connected");
        break;
      }

      String address = cmd.getString("protocolSourceObjectAddress");
      if (address == null || address.isEmpty()) {
        commandCancel(id, "no object address");
        break;
      }
      String asdu = cmd.getString("protocolSourceASDU");
      String valueString = cmd.getString("valueString");

      Object writeValue;
      try {
        writeValue = buildWriteValue(pc, address, asdu, value, valueString);
      } catch (UnsupportedOperationException e) {
        commandCancel(id, e.getMessage());
        Log.log("Commands - Command canceled! " + id + " " + e.getMessage());
        break;
      }

      try {
        // tag name stays the configured address; the address itself may need the
        // 1.0.0 array notation (see TopicParser.toPlc4xAddress)
        PlcWriteRequest wrReq = conn.writeRequestBuilder()
            .addTagAddress(address, TopicParser.toPlc4xAddress(address), writeValue)
            .build();
        PlcWriteResponse wrResp =
            wrReq.execute().get(WRITE_TIMEOUT_SECONDS, TimeUnit.SECONDS);
        PlcResponseCode rc = wrResp.getResponseCode(address);
        if (rc != PlcResponseCode.OK) {
          commandCancel(id, rc != null ? rc.name() : "write error");
          Log.log("Commands - Command error executing! " + id + " " + rc);
          break;
        }
      } catch (Exception e) {
        commandCancel(id, e.getMessage() != null ? e.getMessage() : e.toString());
        Log.log("Commands - Command error executing! " + id + " " + e);
        break;
      }

      Log.log("Commands - Command executed successfully! " + id);
      commandDelivered(id);
      break;
    }
  }

  /**
   * Chooses the Java value to write based on the data type present in the
   * PLC4X address (after the address separator), applying the endianness
   * pre-swap requested in protocolSourceASDU.
   */
  static Object buildWriteValue(
      ProtocolConnection pc, String address, String asdu, double value, String valueString) {
    String addr = address.toUpperCase(Locale.ROOT);
    String sep = pc.addrSeparator;
    String endian = asdu != null ? asdu.toUpperCase(Locale.ROOT).trim() : "";
    boolean swap = EndianUtils.shouldSwap(endian);

    if (addr.contains(sep + "BOOL")) {
      return value != 0;
    }
    if (addr.contains(sep + "BYTE")) {
      return (short) ((long) value & 0xFF);
    }
    if (addr.contains(sep + "USINT") || addr.contains(sep + "SUINT")) {
      return (short) ((long) value & 0xFF);
    }
    if (addr.contains(sep + "SINT")) {
      return (byte) value;
    }
    if (addr.contains(sep + "UINT") || addr.contains(sep + "WORD")
        && !addr.contains(sep + "DWORD") && !addr.contains(sep + "LWORD")) {
      int u16 = (int) ((long) value & 0xFFFF);
      if (swap) {
        u16 = EndianUtils.swapU16(u16);
      }
      return u16;
    }
    if (addr.contains(sep + "INT")) {
      short s = (short) value;
      if (swap) {
        s = EndianUtils.swap(s);
      }
      return s;
    }
    if (addr.contains(sep + "UDINT") || addr.contains(sep + "DWORD")) {
      long u32 = (long) value & 0xFFFFFFFFL;
      if (swap) {
        u32 = EndianUtils.swapU32(u32);
      }
      return u32;
    }
    if (addr.contains(sep + "DINT")) {
      int i32 = (int) value;
      if (swap) {
        i32 = EndianUtils.swap(i32);
      }
      return i32;
    }
    if (addr.contains(sep + "ULINT") || addr.contains(sep + "LWORD")) {
      long u64 = (long) value;
      if (swap) {
        u64 = EndianUtils.swap(u64);
      }
      return new BigInteger(Long.toUnsignedString(u64));
    }
    if (addr.contains(sep + "LINT")) {
      long l = (long) value;
      if (swap) {
        l = EndianUtils.swap(l);
      }
      return l;
    }
    if (addr.contains(sep + "REAL") && !addr.contains(sep + "LREAL")) {
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
        || addr.contains(sep + "WCHAR")) {
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
