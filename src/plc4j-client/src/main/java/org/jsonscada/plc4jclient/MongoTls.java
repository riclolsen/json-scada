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

import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.FileInputStream;
import java.io.IOException;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Paths;
import java.security.GeneralSecurityException;
import java.security.KeyFactory;
import java.security.KeyStore;
import java.security.PrivateKey;
import java.security.cert.Certificate;
import java.security.cert.CertificateFactory;
import java.security.cert.X509Certificate;
import java.security.spec.PKCS8EncodedKeySpec;
import java.util.ArrayList;
import java.util.Base64;
import java.util.List;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import javax.crypto.Cipher;
import javax.crypto.EncryptedPrivateKeyInfo;
import javax.crypto.SecretKeyFactory;
import javax.crypto.spec.PBEKeySpec;
import javax.net.ssl.KeyManager;
import javax.net.ssl.KeyManagerFactory;
import javax.net.ssl.SSLContext;
import javax.net.ssl.TrustManager;
import javax.net.ssl.TrustManagerFactory;
import javax.net.ssl.X509TrustManager;

/**
 * Builds the SSLContext for the MongoDB connection from the TLS settings of the
 * JSON-SCADA config file: tlsCaPemFile (trusted CA certificates), tlsClientPemFile
 * (client certificate chain and private key in one PEM file: PKCS#8, encrypted
 * PKCS#8 or PKCS#1 RSA), tlsClientPfxFile (PKCS#12), tlsClientKeyPassword and the
 * tlsInsecure/tlsAllowChainErrors switches.
 */
final class MongoTls {

  private static final Pattern PEM_BLOCK = Pattern.compile(
      "-----BEGIN ([A-Z0-9 ]+)-----(.*?)-----END \\1-----", Pattern.DOTALL);

  private MongoTls() {}

  /** Returns null when the configuration asks for no custom TLS material. */
  static SSLContext build(ConfigData cfg) throws GeneralSecurityException, IOException {
    boolean insecure = cfg.tlsInsecure || cfg.tlsAllowChainErrors;
    boolean hasCa = !cfg.tlsCaPemFile.isEmpty();
    boolean hasPfx = !cfg.tlsClientPfxFile.isEmpty();
    boolean hasPem = !cfg.tlsClientPemFile.isEmpty();
    if (!insecure && !hasCa && !hasPfx && !hasPem) {
      return null;
    }
    char[] password = cfg.tlsClientKeyPassword.toCharArray();

    TrustManager[] trust = null;
    if (insecure) {
      trust = new TrustManager[] {new TrustAll()};
    } else if (hasCa) {
      KeyStore ca = KeyStore.getInstance(KeyStore.getDefaultType());
      ca.load(null, null);
      int n = 0;
      for (Certificate c : certificates(readFile(cfg.tlsCaPemFile))) {
        ca.setCertificateEntry("ca" + n++, c);
      }
      if (n == 0) {
        throw new GeneralSecurityException("no certificate found in " + cfg.tlsCaPemFile);
      }
      TrustManagerFactory tmf =
          TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm());
      tmf.init(ca);
      trust = tmf.getTrustManagers();
    }

    KeyManager[] keys = null;
    if (hasPfx || hasPem) {
      KeyStore client;
      if (hasPfx) {
        client = KeyStore.getInstance("PKCS12");
        try (InputStream in = new FileInputStream(cfg.tlsClientPfxFile)) {
          client.load(in, password);
        }
      } else {
        client = clientKeyStore(readFile(cfg.tlsClientPemFile), password);
      }
      KeyManagerFactory kmf =
          KeyManagerFactory.getInstance(KeyManagerFactory.getDefaultAlgorithm());
      kmf.init(client, password);
      keys = kmf.getKeyManagers();
    }

