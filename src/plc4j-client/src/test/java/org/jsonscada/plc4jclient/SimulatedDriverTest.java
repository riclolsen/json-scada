package org.jsonscada.plc4jclient;

import static org.junit.jupiter.api.Assertions.*;

import java.util.concurrent.TimeUnit;
import org.apache.plc4x.java.api.PlcDriverManager;
import org.apache.plc4x.java.api.types.PlcResponseCode;
import org.junit.jupiter.api.Test;

class SimulatedDriverTest {
  @Test
  void writesAndReadsTypedValueThroughPlc4x() throws Exception {
    try (var conn = PlcDriverManager.getDefault().getConnectionFactory()
        .getConnection("simulated://plc4j-regression")) {
      ProtocolConnection pc = new ProtocolConnection();
      String address = "STATE/test:INT";
      Object value = CommandsWatcher.buildWriteValue(pc, address, "", 123, null);
      var written = conn.writeRequestBuilder().addTagAddress(address, address, value)
          .build().execute().get(5, TimeUnit.SECONDS);
      assertEquals(PlcResponseCode.OK, written.getResponseCode(address));
      var read = conn.readRequestBuilder().addTagAddress(address, address)
          .build().execute().get(5, TimeUnit.SECONDS);
      var extracted = ValueExtractor.extract(read.getPlcValue(address), "", "test", address);
      assertFalse(extracted.bad);
      assertEquals(123, extracted.valDbl);
      var invalid = conn.readRequestBuilder().addTagAddress("bad", "invalid-topic").build();
      assertNotEquals(PlcResponseCode.OK, invalid.getTagResponseCode("bad"));
      assertNull(invalid.getTag("bad"));
    }
  }
  @Test
  void invalidTopicDoesNotPreventScanningValidTopic() throws Exception {
    ProtocolConnection pc = new ProtocolConnection();
    pc.endpointURLs = java.util.List.of("simulated://plc4j-topic-test");
    pc.topics = java.util.List.of("bad|invalid-topic", "good|RANDOM/test:INT");
    MongoWriter writer = new MongoWriter(new ConfigData(), 1);
    AutoTagCreator creator = new AutoTagCreator() {
      @Override public void getAutoKeyInitialValue(
          com.mongodb.client.MongoCollection<org.bson.Document> coll, ProtocolConnection c) {}
    };
    RedundancyManager.isActive = true;
    try {
      Plc4jClient.connectAndScan(pc, PlcDriverManager.getDefault().getConnectionFactory(),
          null, creator, writer);
      var ops = writer.queue.poll(5, TimeUnit.SECONDS);
      assertNotNull(ops);
      assertEquals(1, ops.size());
      assertEquals(java.util.Set.of("RANDOM/test:INT"), pc.endiannessByTag.keySet());
    } finally {
      pc.retire();
      RedundancyManager.isActive = false;
    }
  }
}
