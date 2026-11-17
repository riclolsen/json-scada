package org.jsonscada.plc4jclient;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;

import java.io.IOException;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;
import java.security.KeyStore;
import org.junit.jupiter.api.Test;

class MongoTlsTest {

  private static String resource(String name) throws IOException {
    try (InputStream in = MongoTlsTest.class.getResourceAsStream("/tls/" + name)) {
      return new String(in.readAllBytes(), StandardCharsets.ISO_8859_1);
    }
  }

  private static void assertHasKey(KeyStore ks, char[] pw) throws Exception {
    assertEquals(1, ks.getCertificateChain("client").length);
    assertNotNull(ks.getKey("client", pw));
  }

  @Test
  void loadsPkcs8Key() throws Exception {
    assertHasKey(MongoTls.clientKeyStore(resource("client-pkcs8.pem.txt"), new char[0]), new char[0]);
  }

  @Test
  void loadsPkcs1RsaKey() throws Exception {
    assertHasKey(MongoTls.clientKeyStore(resource("client-pkcs1.pem.txt"), new char[0]), new char[0]);
  }

  @Test
  void loadsEncryptedPkcs8Key() throws Exception {
    char[] pw = "secret".toCharArray();
    assertHasKey(MongoTls.clientKeyStore(resource("client-enc.pem.txt"), pw), pw);
  }

  @Test
  void loadsOpenSslDefaultEncryptedKey() throws Exception {
    char[] pw = "secret".toCharArray();
    assertHasKey(MongoTls.clientKeyStore(resource("client-pbes2.pem.txt"), pw), pw);
  }

  @Test
  void noTlsConfigurationMeansNoContext() throws Exception {
    assertNull(MongoTls.build(new ConfigData()));
  }

  @Test
  void insecureBuildsContext() throws Exception {
    ConfigData cfg = new ConfigData();
    cfg.tlsInsecure = true;
    assertNotNull(MongoTls.build(cfg));
  }
  @Test
  void validationSwitchesDoNotEnableTlsOnPlaintextMongoUri() {
    ConfigData cfg = new ConfigData();
    cfg.mongoConnectionString = "mongodb://localhost:27017/?tls=false";
    cfg.tlsAllowChainErrors = true;
    cfg.tlsAllowInvalidHostnames = true;
    var ssl = MongoConnector.settings(cfg).getSslSettings();
    org.junit.jupiter.api.Assertions.assertFalse(ssl.isEnabled());
    org.junit.jupiter.api.Assertions.assertTrue(ssl.isInvalidHostNameAllowed());
  }

  @Test
  void tlsUriUsesConfiguredValidationSwitches() {
    ConfigData cfg = new ConfigData();
    cfg.mongoConnectionString = "mongodb://localhost:27017/?tls=true";
    cfg.tlsInsecure = true;
    var ssl = MongoConnector.settings(cfg).getSslSettings();
    org.junit.jupiter.api.Assertions.assertTrue(ssl.isEnabled());
    org.junit.jupiter.api.Assertions.assertTrue(ssl.isInvalidHostNameAllowed());
    assertNotNull(ssl.getContext());
  }
}
