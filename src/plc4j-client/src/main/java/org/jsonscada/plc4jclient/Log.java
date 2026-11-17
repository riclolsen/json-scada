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

import java.time.LocalDateTime;
import java.time.format.DateTimeFormatter;

/** Driver log output to stdout, levels 0 (min) to 3 (debug). */
public final class Log {
  public static final int LEVEL_MIN = 0;
  public static final int LEVEL_BASIC = 1;
  public static final int LEVEL_DETAILED = 2;
  public static final int LEVEL_DEBUG = 3;

  public static volatile int level = LEVEL_BASIC;

  private static final DateTimeFormatter FMT =
      DateTimeFormatter.ofPattern("yyyy/MM/dd HH:mm:ss.SSSSSS");

  private Log() {}

  public static void log(String msg) {
    System.out.println(LocalDateTime.now().format(FMT) + " " + msg);
  }

  public static void log(String msg, Throwable e) {
    log(msg + " - " + e);
  }

  /** Logs and terminates the process. */
  public static void fatal(String msg) {
    log(msg);
    System.exit(1);
  }
}
