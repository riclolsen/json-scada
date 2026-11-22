/*
 * {json:scada} - Copyright (c) 2020-2025 - Ricardo L. Olsen
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

// Small promise wrapper around child_process.spawn used by both platform backends.
// Always shell:false with an args array — command lines are never assembled from
// strings, so request data cannot be injected into a shell.

'use strict'

const { spawn } = require('node:child_process')

const DEFAULT_TIMEOUT_MS = 30000

// Runs a command and resolves with { code, stdout, stderr, timedOut }. Never rejects
// on a non-zero exit code (callers inspect .code); rejects only on spawn failure.
function run(cmd, args, opts = {}) {
  return new Promise((resolve, reject) => {
    let child
    try {
      child = spawn(cmd, args, {
        shell: false,
        windowsHide: true,
        env: opts.env ? { ...process.env, ...opts.env } : process.env,
        cwd: opts.cwd || undefined,
      })
    } catch (err) {
      reject(err)
      return
    }

    let stdout = ''
    let stderr = ''
    let timedOut = false
    // collect raw buffers and decode them once at the end (opts.utf16: UTF-16LE,
    // else UTF-8), so multi-byte characters split across chunks stay intact.
    const chunksOut = []
    const chunksErr = []

    const timer = setTimeout(() => {
      timedOut = true
      try {
        child.kill()
      } catch (e) {
        /* ignore */
      }
    }, opts.timeout || DEFAULT_TIMEOUT_MS)

    child.stdout.on('data', (d) => chunksOut.push(d))
    child.stderr.on('data', (d) => chunksErr.push(d))

    child.on('error', (err) => {
      clearTimeout(timer)
      reject(err)
    })

    child.on('close', (code) => {
      clearTimeout(timer)
      const enc = opts.utf16 ? 'utf16le' : 'utf8'
      stdout = Buffer.concat(chunksOut).toString(enc)
      stderr = Buffer.concat(chunksErr).toString(enc)
      resolve({ code, stdout, stderr, timedOut })
    })
  })
}

// An error of the service management meant for the admin (a driver that is
// not manageable, a failed supervisorctl/WinSW/sc action with its output): its
// message is answered to the AdminUI, other errors only go to the log.
class ProcessError extends Error {
  constructor(msg) {
    super(String(msg).slice(0, 1000))
    this.name = 'ProcessError'
  }
}

module.exports = { run, DEFAULT_TIMEOUT_MS, ProcessError }
