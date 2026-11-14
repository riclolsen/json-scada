/*
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

export interface ILog {
  levelMin: number;
  levelNormal: number;
  levelDetailed: number;
  levelDebug: number;
  levelCurrent: number;
  log: (msg: string, level?: number) => void;
}

const Log: ILog = {
  // simple message logger
  levelMin: 0,
  levelNormal: 1,
  levelDetailed: 2,
  levelDebug: 3,
  levelCurrent: 1,
  log: function (msg: string, level: number = 1) {
    if (level <= this.levelCurrent) {
      let dt = new Date()
      // stderr: stdout carries the MCP stdio protocol stream
      console.error(dt.toISOString() + ' - ' + msg)
    }
  },
}

export default Log
