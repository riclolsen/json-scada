package org.jsonscada.plc4jclient;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.util.List;
import org.apache.plc4x.java.spi.values.PlcINT;
import org.apache.plc4x.java.spi.values.PlcList;
import org.apache.plc4x.java.spi.values.PlcREAL;
import org.apache.plc4x.java.api.value.PlcValue;
import org.junit.jupiter.api.Test;

class ValueExtractorTest {

  @Test
  void extractsInt() {
    ExtractedValue ev = ValueExtractor.extract(new PlcINT(-12), "", "c", "t");
    assertFalse(ev.bad);
    assertEquals(-12, ev.valDbl);
    assertEquals("-12", ev.valStr);
  }

  @Test
  void swapsInt() {
    ExtractedValue ev = ValueExtractor.extract(new PlcINT(0x0102), "LITTLE_ENDIAN", "c", "t");
    assertEquals(0x0201, ev.valDbl);
  }

  @Test
  void nanAndInfinityAreBadWithValidJson() {
    ExtractedValue nan = ValueExtractor.extract(new PlcREAL(Float.NaN), "", "c", "t");
    assertTrue(nan.bad);
    assertEquals(0, nan.valDbl);
    assertEquals("null", nan.valJson);
    ExtractedValue inf = ValueExtractor.extract(new PlcREAL(Float.POSITIVE_INFINITY), "", "c", "t");
    assertTrue(inf.bad);
  }

  @Test
  void listWithBadElementIsBad() {
    PlcValue list = new PlcList(List.of(new PlcINT(1), new PlcREAL(Float.NaN)));
    ExtractedValue ev = ValueExtractor.extract(list, "", "c", "t");
    assertTrue(ev.bad);
    assertEquals(2, ev.valArrDbl.size());
    assertEquals("[1,0]", ev.valJson);
  }
}
