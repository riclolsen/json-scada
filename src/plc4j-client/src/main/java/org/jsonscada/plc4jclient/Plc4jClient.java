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

import ch.qos.logback.classic.Level;
import com.mongodb.client.MongoCollection;
import java.util.List;
import java.util.Locale;
import java.util.ServiceLoader;
import java.util.concurrent.TimeUnit;
import org.apache.plc4x.java.api.PlcConnection;
import org.apache.plc4x.java.api.PlcConnectionFactory;
import org.apache.plc4x.java.api.PlcDriver;
import org.apache.plc4x.java.api.PlcDriverManager;
import org.apache.plc4x.java.api.messages.PlcReadRequest;
import org.bson.Document;
import org.bson.types.ObjectId;
import org.slf4j.LoggerFactory;

/**
 * PLC4J Client - Generic PLC protocol driver for {json:scada}, based on Apache
 * PLC4X for Java. Registers in MongoDB as protocolDriver "PLC4X": it is a
 * drop-in alternative executable for the Go plc4x-client, sharing the same
 * instance/connection/tag configuration. Only one of the two executables may
 * run for a given instance number.
 */
public final class Plc4jClient {

  public static final String SOFTWARE_VERSION =
      "{json:scada} PLC4J Generic PLC Protocol Driver v.0.1.0 - Copyright 2020-2026 Ricardo L. Olsen";
  public static final String DRIVER_NAME = "PLC4X";

  private static final long PING_TIMEOUT_SECONDS = 5;

  private Plc4jClient() {}

  public static void main(String[] args) {
    Log.log(SOFTWARE_VERSION);
    Log.log("Usage plc4j-client [instance number] [log level] [config file name] [point filter]");

    ConfigLoader.Result cfgRes = ConfigLoader.load(args);
    Log.level = cfgRes.logLevel;
    setLibLogLevel(cfgRes.logLevel);
    Log.log("Log level set to: " + cfgRes.logLevel);

    listRegisteredDrivers();

    MongoWriter writer = new MongoWriter(cfgRes.cfg, cfgRes.instanceNumber);
    Thread writerThread = new Thread(writer, "mongo-writer");
    writerThread.start();

    RedundancyManager redundancy = new RedundancyManager(cfgRes.cfg);
    AutoTagCreator autoTagCreator = new AutoTagCreator();
    PlcConnectionFactory connectionFactory = PlcDriverManager.getDefault().getConnectionFactory();

    // keep retrying protocol reconnection when disconnected
    while (true) {
      List<ProtocolConnection> conns = writer.protocolConns;
      MongoCollection<Document> rtDataColl = writer.rtDataColl;
      MongoCollection<Document> instancesColl = writer.instancesColl;
      ObjectId instanceId = writer.instanceId;
      if (conns == null || conns.isEmpty() || rtDataColl == null || instancesColl == null
          || instanceId == null) {
        RedundancyManager.isActive = false;
        RedundancyManager.sleepMs(1000);
        continue;
      }
      redundancy.process(instancesColl, instanceId);
      if (!RedundancyManager.isActive) {
        RedundancyManager.sleepMs(1000);
        continue;
      }
      for (ProtocolConnection pc : conns) {
        try {
          connectAndScan(pc, connectionFactory, rtDataColl, autoTagCreator, writer);
        } catch (Exception e) {
          Log.log(pc.name + ": error setting up connection - " + e);
        }
      }
      RedundancyManager.sleepMs(2000);
    }
  }

