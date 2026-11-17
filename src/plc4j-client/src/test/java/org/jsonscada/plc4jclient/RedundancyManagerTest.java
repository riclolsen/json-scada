package org.jsonscada.plc4jclient;

import static org.junit.jupiter.api.Assertions.*;

import com.mongodb.client.FindIterable;
import com.mongodb.client.MongoCollection;
import com.mongodb.client.result.UpdateResult;
import java.lang.reflect.Proxy;
import java.util.Date;
import java.util.concurrent.atomic.AtomicBoolean;
import org.bson.Document;
import org.bson.types.ObjectId;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.Test;

class RedundancyManagerTest {
  @AfterEach
  void reset() { RedundancyManager.isActive = false; }

  @SuppressWarnings("unchecked")
  private MongoCollection<Document> collection(Document doc, boolean matched, AtomicBoolean fail) {
    FindIterable<Document> find = (FindIterable<Document>) Proxy.newProxyInstance(
        FindIterable.class.getClassLoader(), new Class<?>[] {FindIterable.class},
        (p, m, a) -> doc);
    return (MongoCollection<Document>) Proxy.newProxyInstance(
        MongoCollection.class.getClassLoader(), new Class<?>[] {MongoCollection.class},
        (p, m, a) -> {
          if (fail.get()) { throw new IllegalStateException("offline"); }
          if (m.getName().equals("find")) { return find; }
          if (m.getName().equals("updateOne")) {
            if (matched) { doc.putAll(((Document) a[1]).get("$set", Document.class)); }
            return UpdateResult.acknowledged(matched ? 1 : 0, matched ? 1L : 0L, null);
          }
          return null;
        });
  }

  @Test
  void advancingHeartbeatResetsConsecutiveStalls() {
    ConfigData cfg = new ConfigData(); cfg.nodeName = "standby";
    RedundancyManager manager = new RedundancyManager(cfg);
    Document doc = new Document("protocolDriver", "PLC4X")
        .append("activeNodeName", "primary").append("activeNodeKeepAliveTimeTag", new Date(1));
    MongoCollection<Document> coll = collection(doc, true, new AtomicBoolean());
    for (int i = 0; i < 30; i++) {
      if (i % 2 == 0) { doc.put("activeNodeKeepAliveTimeTag", new Date(i + 1)); }
      manager.process(coll, new ObjectId());
      assertFalse(RedundancyManager.isActive);
    }
    for (int i = 0; i < 6; i++) { manager.process(coll, new ObjectId()); }
    assertTrue(RedundancyManager.isActive);
  }

  @Test
  void failedClaimCannotActivate() {
    ConfigData cfg = new ConfigData(); cfg.nodeName = "standby";
    RedundancyManager manager = new RedundancyManager(cfg);
    Document doc = new Document("protocolDriver", "PLC4X").append("activeNodeName", "primary");
    MongoCollection<Document> coll = collection(doc, false, new AtomicBoolean());
    for (int i = 0; i < 6; i++) { manager.process(coll, new ObjectId()); }
    assertFalse(RedundancyManager.isActive);
  }

  @Test
  void databaseFailureDeactivatesNode() {
    ConfigData cfg = new ConfigData();
    RedundancyManager.isActive = true;
    new RedundancyManager(cfg).process(collection(new Document(), true, new AtomicBoolean(true)), new ObjectId());
    assertFalse(RedundancyManager.isActive);
  }
}
