package org.jsonscada.plc4jclient;

import static org.junit.jupiter.api.Assertions.assertEquals;

import org.junit.jupiter.api.Test;

class TopicParserTest {

  @Test
  void parsesNameAddressAndEndianness() {
    TopicParser.ParsedTopic pt = TopicParser.parse("T1|holding-register:5:INT|little_endian", ":");
    assertEquals("T1", pt.jsTagName);
    assertEquals("holding-register:5:INT", pt.address);
    assertEquals("LITTLE_ENDIAN", pt.endianness);
    assertEquals(0, pt.arrayLength);
    assertEquals("analog", pt.jsType);
  }

  @Test
  void legacyArrayIsTranslatedButKeepsConfiguredAddress() {
    TopicParser.ParsedTopic pt = TopicParser.parse("A|holding-register:20:INT[10]", ":");
    assertEquals("holding-register:20:INT[10]", pt.address);
    assertEquals("holding-register:20[0..9]:INT", pt.plc4xAddress);
    assertEquals(10, pt.arrayLength);
  }

  @Test
  void rangeArrayLength() {
    assertEquals(10, TopicParser.arrayLength("holding-register:20[0..9]:INT"));
    assertEquals(-1, TopicParser.arrayLength("holding-register:20[9..0]:INT"));
  }

  @Test
  void zeroLengthLegacyArrayIsInvalid() {
    assertEquals(-1, TopicParser.arrayLength("holding-register:20:INT[0]"));
  }

  @Test
  void infersTypes() {
    assertEquals("digital", TopicParser.inferType("coil:1:BOOL", ":"));
    assertEquals("string", TopicParser.inferType("holding-register:1:STRING", ":"));
    assertEquals("string", TopicParser.inferType("holding-register:1:WSTRING", ":"));
    assertEquals("string", TopicParser.inferType("holding-register:1:WCHAR", ":"));
    assertEquals("json", TopicParser.inferType("x:Struct", ":"));
    assertEquals("analog", TopicParser.inferType("holding-register:1:REAL", ":"));
  }
}
