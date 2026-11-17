package org.jsonscada.plc4jclient;

import static org.junit.jupiter.api.Assertions.*;

import com.mongodb.client.FindIterable;
import com.mongodb.client.MongoCollection;
import java.lang.reflect.Proxy;
import java.util.concurrent.atomic.AtomicReference;
import org.bson.Document;
import org.junit.jupiter.api.Test;

class AutoTagCreatorTest {
  @Test
  @SuppressWarnings("unchecked")
  void largeConnectionNumbersAllocateDoubleIdsWithoutOverflow() {
    AtomicReference<Document> inserted = new AtomicReference<>();
    FindIterable<Document> find = (FindIterable<Document>) Proxy.newProxyInstance(
        FindIterable.class.getClassLoader(), new Class<?>[] {FindIterable.class},
        (p, m, a) -> m.getName().equals("first") ? null : p);
    MongoCollection<Document> coll = (MongoCollection<Document>) Proxy.newProxyInstance(
        MongoCollection.class.getClassLoader(), new Class<?>[] {MongoCollection.class},
        (p, m, a) -> {
          if (m.getName().equals("find")) { return find; }
          if (m.getName().equals("insertOne")) { inserted.set(new Document((Document) a[0])); }
          return null;
        });
    ProtocolConnection pc = new ProtocolConnection(); pc.protocolConnectionNumber = 3000;
    AutoTagCreator creator = new AutoTagCreator();
    creator.getAutoKeyInitialValue(coll, pc);
    assertEquals(3_000_000_000L, pc.autoKeyId);
    Document tag = RtDataTagDefaults.newTagDocument();
    tag.put("protocolSourceObjectAddress", "STATE/test:INT");
    creator.autoCreateTag(tag, coll, pc);
    assertInstanceOf(Double.class, inserted.get().get("_id"));
    assertEquals(3_000_000_001.0, inserted.get().getDouble("_id"));
  }
}
