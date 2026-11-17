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

import com.mongodb.ConnectionString;
import com.mongodb.MongoClientSettings;
import com.mongodb.client.MongoClient;
import com.mongodb.client.MongoClients;
import com.mongodb.client.MongoCollection;
import com.mongodb.client.MongoDatabase;
import org.bson.Document;

/** MongoDB connection; TLS material from the config file is applied through an SSLContext. */
public class MongoConnector implements AutoCloseable {
  public final MongoClient client;
  public final MongoCollection<Document> rtData;
  public final MongoCollection<Document> instances;
  public final MongoCollection<Document> connections;
  public final MongoCollection<Document> commands;

  private MongoConnector(MongoClient client, MongoDatabase db) {
    this.client = client;
    this.rtData = db.getCollection("realtimeData");
    this.instances = db.getCollection("protocolDriverInstances");
    this.connections = db.getCollection("protocolConnections");
    this.commands = db.getCollection("commandsQueue");
  }

  static MongoClientSettings settings(ConfigData cfg) {
    // The Java driver has no tlsCAFile/tlsCertificateKeyFile connection string
    // options: the PEM/PFX files are loaded into an SSLContext instead.
    MongoClientSettings.Builder settings = MongoClientSettings.builder()
        .applyConnectionString(new ConnectionString(cfg.mongoConnectionString));
    try {
      var ctx = MongoTls.build(cfg);
      if (ctx != null || cfg.tlsAllowInvalidHostnames) {
        boolean enabled = settings.build().getSslSettings().isEnabled()
            || !cfg.tlsCaPemFile.isEmpty() || !cfg.tlsClientPemFile.isEmpty()
            || !cfg.tlsClientPfxFile.isEmpty();
        boolean skipHostCheck = cfg.tlsAllowInvalidHostnames || cfg.tlsInsecure;
        settings.applyToSslSettings(
            ssl -> ssl.enabled(enabled).context(ctx).invalidHostNameAllowed(skipHostCheck));
      }
    } catch (Exception e) {
      throw new IllegalStateException("TLS configuration error: " + e.getMessage(), e);
    }

    return settings.build();
  }

  public static MongoConnector connect(ConfigData cfg) {
    MongoClient client = MongoClients.create(settings(cfg));
    try {
      MongoDatabase db = client.getDatabase(cfg.mongoDatabaseName);
      // fail fast if the server is unreachable
      db.runCommand(new Document("ping", 1));
      return new MongoConnector(client, db);
    } catch (RuntimeException e) {
      client.close();
      throw e;
    }
  }

  @Override
  public void close() {
    try {
      client.close();
    } catch (Exception e) {
      // ignore errors on close
    }
  }
}
