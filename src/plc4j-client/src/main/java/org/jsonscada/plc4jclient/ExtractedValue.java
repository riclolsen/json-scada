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

import java.util.ArrayList;
import java.util.List;

/** Result of converting a PlcValue for JSON-SCADA (double, string, json, array, bad). */
public class ExtractedValue {
  public double valDbl = 0;
  public String valStr = "";
  public String valJson = "{}";
  public List<Double> valArrDbl = new ArrayList<>();
  public boolean bad = false;

  public static ExtractedValue ofBad() {
    ExtractedValue ev = new ExtractedValue();
    ev.bad = true;
    return ev;
  }
}
