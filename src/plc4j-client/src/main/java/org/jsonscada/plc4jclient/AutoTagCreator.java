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

import com.mongodb.MongoWriteException;
import com.mongodb.client.MongoCollection;
import com.mongodb.client.model.Filters;
import com.mongodb.client.model.Sorts;
import com.mongodb.client.model.UpdateOptions;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;
import org.bson.Document;

/**
 * Automatic creation of realtimeData tags:
 * _id key space (connNumber*1e6, (connNumber+1)*1e6), duplicate-key handling,
 * skip when the address is already configured on the connection.
 */
public class AutoTagCreator {

  // should be more than estimated maximum points on a connection
  public static final int AUTO_KEY_MULTIPLIER = 1_000_000;

  private final Set<String> createdTags = ConcurrentHashMap.newKeySet();

  /**
   * Positions the connection auto key at the highest _id already used in the
   * connection's key range.
   */
  public void getAutoKeyInitialValue(MongoCollection<Document> rtData, ProtocolConnection pc) {
    pc.autoKeyId = (long) pc.protocolConnectionNumber * AUTO_KEY_MULTIPLIER;
    try {
      Document res = rtData
          .find(Filters.and(
              Filters.gt("_id", pc.autoKeyId),
              Filters.lt("_id", ((long) pc.protocolConnectionNumber + 1) * AUTO_KEY_MULTIPLIER)))
          .sort(Sorts.descending("_id"))
          .limit(1)
          .first();
      if (res != null && res.get("_id") instanceof Number n && n.longValue() > pc.autoKeyId) {
        pc.autoKeyId = n.longValue();
      }
    } catch (Exception e) {
      Log.log("Mongodb - Error querying max auto key for connection " + pc.name + " - " + e);
    }
  }

  /**
   * Creates the tag if it does not exist yet. The tagDoc must come from
   * RtDataTagDefaults.newTagDocument() with tag/type/address/groups already set.
   */
  public void autoCreateTag(
      Document tagDoc, MongoCollection<Document> rtData, ProtocolConnection pc) {
    String tag = tagDoc.getString("tag");
    String objectAddress = tagDoc.getString("protocolSourceObjectAddress");
    if (tag == null || tag.isEmpty()) {
      tag = Plc4jClient.DRIVER_NAME + "." + pc.protocolConnectionNumber + "." + objectAddress;
      tagDoc.put("tag", tag);
    }

    if (createdTags.contains(tag)) {
      return;
    }

    // try to find it, only create a new tag when not found
    try {
      Document existing = rtData
          .find(Filters.and(
              Filters.eq("protocolSourceObjectAddress", objectAddress),
              Filters.eq("protocolSourceConnectionNumber",
                  (double) pc.protocolConnectionNumber)))
          .first();
      if (existing != null) {
        createdTags.add(tag);
        return;
      }
    } catch (Exception e) {
      Log.log("Mongodb - Error querying tag - " + tag + " - " + e);
      return;
    }

    tagDoc.put("description", tag);
    tagDoc.put("ungroupedDescription", objectAddress);
    tagDoc.put("eventTextFalse", "OFF");
    tagDoc.put("eventTextTrue", "ON");
    tagDoc.put("stateTextFalse", "OFF");
    tagDoc.put("stateTextTrue", "ON");
    tagDoc.put("origin", "supervised");
    tagDoc.put("unit", "");

    long keyLimit = ((long) pc.protocolConnectionNumber + 1) * AUTO_KEY_MULTIPLIER;
    while (true) {
      pc.autoKeyId++;
      if (pc.autoKeyId >= keyLimit) {
        Log.log("Mongodb: auto tag key range exhausted for connection " + pc.name
            + ", can not create tag - " + tag);
        return;
      }
      tagDoc.put("_id", (double) pc.autoKeyId);
      try {
        rtData.insertOne(tagDoc);
        Log.log("Mongodb: new tag inserted - " + tag);
        break;
      } catch (MongoWriteException e) {
        String msg = e.getMessage() != null ? e.getMessage() : "";
        boolean dup = e.getError() != null && e.getError().getCode() == 11000;
        if (dup && msg.contains("_id")) {
          // duplicate _id: increment key and retry
          Log.log("Mongodb: duplicated _id while inserting new tag - " + tag + " "
              + tagDoc.get("_id"));
          continue;
        }
        if (dup && msg.contains("tag")) {
          // existing tag name: update its object address instead
          Log.log("Mongodb: duplicated tag while inserting new tag - " + tag + " "
              + tagDoc.get("_id"));
          try {
            rtData.updateOne(
                Filters.and(
                    Filters.eq("tag", tag),
                    Filters.eq("protocolSourceConnectionNumber",
                        (double) pc.protocolConnectionNumber)),
                new Document("$set", new Document()
                    .append("protocolSourceObjectAddress", objectAddress)
                    .append("origin", "supervised")),
                new UpdateOptions().upsert(true));
          } catch (Exception e2) {
            Log.log("Mongodb: Error updating tag - " + tag + " - " + e2);
            return;
          }
          Log.log("Mongodb: updated tag - " + tag + " protocolSourceObjectAddress="
              + objectAddress);
          break;
        }
        Log.log("Mongodb: error while inserting new tag - " + tag + " " + tagDoc.get("_id")
            + " - " + e);
        return;
      } catch (Exception e) {
        Log.log("Mongodb: error while inserting new tag - " + tag + " " + tagDoc.get("_id")
            + " - " + e);
        return;
      }
    }
    createdTags.add(tag);
  }
}
