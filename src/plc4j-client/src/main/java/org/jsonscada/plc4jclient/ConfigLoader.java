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

import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import org.bson.Document;

/**
 * Command line / environment / config file resolution: args [instance] [log level]
 * [config file] [point filter], env vars JS_PLC4X_INSTANCE, JS_PLC4X_LOGLEVEL and
 * JS_CONFIG_FILE. The 4th argument (point filter) is accepted for command line
 * compatibility with the other drivers and ignored.
 */
public final class ConfigLoader {

  public static class Result {
    public ConfigData cfg;
    public int instanceNumber = 1;
    public int logLevel = Log.LEVEL_BASIC;
  }

  private ConfigLoader() {}

  public static Result load(String[] args) {
    Result r = new Result();

    String envInstance = System.getenv("JS_PLC4X_INSTANCE");
    if (envInstance != null && !envInstance.isEmpty()) {
      try {
        r.instanceNumber = Integer.parseInt(envInstance.trim());
      } catch (NumberFormatException e) {
        Log.log("JS_PLC4X_INSTANCE environment variable should be a number!");
        System.exit(2);
      }
    }
    if (args.length > 0) {
      try {
        r.instanceNumber = Integer.parseInt(args[0].trim());
      } catch (NumberFormatException e) {
        Log.log("Instance parameter should be a number!");
        System.exit(2);
      }
    }

    String envLogLevel = System.getenv("JS_PLC4X_LOGLEVEL");
    if (envLogLevel != null && !envLogLevel.isEmpty()) {
      try {
        r.logLevel = Integer.parseInt(envLogLevel.trim());
      } catch (NumberFormatException e) {
        Log.log("JS_PLC4X_LOGLEVEL environment variable should be a number!");
        System.exit(2);
      }
    }
    if (args.length > 1) {
      try {
        r.logLevel = Integer.parseInt(args[1].trim());
      } catch (NumberFormatException e) {
        Log.log("Log Level parameter should be a number!");
        System.exit(2);
      }
    }

    Path cfgFile = Paths.get("..", "conf", "json-scada.json");
    if (!Files.exists(cfgFile)) {
      cfgFile = Paths.get("c:\\", "json-scada", "conf", "json-scada.json");
    }
    if (!Files.exists(cfgFile)) {
      cfgFile = Paths.get("/home", "jsonscada", "json-scada", "conf", "json-scada.json");
    }
    String envCfgFile = System.getenv("JS_CONFIG_FILE");
    if (envCfgFile != null && !envCfgFile.isEmpty()) {
      cfgFile = Paths.get(envCfgFile);
    }
    if (args.length > 2) {
      cfgFile = Paths.get(args[2]);
    }

    if (args.length > 3) {
      Log.log("Point filter argument ignored: not supported by this driver.");
    }

    String contents;
    try {
      contents = Files.readString(cfgFile);
      if (!contents.isEmpty() && contents.charAt(0) == 0xFEFF) {
        contents = contents.substring(1); // strip UTF-8 BOM (common on Windows)
      }
    } catch (Exception e) {
      Log.log("Failed to read config file: " + cfgFile + " - " + e);
      System.exit(1);
      return r; // unreachable
    }

    Document doc;
    try {
      doc = Document.parse(contents);
    } catch (Exception e) {
      Log.log("Failed to parse config file JSON: " + e);
      System.exit(1);
      return r; // unreachable
    }

    ConfigData cfg = new ConfigData();
    cfg.nodeName = str(doc, "nodeName");
    cfg.mongoConnectionString = str(doc, "mongoConnectionString");
    cfg.mongoDatabaseName = str(doc, "mongoDatabaseName");
    cfg.tlsCaPemFile = str(doc, "tlsCaPemFile");
    cfg.tlsClientPemFile = str(doc, "tlsClientPemFile");
    cfg.tlsClientPfxFile = str(doc, "tlsClientPfxFile");
    cfg.tlsClientKeyPassword = str(doc, "tlsClientKeyPassword");
    cfg.tlsAllowInvalidHostnames = bool(doc, "tlsAllowInvalidHostnames");
    cfg.tlsAllowChainErrors = bool(doc, "tlsAllowChainErrors");
    cfg.tlsInsecure = bool(doc, "tlsInsecure");

    if (cfg.mongoConnectionString.isEmpty()
        || cfg.mongoDatabaseName.isEmpty()
        || cfg.nodeName.isEmpty()) {
      Log.log("Empty string in config file.");
      System.exit(1);
    }
    r.cfg = cfg;
    return r;
  }

  private static String str(Document doc, String key) {
    Object v = doc.get(key);
    return v instanceof String s ? s.trim() : "";
  }

  private static boolean bool(Document doc, String key) {
    Object v = doc.get(key);
    return v instanceof Boolean b && b;
  }
}