    SSLContext ctx = SSLContext.getInstance("TLS");
    ctx.init(keys, trust, null);
    return ctx;
  }

  private static String readFile(String file) throws IOException {
    return Files.readString(Paths.get(file), StandardCharsets.ISO_8859_1);
  }

  /** All X.509 certificates found in PEM text, in file order. */
  static List<Certificate> certificates(String pem) throws GeneralSecurityException {
    List<Certificate> out = new ArrayList<>();
    CertificateFactory cf = CertificateFactory.getInstance("X.509");
    Matcher m = PEM_BLOCK.matcher(pem);
    while (m.find()) {
      String type = m.group(1);
      if (type.equals("CERTIFICATE") || type.equals("X509 CERTIFICATE")) {
        out.add(cf.generateCertificate(new ByteArrayInputStream(der(m.group(2)))));
      }
    }
    return out;
  }

  /** Key store with the certificate chain and private key of a combined PEM text. */
  static KeyStore clientKeyStore(String pem, char[] password)
      throws GeneralSecurityException, IOException {
    List<Certificate> chain = certificates(pem);
    if (chain.isEmpty()) {
      throw new GeneralSecurityException("no client certificate found in the PEM file");
    }
    PrivateKey key = null;
    Matcher m = PEM_BLOCK.matcher(pem);
    while (m.find() && key == null) {
      String type = m.group(1);
      String body = m.group(2);
      switch (type) {
        case "PRIVATE KEY":
          key = pkcs8(der(body));
          break;
        case "ENCRYPTED PRIVATE KEY":
          key = pkcs8(decrypt(der(body), password));
          break;
        case "RSA PRIVATE KEY":
          if (body.contains("ENCRYPTED")) {
            throw new GeneralSecurityException("encrypted PKCS#1 key is not supported, "
                + "convert it: openssl pkcs8 -topk8 -v2 aes256 -in key.pem -out key8.pem");
          }
          key = pkcs8(wrapRsaPkcs1(der(body)));
          break;
        case "EC PRIVATE KEY":
          throw new GeneralSecurityException("SEC1 EC key is not supported, convert it: "
              + "openssl pkcs8 -topk8 -nocrypt -in key.pem -out key8.pem");
        default:
          break;
      }
    }
    if (key == null) {
      throw new GeneralSecurityException("no private key found in the PEM file");
    }
    KeyStore ks = KeyStore.getInstance("PKCS12");
    ks.load(null, null);
    ks.setKeyEntry("client", key, password, chain.toArray(new Certificate[0]));
    return ks;
  }

  private static byte[] der(String pemBody) {
    // PEM headers (e.g. Proc-Type) contain ':' and are not base64: keep base64 lines only
    StringBuilder sb = new StringBuilder();
    for (String line : pemBody.split("\\r?\\n")) {
      String t = line.trim();
      if (!t.isEmpty() && !t.contains(":")) {
        sb.append(t);
      }
    }
    return Base64.getDecoder().decode(sb.toString());
  }

  private static byte[] decrypt(byte[] encrypted, char[] password)
      throws GeneralSecurityException, IOException {
    EncryptedPrivateKeyInfo info = new EncryptedPrivateKeyInfo(encrypted);
    SecretKeyFactory skf = SecretKeyFactory.getInstance(info.getAlgName());
    var secret = skf.generateSecret(new PBEKeySpec(password));
    Cipher cipher = Cipher.getInstance(info.getAlgName());
    cipher.init(Cipher.DECRYPT_MODE, secret, info.getAlgParameters());
    return info.getKeySpec(cipher).getEncoded();
  }

  private static PrivateKey pkcs8(byte[] der) throws GeneralSecurityException {
    PKCS8EncodedKeySpec spec = new PKCS8EncodedKeySpec(der);
    GeneralSecurityException last = null;
    for (String alg : new String[] {"RSA", "EC", "Ed25519", "DSA"}) {
      try {
        return KeyFactory.getInstance(alg).generatePrivate(spec);
      } catch (GeneralSecurityException e) {
        last = e;
      }
    }
    throw new GeneralSecurityException("unsupported private key type", last);
  }

  /** Wraps a PKCS#1 RSAPrivateKey into a PKCS#8 PrivateKeyInfo structure. */
  static byte[] wrapRsaPkcs1(byte[] pkcs1) {
    byte[] version = {0x02, 0x01, 0x00};
    byte[] algorithm = {0x30, 0x0D, 0x06, 0x09, 0x2A, (byte) 0x86, 0x48, (byte) 0x86,
        (byte) 0xF7, 0x0D, 0x01, 0x01, 0x01, 0x05, 0x00};
    ByteArrayOutputStream octet = new ByteArrayOutputStream();
    octet.write(0x04);
    writeLength(octet, pkcs1.length);
    octet.writeBytes(pkcs1);
    ByteArrayOutputStream inner = new ByteArrayOutputStream();
    inner.writeBytes(version);
    inner.writeBytes(algorithm);
    inner.writeBytes(octet.toByteArray());
    ByteArrayOutputStream outer = new ByteArrayOutputStream();
    outer.write(0x30);
    writeLength(outer, inner.size());
    outer.writeBytes(inner.toByteArray());
    return outer.toByteArray();
  }

  private static void writeLength(ByteArrayOutputStream out, int len) {
    if (len < 0x80) {
      out.write(len);
    } else if (len <= 0xFF) {
      out.write(0x81);
      out.write(len);
    } else if (len <= 0xFFFF) {
      out.write(0x82);
      out.write(len >> 8);
      out.write(len & 0xFF);
    } else {
      out.write(0x83);
      out.write(len >> 16);
      out.write((len >> 8) & 0xFF);
      out.write(len & 0xFF);
    }
  }

  /** Accepts any server certificate chain (tlsInsecure / tlsAllowChainErrors). */
  private static final class TrustAll implements X509TrustManager {
    @Override
    public void checkClientTrusted(X509Certificate[] chain, String authType) {}

    @Override
    public void checkServerTrusted(X509Certificate[] chain, String authType) {}

    @Override
    public X509Certificate[] getAcceptedIssuers() {
      return new X509Certificate[0];
    }
  }
}
