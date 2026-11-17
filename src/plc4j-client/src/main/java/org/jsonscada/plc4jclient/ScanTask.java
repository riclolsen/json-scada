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

import com.mongodb.client.model.WriteModel;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;
import org.apache.plc4x.java.api.PlcConnection;
import org.apache.plc4x.java.api.messages.PlcReadRequest;
import org.apache.plc4x.java.api.messages.PlcReadResponse;
import org.apache.plc4x.java.api.types.PlcResponseCode;
import org.apache.plc4x.java.api.value.PlcValue;

/**
 * Periodic integrity read of all topics of one PLC connection: immediate read
 * after connect, then every giInterval seconds. The task is bound to the
 * {@link PlcConnection} it was started for and ends (closing that connection)
 * as soon as the connection is replaced, closed, retired by a configuration
 * reload, the instance goes inactive or the device stops answering; the
 * supervision loop then reconnects with a fresh task.
 */
public class ScanTask implements Runnable {

  /** Consecutive failed integrity reads before the connection is recycled. */
  private static final int MAX_CONSECUTIVE_READ_FAILURES = 3;
  /** Granularity of the wait between reads, so the task notices state changes. */
  private static final long WAIT_SLICE_MS = 500;

  private final ProtocolConnection pc;
  private final MongoWriter writer;
  private final PlcConnection conn;
  private final PlcReadRequest request;
  private int consecutiveFailures = 0;

  public ScanTask(
      ProtocolConnection pc, MongoWriter writer, PlcConnection conn, PlcReadRequest request) {
    this.pc = pc;
    this.writer = writer;
    this.conn = conn;
    this.request = request;
  }

  /** True while this task's connection is still the live one of the protocol connection. */
  private boolean alive() {
    return !Thread.currentThread().isInterrupted() && RedundancyManager.isActive
        && !pc.retired && pc.plcConn == conn && conn.isConnected();
  }

  @Override
  public void run() {
    try {
      loop();
    } finally {
      pc.close(conn);
    }
  }

  private void loop() {
    long periodMs = pc.giInterval > 0 ? (long) (pc.giInterval * 1000) : 300_000L;
    if (pc.giInterval <= 0) {
      Log.log(pc.name + ": giInterval not set, defaulting to 300s");
    }
    if (!execRead()) {
      consecutiveFailures++;
    }
    while (true) {
      long next = System.currentTimeMillis() + periodMs;
      while (System.currentTimeMillis() < next) {
        if (!alive()) {
          return;
        }
        RedundancyManager.sleepMs(Math.max(1, Math.min(WAIT_SLICE_MS, next - System.currentTimeMillis())));
      }
      if (!alive()) {
        return;
      }
      if (!RedundancyManager.isActive) {
        Log.log("Instance inactive! Closing connection...");
        return;
      }
      if (execRead()) {
        consecutiveFailures = 0;
        continue;
      }
      consecutiveFailures++;
      if (consecutiveFailures >= MAX_CONSECUTIVE_READ_FAILURES
          || !Plc4jClient.pingOk(pc.name, conn)) {
        Log.log(pc.name + ": device not answering, closing connection...");
        return;
      }
    }
  }

  /** Executes the integrity read; returns false on read failure. */
  boolean execRead() {
    if (!alive()) {
      return false;
    }
    if (Log.level >= Log.LEVEL_BASIC) {
      Log.log(pc.name + ": integrity read...");
    }
    PlcReadResponse resp;
    java.util.concurrent.CompletableFuture<? extends PlcReadResponse> future = null;
    try {
      future = request.execute();
      long timeoutS = Math.max(5, Math.min(60, (long) pc.giInterval));
      resp = future.get(timeoutS, TimeUnit.SECONDS);
    } catch (Exception e) {
      if ((e instanceof TimeoutException || e instanceof InterruptedException) && future != null) {
        future.cancel(true); // do not leave the request pending against a hung device
      }
      if (e instanceof InterruptedException) {
        Thread.currentThread().interrupt();
      }
      Log.log(pc.name + ": error executing read-request: "
          + (e.getMessage() != null ? e.getMessage() : e.toString()));
      return false;
    }

    List<WriteModel<org.bson.Document>> ops = new ArrayList<>();
    for (String tagName : resp.getTagNames()) {
      PlcResponseCode rc = null;
      try {
        rc = resp.getResponseCode(tagName);
      } catch (Exception e) {
        // tolerate drivers without per-tag response codes
      }
      PlcValue v = null;
      if (rc == null || rc == PlcResponseCode.OK) {
        try {
          v = resp.getPlcValue(tagName);
        } catch (Exception e) {
          // value extraction below will mark it bad
        }
      }
      ExtractedValue ev;
      String asduAtSource;
      if (v == null) {
        ev = ExtractedValue.ofBad();
        asduAtSource = rc != null ? rc.name() : "ERROR";
        if (Log.level >= Log.LEVEL_DETAILED) {
          Log.log(pc.name + ": Read result '" + tagName + "': response code " + asduAtSource);
        }
      } else {
        String endianness = pc.endiannessByTag.getOrDefault(tagName, "");
        ev = ValueExtractor.extract(v, endianness, pc.name, tagName);
        asduAtSource = v.getPlcValueType() != null ? v.getPlcValueType().name() : "Unknown";
      }
      SourceDataUpdater.appendOps(
          ops, pc, tagName, asduAtSource, ev, pc.arrayLengthByTag.getOrDefault(tagName, 0));
    }
    if (alive() && !ops.isEmpty() && !writer.queue.offer(ops)) {
      Log.log("Error: mongo write channel full. Discarding values!");
    }
    return true;
  }
}
