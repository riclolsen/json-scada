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
import com.mongodb.client.model.Filters;
import java.util.Date;
import java.util.List;
import java.util.Objects;
import org.bson.Document;
import org.bson.types.ObjectId;

/**
 * Redundancy control over the protocolDriverInstances document, state
 * machine of the other JSON-SCADA drivers: the active node updates the keep-alive;
 * the inactive node takes over after observing more than LIMIT consecutive polls
 * with an unchanged keep-alive.
 */
public class RedundancyManager {

  public static volatile boolean isActive = false;

  private static final int COUNT_KEEP_ALIVE_UPDATES_LIMIT = 4;

  private final ConfigData cfg;
  private int countKeepAliveUpdates = 0;
  private Date lastActiveNodeKeepAliveTimeTag = null;

  public RedundancyManager(ConfigData cfg) {
    this.cfg = cfg;
  }

  public void process(MongoCollection<Document> instancesColl, ObjectId instanceId) {
    Document instance;
    try {
      instance = instancesColl.find(Filters.eq("_id", instanceId)).first();
    } catch (Exception e) {
      isActive = false;
      Log.log("Redundancy - Error querying protocolDriverInstances!");
      Log.log(e.toString());
      return;
    }
    if (instance == null || !(instance.get("protocolDriver") instanceof String)) {
      isActive = false;
      Log.log("Redundancy - No driver instance found!");
      return;
    }

    List<String> nodeNames = null;
    try {
      nodeNames = instance.getList("nodeNames", String.class);
    } catch (Exception e) {
      // tolerate malformed nodeNames
    }
    if (nodeNames != null && !nodeNames.isEmpty() && !containsTrimmed(nodeNames, cfg.nodeName)) {
      Log.fatal("Redundancy - This node name not in the list of nodes from driver instance!");
    }

    String activeNodeName = instance.getString("activeNodeName");
    Date keepAlive = null;
    if (instance.get("activeNodeKeepAliveTimeTag") instanceof Date dt) {
      keepAlive = dt;
    }

    boolean activate = false;
    if (cfg.nodeName.equals(activeNodeName)) {
      if (!isActive) {
        Log.log("Redundancy - ACTIVATING this Node!");
      }
      activate = true;
    } else {
      if (isActive) { // was active, other node assumed, so be inactive and wait
        Log.log("Redundancy - DEACTIVATING this Node (other node active)!");
        countKeepAliveUpdates = 0;
        isActive = false;
        lastActiveNodeKeepAliveTimeTag = keepAlive;
        sleepMs(1000);
      }
      isActive = false;
      if (Objects.equals(lastActiveNodeKeepAliveTimeTag, keepAlive)) {
        countKeepAliveUpdates++;
      } else {
        // keep-alive advanced: the active node is alive, only consecutive stalls count
        countKeepAliveUpdates = 0;
      }
      lastActiveNodeKeepAliveTimeTag = keepAlive;
      if (countKeepAliveUpdates > COUNT_KEEP_ALIVE_UPDATES_LIMIT) { // time exceeded, be active
        Log.log("Redundancy - ACTIVATING this Node!");
        activate = true;
      }
    }

    if (activate) {
      if (Log.level >= Log.LEVEL_DETAILED) {
        Log.log("Redundancy - This node is active.");
      }
      try {
        // Claim only the state we observed; concurrent contenders cannot both win.
        var result = instancesColl.updateOne(
            Filters.and(Filters.eq("_id", instanceId),
                Filters.eq("activeNodeName", activeNodeName),
                Filters.eq("activeNodeKeepAliveTimeTag", keepAlive)),
            new Document("$set", new Document()
                .append("activeNodeName", cfg.nodeName)
                .append("activeNodeKeepAliveTimeTag", new Date())));
        isActive = result.getMatchedCount() != 0;
        if (!isActive) {
          countKeepAliveUpdates = 0;
        }
      } catch (Exception e) {
        isActive = false;
        Log.log(e.toString());
      }
    }
  }

  private static boolean containsTrimmed(List<String> list, String value) {
    String t = value.trim();
    for (String s : list) {
      if (s != null && t.equals(s.trim())) {
        return true;
      }
    }
    return false;
  }

  static void sleepMs(long ms) {
    try {
      Thread.sleep(ms);
    } catch (InterruptedException e) {
      Thread.currentThread().interrupt();
    }
  }
}
