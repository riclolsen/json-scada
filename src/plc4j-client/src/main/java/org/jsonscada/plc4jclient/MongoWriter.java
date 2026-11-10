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

import com.mongodb.bulk.BulkWriteResult;
import com.mongodb.client.MongoCollection;
import com.mongodb.client.model.BulkWriteOptions;
import com.mongodb.client.model.Filters;
import com.mongodb.client.model.WriteModel;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.BlockingQueue;
import java.util.concurrent.LinkedBlockingQueue;
import org.bson.Document;
import org.bson.types.ObjectId;

/**
 * Owns the MongoDB lifecycle (like the Go mongoWriter goroutine): connects with
 * retry, loads the instance and connections configuration, starts the commands
 * change-stream watcher and consumes the bulk-write queue. On any Mongo failure
 * the whole layer reconnects and reconfigures.
 */
public class MongoWriter implements Runnable {

  public final BlockingQueue<List<WriteModel<Document>>> queue = new LinkedBlockingQueue<>(1000);

  // state published to the main supervision loop
  public volatile List<ProtocolConnection> protocolConns;
  public volatile MongoCollection<Document> rtDataColl;
  public volatile MongoCollection<Document> instancesColl;
  public volatile MongoCollection<Document> connectionsColl;
  public volatile ObjectId instanceId;

  private final ConfigData cfg;
  private final int instanceNumber;

  public MongoWriter(ConfigData cfg, int instanceNumber) {
    this.cfg = cfg;
    this.instanceNumber = instanceNumber;
  }

  @Override
  public void run() {
    while (true) {
      MongoConnector mongo = null;
      try {
        Log.log("Mongodb - Try to connect server...");
        mongo = MongoConnector.connect(cfg);
        Log.log("Mongodb - Connected to server.");

        // loads config and sets this.instanceId, then publish state for the main loop
        List<ProtocolConnection> conns = configInstance(mongo);
        this.protocolConns = conns;
        this.instancesColl = mongo.instances;
        this.connectionsColl = mongo.connections;
        this.rtDataColl = mongo.rtData;

        // start the commands change-stream watcher for this Mongo session
        Thread cmdThread =
            new Thread(new CommandsWatcher(mongo.commands, conns), "commands-watcher");
        cmdThread.setDaemon(true);
        cmdThread.start();

        // consume the bulk-write queue until a Mongo error occurs
        while (true) {
          List<WriteModel<Document>> ops = queue.take();
          if (ops.isEmpty()) {
            continue;
          }
          BulkWriteResult res = mongo.rtData.bulkWrite(ops, new BulkWriteOptions().ordered(false));
          if (Log.level >= Log.LEVEL_DETAILED) {
            Log.log("Mongodb - Opers: " + ops.size() + ", Matched count: "
                + res.getMatchedCount() + ", Updated Count: " + res.getModifiedCount());
          }
        }
      } catch (InterruptedException e) {
        Thread.currentThread().interrupt();
        return;
      } catch (Exception e) {
        Log.log("Mongodb - error!");
        Log.log(e.toString());
      } finally {
        if (mongo != null) {
          mongo.close();
        }
      }
      RedundancyManager.sleepMs(10000);
    }
  }

  /** Loads instance and connections config; exits the process if no instance found. */
  private List<ProtocolConnection> configInstance(MongoConnector mongo) {
    Document instance = mongo.instances
        .find(Filters.and(
            Filters.eq("protocolDriver", Plc4jClient.DRIVER_NAME),
            Filters.eq("protocolDriverInstanceNumber", instanceNumber),
            Filters.eq("enabled", true)))
        .first();
    if (instance == null || !(instance.get("protocolDriver") instanceof String)) {
      Log.log("No driver instance found on configuration! Driver Name: "
          + Plc4jClient.DRIVER_NAME + " Instance number: " + instanceNumber);
      System.exit(1);
    }
    this.instanceId = instance.getObjectId("_id");

    List<ProtocolConnection> conns = new ArrayList<>();
    for (Document d : mongo.connections.find(Filters.and(
        Filters.eq("protocolDriver", Plc4jClient.DRIVER_NAME),
        Filters.eq("protocolDriverInstanceNumber", instanceNumber),
        Filters.eq("enabled", true)))) {
      conns.add(ProtocolConnection.fromDocument(d));
    }
    return conns;
  }
}
