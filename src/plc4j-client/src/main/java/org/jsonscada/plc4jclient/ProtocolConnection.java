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

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.atomic.AtomicBoolean;
import org.apache.plc4x.java.api.PlcConnection;
import org.bson.Document;

/** A protocolConnections document plus the runtime state of the PLC connection. */
public class ProtocolConnection {
  // configuration (from MongoDB)
  public String protocolDriver = "";
  public int protocolDriverInstanceNumber = 1;
  public int protocolConnectionNumber = 0;
  public String name = "";
  public String description = "";
  public boolean enabled = false;
  public boolean commandsEnabled = false;
  public boolean autoCreateTags = false;
  public List<String> endpointURLs = new ArrayList<>();
  public List<String> topics = new ArrayList<>();
  public double giInterval = 0;

  // runtime state
  public volatile PlcConnection plcConn;
  public volatile String addrSeparator = ":";
  public final Map<String, String> endiannessByTag = new ConcurrentHashMap<>();
  /** Element count of array addresses, by configured address (absent for scalars). */
  public final Map<String, Integer> arrayLengthByTag = new ConcurrentHashMap<>();
  /** Last _id handed out in the connection's auto-tag key range (long: no int overflow). */
  public long autoKeyId = 0;
  public int reconnectCount = 0;
  /** True while a connection attempt is running (at most one per connection). */
  public final AtomicBoolean connecting = new AtomicBoolean(false);
  /** Earliest time (epoch ms) of the next connection attempt after a failure. */
  public volatile long nextAttemptMs = 0;
  /** Set when the configuration was reloaded: this object must not be used anymore. */
  public volatile boolean retired = false;

  public static ProtocolConnection fromDocument(Document d) {
    ProtocolConnection pc = new ProtocolConnection();
    pc.protocolDriver = d.get("protocolDriver") instanceof String s ? s : "";
    pc.protocolDriverInstanceNumber = intVal(d, "protocolDriverInstanceNumber", 1);
    pc.protocolConnectionNumber = intVal(d, "protocolConnectionNumber", 0);
    pc.name = d.get("name") instanceof String s ? s : "";
    pc.description = d.get("description") instanceof String s ? s : "";
    pc.enabled = Boolean.TRUE.equals(d.getBoolean("enabled"));
    pc.commandsEnabled = Boolean.TRUE.equals(d.getBoolean("commandsEnabled"));
    pc.autoCreateTags = Boolean.TRUE.equals(d.getBoolean("autoCreateTags"));
    pc.endpointURLs = stringList(d, "endpointURLs");
    pc.topics = stringList(d, "topics");
    pc.giInterval = dblVal(d, "giInterval", 0);
    return pc;
  }

  static int intVal(Document d, String key, int def) {
    Object v = d.get(key);
    return v instanceof Number n ? n.intValue() : def;
  }

  static double dblVal(Document d, String key, double def) {
    Object v = d.get(key);
    return v instanceof Number n ? n.doubleValue() : def;
  }

  static List<String> stringList(Document d, String key) {
    List<String> out = new ArrayList<>();
    Object v = d.get(key);
    if (v instanceof List<?> list) {
      for (Object o : list) {
        if (o instanceof String s) {
          out.add(s);
        }
      }
    }
    return out;
  }

  /** Closes the current PLC connection ignoring errors and clears the reference. */
  public void closeQuietly() {
    close(plcConn);
  }

  /**
   * Closes one specific PLC connection ignoring errors; the reference is cleared
   * only when it still points to that connection, so a stale scan thread can
   * never drop a newer connection.
   */
  public synchronized void close(PlcConnection c) {
    if (c == null) {
      return;
    }
    if (plcConn == c) {
      plcConn = null;
    }
    try {
      c.close();
    } catch (Exception e) {
      // ignore errors on close
    }
  }

  /** Marks the object as obsolete and closes its PLC connection. */
  public synchronized void retire() {
    retired = true;
    closeQuietly();
  }
}
