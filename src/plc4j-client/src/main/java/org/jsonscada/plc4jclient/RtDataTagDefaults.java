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

import org.bson.Document;

/**
 * Factory for new realtimeData tag documents with the JSON-SCADA field set and
 * defaults. Numeric fields are written as BSON doubles.
 */
public final class RtDataTagDefaults {

  private RtDataTagDefaults() {}

  public static Document newTagDocument() {
    return new Document()
        .append("_id", 0.0)
        .append("protocolSourceASDU", "")
        .append("protocolSourceCommonAddress", "")
        .append("protocolSourceConnectionNumber", 0.0)
        .append("protocolSourceObjectAddress", "")
        .append("protocolSourceCommandUseSBO", false)
        .append("protocolSourceCommandDuration", 0.0)
        .append("alarmState", -1.0)
        .append("alarmRange", 0.0)
        .append("description", "")
        .append("ungroupedDescription", "")
        .append("group1", "")
        .append("group2", "")
        .append("group3", "")
        .append("stateTextFalse", "")
        .append("stateTextTrue", "")
        .append("eventTextFalse", "")
        .append("eventTextTrue", "")
        .append("origin", "supervised")
        .append("tag", "")
        .append("type", "analog")
        .append("value", 0.0)
        .append("valueString", "")
        .append("valueJson", null)
        .append("alarmDisabled", false)
        .append("alerted", false)
        .append("alarmed", false)
        .append("annotation", "")
        .append("commandBlocked", false)
        .append("commandOfSupervised", 0.0)
        .append("commissioningRemarks", "")
        .append("formula", 0.0)
        .append("frozen", false)
        .append("frozenDetectTimeout", 0.0)
        .append("hiLimit", Double.MAX_VALUE)
        .append("hihiLimit", Double.MAX_VALUE)
        .append("hihihiLimit", Double.MAX_VALUE)
        .append("historianDeadBand", 0.0)
        .append("historianPeriod", 0.0)
        .append("hysteresis", 0.0)
        .append("invalid", false)
        .append("invalidDetectTimeout", 0.0)
        .append("isEvent", false)
        .append("kconv1", 1.0)
        .append("kconv2", 0.0)
        .append("location", null)
        .append("loLimit", -Double.MAX_VALUE)
        .append("loloLimit", -Double.MAX_VALUE)
        .append("lololoLimit", -Double.MAX_VALUE)
        .append("notes", "")
        .append("overflow", false)
        .append("parcels", null)
        .append("priority", 0.0)
        .append("protocolDestinations", null)
        .append("sourceDataUpdate", null)
        .append("supervisedOfCommand", 0.0)
        .append("substituted", false)
        .append("timeTag", null)
        .append("timeTagAlarm", null)
        .append("timeTagAtSource", null)
        .append("timeTagAtSourceOk", null)
        .append("transient", false)
        .append("unit", "")
        .append("updatesCnt", 0.0)
        .append("valueDefault", 0.0)
        .append("zeroDeadband", 0.0);
  }
}