  static void connectAndScan(
      ProtocolConnection pc,
      PlcConnectionFactory connectionFactory,
      MongoCollection<Document> rtDataColl,
      AutoTagCreator autoTagCreator,
      MongoWriter writer) {

    if (pc.plcConn != null && pc.plcConn.isConnected()) {
      return;
    }
    pc.closeQuietly();

    autoTagCreator.getAutoKeyInitialValue(rtDataColl, pc);

    if (pc.endpointURLs == null || pc.endpointURLs.isEmpty()) {
      Log.fatal("No server endpoint for connection: " + pc.name);
    }

    String connUrl = pc.endpointURLs.get(pc.reconnectCount % pc.endpointURLs.size());
    pc.reconnectCount++;
    Log.log("Instance: " + pc.protocolDriverInstanceNumber + " Connection: "
        + pc.protocolConnectionNumber + " " + pc.name);
    Log.log(pc.name + ": Server endpoint URL: " + connUrl);

    String protocolId = pc.endpointURLs.get(0).split(":")[0].toLowerCase(Locale.ROOT);
    pc.addrSeparator = TopicParser.addrSeparatorForScheme(protocolId);

    // try to connect to plc
    PlcConnection connection;
    try {
      connection = connectionFactory.getConnection(connUrl);
    } catch (Exception e) {
      Log.log(pc.name + ": Error connecting to PLC: "
          + (e.getMessage() != null ? e.getMessage() : e.toString()));
      return;
    }
    pc.plcConn = connection;

    // try to ping the plc (tolerated when the driver doesn't support ping)
    if (!pingOk(pc)) {
      Log.log(pc.name + ": Couldn't ping device");
      pc.closeQuietly();
      return;
    }

    // log metadata of connection
    try {
      var md = connection.getMetadata();
      if (!md.isReadSupported()) {
        Log.log(pc.name + ": This connection doesn't support read operations");
        pc.closeQuietly();
        return;
      }
      if (!md.isWriteSupported()) {
        Log.log(pc.name + ": This connection doesn't support write operations");
      }
      if (!md.isBrowseSupported()) {
        Log.log(pc.name + ": This connection doesn't support browsing");
      }
      if (!md.isSubscribeSupported()) {
        Log.log(pc.name + ": This connection doesn't support subscriptions");
      }
    } catch (Exception e) {
      Log.log(pc.name + ": error checking connection metadata - " + e);
    }

    // build a read-request with all topics
    PlcReadRequest.Builder reqBld = connection.readRequestBuilder();
    pc.endiannessByTag.clear();
    for (String topic : pc.topics) {
      TopicParser.ParsedTopic pt = TopicParser.parse(topic, pc.addrSeparator);
      if (pt.address.isEmpty()) {
        continue;
      }
      if (pt.arrayLength < 0) {
        Log.log(pc.name + ": error parsing array number from address: " + pt.address);
        continue;
      }
      pc.endiannessByTag.put(pt.address, pt.endianness);
      // the configured address stays the PLC4X tag name (and the realtimeData
      // key); only the address handed to PLC4X may have been translated
      reqBld.addTagAddress(pt.address, pt.plc4xAddress);
      if (!pt.plc4xAddress.equals(pt.address) && Log.level >= Log.LEVEL_BASIC) {
        Log.log(pc.name + ": legacy array address '" + pt.address
            + "' sent to PLC4X as '" + pt.plc4xAddress + "'");
      }

      if (pc.autoCreateTags) {
        if (pt.arrayLength > 1) {
          for (int i = 0; i < pt.arrayLength; i++) {
            Document rtd = RtDataTagDefaults.newTagDocument();
            rtd.put("tag", pt.jsTagName.isEmpty() ? "" : pt.jsTagName + "[" + i + "]");
            rtd.put("protocolSourceConnectionNumber", (double) pc.protocolConnectionNumber);
            rtd.put("protocolSourceObjectAddress", pt.address + "[" + i + "]");
            rtd.put("protocolSourceASDU", pt.endianness);
            rtd.put("group1", DRIVER_NAME);
            rtd.put("group2", pc.name);
            rtd.put("group3", pt.address);
            rtd.put("type", pt.jsType);
            autoTagCreator.autoCreateTag(rtd, rtDataColl, pc);
            if (Log.level >= Log.LEVEL_BASIC) {
              Log.log(pc.name + ": tagName: " + rtd.getString("tag") + " address: "
                  + pt.address + "[" + i + "]");
            }
          }
        } else {
          Document rtd = RtDataTagDefaults.newTagDocument();
          rtd.put("tag", pt.jsTagName);
          rtd.put("protocolSourceConnectionNumber", (double) pc.protocolConnectionNumber);
          rtd.put("protocolSourceObjectAddress", pt.address);
          rtd.put("protocolSourceASDU", pt.endianness);
          rtd.put("group1", DRIVER_NAME);
          rtd.put("group2", pc.name);
          rtd.put("group3", pt.address);
          rtd.put("type", pt.jsType);
          autoTagCreator.autoCreateTag(rtd, rtDataColl, pc);
          if (Log.level >= Log.LEVEL_BASIC) {
            Log.log(pc.name + ": tagName: " + rtd.getString("tag") + " address: " + pt.address);
          }
        }
      } else if (Log.level >= Log.LEVEL_BASIC) {
        Log.log(pc.name + ": address: " + pt.address);
      }
    }

    try {
      pc.readRequest = reqBld.build();
    } catch (Exception e) {
      Log.log(pc.name + ": error preparing read-request: " + e);
      pc.closeQuietly();
      return;
    }

    Thread scanThread = new Thread(new ScanTask(pc, writer), pc.name + "-scan");
    scanThread.setDaemon(true);
    scanThread.start();
  }

  /**
   * Pings the PLC. Drivers that do not implement ping are treated as reachable
   * (unlike plc4go, several PLC4J drivers throw/complete exceptionally with
   * "not supported" for ping).
   */
  static boolean pingOk(ProtocolConnection pc) {
    PlcConnection conn = pc.plcConn;
    if (conn == null) {
      return false;
    }
    try {
      conn.ping().get(PING_TIMEOUT_SECONDS, TimeUnit.SECONDS);
      return true;
    } catch (UnsupportedOperationException e) {
      return true;
    } catch (Exception e) {
      String msg = e.getMessage() != null ? e.getMessage().toLowerCase(Locale.ROOT) : "";
      if (msg.contains("not supported") || msg.contains("unsupported")
          || msg.contains("not implemented")) {
        return true;
      }
      Log.log(pc.name + ": ping error - " + e);
      return false;
    }
  }

  /** Logs the PLC4X drivers available on the classpath (fat-jar self check). */
  static void listRegisteredDrivers() {
    try {
      StringBuilder sb = new StringBuilder("Registered PLC4X drivers:");
      for (PlcDriver driver : ServiceLoader.load(PlcDriver.class)) {
        sb.append(" ").append(driver.getProtocolCode());
      }
      Log.log(sb.toString());
    } catch (Throwable e) {
      Log.log("Error listing PLC4X drivers - " + e);
    }
  }

  /** Maps the driver log level to the SLF4J/logback level of the PLC4J internals. */
  static void setLibLogLevel(int logLevel) {
    try {
      ch.qos.logback.classic.Logger root = (ch.qos.logback.classic.Logger)
          LoggerFactory.getLogger(org.slf4j.Logger.ROOT_LOGGER_NAME);
      if (logLevel >= Log.LEVEL_DEBUG) {
        root.setLevel(Level.DEBUG);
      } else if (logLevel >= Log.LEVEL_DETAILED) {
        root.setLevel(Level.INFO);
      } else {
        root.setLevel(Level.WARN);
      }
    } catch (Throwable e) {
      Log.log("Error setting library log level - " + e);
    }
  }
}
