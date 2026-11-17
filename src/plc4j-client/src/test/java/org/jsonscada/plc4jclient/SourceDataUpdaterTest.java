package org.jsonscada.plc4jclient;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

import com.mongodb.client.model.UpdateManyModel;
import com.mongodb.client.model.WriteModel;
import java.util.ArrayList;
import java.util.List;
import org.bson.Document;
import org.junit.jupiter.api.Test;

class SourceDataUpdaterTest {

  private static Document filterOf(WriteModel<Document> m) {
    return (Document) ((UpdateManyModel<Document>) m).getFilter();
  }

  private static boolean invalid(WriteModel<Document> m) {
    Document set = (Document) ((Document) ((UpdateManyModel<Document>) m).getUpdate()).get("$set");
    return ((Document) set.get("sourceDataUpdate")).getBoolean("invalidAtSource");
  }

  @Test
  void failedArrayReadInvalidatesEveryElementTag() {
    ProtocolConnection pc = new ProtocolConnection();
    pc.protocolConnectionNumber = 7;
    List<WriteModel<Document>> ops = new ArrayList<>();
    SourceDataUpdater.appendOps(ops, pc, "holding-register:20:INT[3]", "ERROR",
        ExtractedValue.ofBad(), 3);
    assertEquals(3, ops.size());
    for (int i = 0; i < 3; i++) {
      assertEquals("holding-register:20:INT[3][" + i + "]",
          filterOf(ops.get(i)).getString("protocolSourceObjectAddress"));
      assertTrue(invalid(ops.get(i)));
    }
  }

  @Test
  void shortArrayReadInvalidatesMissingElements() {
    ProtocolConnection pc = new ProtocolConnection();
    ExtractedValue ev = new ExtractedValue();
    ev.valArrDbl.add(1.0);
    ev.valArrDbl.add(2.0);
    List<WriteModel<Document>> ops = new ArrayList<>();
    SourceDataUpdater.appendOps(ops, pc, "a", "INT", ev, 3);
    assertEquals(3, ops.size());
    assertEquals(false, invalid(ops.get(1)));
    assertTrue(invalid(ops.get(2)));
  }

  @Test
  void scalarUsesPlainAddress() {
    ProtocolConnection pc = new ProtocolConnection();
    List<WriteModel<Document>> ops = new ArrayList<>();
    SourceDataUpdater.appendOps(ops, pc, "a", "INT", new ExtractedValue(), 0);
    assertEquals(1, ops.size());
    assertEquals("a", filterOf(ops.get(0)).getString("protocolSourceObjectAddress"));
  }
}
