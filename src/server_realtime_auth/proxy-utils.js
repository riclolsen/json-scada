/*
 * Reverse proxy helpers for JSON SCADA.
 * {json:scada} - Copyright (c) 2020-2024 - Ricardo L. Olsen
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

'use strict'

const http = require('http')
const {
  createProxyMiddleware,
  fixRequestBody,
} = require('http-proxy-middleware')
const jwt = require('jsonwebtoken')
const config = require('./app/config/auth.config.js')
const Log = require('./simple-logger')
const UserRights = require('./app/services/user-rights')

const NODERED_MOUNT_PATH = '/nodered'
const NODERED_DEFAULT_SERVER = 'http://127.0.0.1:1880/nodered/'
const N8N_MOUNT_PATH = '/n8n'
const N8N_DEFAULT_SERVER = 'http://127.0.0.1:5678/'
const LOGIO_MOUNT_PATH = '/log-io'
const LOGIO_DEFAULT_SERVER = 'http://127.0.0.1:6688'
const SUPERVISOR_MOUNT_PATH = '/supervisor'
// credentials of the [inet_http_server] section of the supervisord.conf shipped
// in platform-*/ (and of the docker image); override with JS_SUPERVISOR_SERVER.
const SUPERVISOR_DEFAULT_SERVER = 'http://admin:jsonscada@127.0.0.1:9000'

// Express strips the mount path from req.url when a middleware is mounted with
// app.use('/nodered', ...), so a proxy mounted that way would forward '/' instead of
// '/nodered/' and the target answers 404. http-proxy-middleware did this restore itself
// while legacyCreateProxyMiddleware existed (req.url = req.originalUrl || req.url); v4
// removed both the legacy factory and that patch, so it has to be done here.
// Websocket upgrades never go through express: they are handed to proxy.upgrade() with
// the full url already in req.url and no originalUrl, so they are left untouched.
function withMountedUrl(proxy) {
  const middleware = function (req, res, next) {
    if (req.originalUrl) req.url = req.originalUrl
    return proxy(req, res, next)
  }
  if (typeof proxy.upgrade === 'function')
    middleware.upgrade = proxy.upgrade.bind(proxy)
  return middleware
}

// Express strips the mount path from req.url, while http upgrade requests (websockets)
// never go through express and keep the full url. Matching on originalUrl when available
// lets the same middleware filter both cases.
function mountPathFilter(mountPath) {
  return function (_pathname, req) {
    const url = (req?.originalUrl || req?.url || '').split('?')[0]
    return url === mountPath || url.startsWith(mountPath + '/')
  }
}

// The session token of this server is not for the proxied apps: it is removed
// from the forwarded request (header and cookie), so a proxied app (or whoever
// controls it) never gets a token valid for JSON-SCADA. Other cookies stay.
function stripSessionToken(req) {
  delete req.headers['x-access-token']
  const ck = req.headers.cookie
  if (typeof ck !== 'string') return
  const kept = ck
    .split(';')
    .filter((c) => c.split('=')[0].trim() !== 'x-access-token')
    .join(';')
    .trim()
  if (kept === '') delete req.headers.cookie
  else req.headers.cookie = kept
}

// Token check for websocket upgrades, that do not pass through the express middlewares.
function checkUpgradeToken(req) {
  let token = req.headers['x-access-token']
  if (!token && typeof req.headers?.cookie === 'string') {
    req.headers.cookie.split(';').forEach((cookie) => {
      const sep = cookie.indexOf('=')
      if (sep === -1) return
      if (cookie.slice(0, sep).trim() === 'x-access-token')
        token = decodeURIComponent(cookie.slice(sep + 1).trim())
    })
  }
  if (!token) return false

  try {
    return jwt.verify(token, config.secret, { algorithms: ['HS256'] })
  } catch (err) {
    return false
  }
}

// Reverse proxy for a web app that serves itself under a path prefix of its own,
// mounted on mountPath. The path of the target url replaces the mount path on the
// forwarded request (nothing to rewrite when both are the same).
// Websocket upgrades are handled by attachProxyUpgrade, to keep them authenticated.
function createPrefixedProxy(label, mountPath, server, defaultServer) {
  let target
  try {
    target = new URL(server)
  } catch (err) {
    Log.log(
      'Invalid ' + label + ' server url: ' + server + ', using ' + defaultServer
    )
    target = new URL(defaultServer)
  }

  const basePath = target.pathname.replace(/\/+$/, '')
  Log.log(
    label +
      ' reverse proxy on ' +
      mountPath +
      ' -> ' +
      target.origin +
      basePath +
      '/'
  )

  return withMountedUrl(
    createProxyMiddleware({
      target: target.origin,
      changeOrigin: true,
      ws: false, // upgrades are handled by attachProxyUpgrade, to keep them authenticated
      pathFilter: mountPathFilter(mountPath),
      ...(basePath === mountPath
        ? {}
        : { pathRewrite: { ['^' + mountPath]: basePath } }),
      on: {
        // the json/urlencoded body parsers run before this proxy, restore the consumed body
        proxyReq: fixRequestBody,
        error: (err, req, resOrSocket) => {
          Log.log(label + ' proxy error: ' + err.message)
          if (typeof resOrSocket?.writeHead === 'function') {
            if (!resOrSocket.headersSent) {
              resOrSocket.writeHead(502, { 'Content-Type': 'text/plain' })
              resOrSocket.end(label + ' server not available')
            }
          } else resOrSocket?.destroy?.()
        },
      },
    })
  )
}

// Reverse proxy for the Node-RED editor mounted on /nodered.
// Node-RED serves itself under httpAdminRoot ('/nodered/' by default).
// Websocket upgrades (<httpAdminRoot>/comms) are handled by attachNoderedUpgrade.
function createNoderedProxy(noderedServer) {
  return createPrefixedProxy(
    'Node-RED',
    NODERED_MOUNT_PATH,
    noderedServer,
    NODERED_DEFAULT_SERVER
  )
}

// Reverse proxy for the n8n editor mounted on /n8n.
// n8n serves everything from its root (/, /static, /rest, /rest/push) and uses N8N_PATH
// only for the links of its pages, so it must run with N8N_PATH=/n8n/ and the mount
// path is stripped (target url without a path, the default). Websocket upgrades (push
// channel, /n8n/rest/push) are handled by attachN8nUpgrade.
function createN8nProxy(n8nServer) {
  return createPrefixedProxy('n8n', N8N_MOUNT_PATH, n8nServer, N8N_DEFAULT_SERVER)
}

// Reverse proxy for the websocket transport of the log.io ui, mounted on /log-io.
// The ui derives its socket.io path from the page mount point, so it connects to
// /log-io/socket.io while the log.io server serves socket.io at /socket.io: the mount
// path is stripped from the forwarded request, just like the express /log-io proxy does
// for the http (polling) requests.
// Only the upgrades reach this proxy, through attachLogioUpgrade.
function createLogioProxy(logioServer) {
  let target
  try {
    target = new URL(logioServer)
  } catch (err) {
    Log.log(
      'Invalid log.io server url: ' +
        logioServer +
        ', using ' +
        LOGIO_DEFAULT_SERVER
    )
    target = new URL(LOGIO_DEFAULT_SERVER)
  }

  Log.log(
    'Log.io websocket reverse proxy on ' +
      LOGIO_MOUNT_PATH +
      ' -> ' +
      target.origin +
      '/'
  )

  return withMountedUrl(
    createProxyMiddleware({
      target: target.origin,
      changeOrigin: true,
      ws: false, // upgrades are handled by attachLogioUpgrade, to keep them authenticated
      pathFilter: mountPathFilter(LOGIO_MOUNT_PATH), // do not grab upgrades of other proxies
      pathRewrite: { ['^' + LOGIO_MOUNT_PATH]: '' },
      on: {
        // the json/urlencoded body parsers run before this proxy, restore the consumed body
        proxyReq: fixRequestBody,
        error: (err, req, resOrSocket) => {
          Log.log('Log.io proxy error: ' + err.message)
          if (typeof resOrSocket?.writeHead === 'function') {
            if (!resOrSocket.headersSent) {
              resOrSocket.writeHead(502, { 'Content-Type': 'text/plain' })
              resOrSocket.end('Log.io server not available')
            }
          } else resOrSocket?.destroy?.()
        },
      },
    })
  )
}

// Reverse proxy for the supervisord web interface ([inet_http_server], port 9000),
// mounted on /supervisor. It is the log viewer / process status page of the linux
// and docker runtimes, where log.io is not installed.
// Supervisor has no notion of a base path, but every url of its ui is relative
// ('index.html?action=...', 'logtail/<name>', 'stylesheets/supervisor.css'), so the
// pages work unchanged as long as the browser resolves them against '/supervisor/'
// - the route redirects the bare mount path to that trailing slash. What is not
// relative is the Location of the redirect answering a POST to its form, rewritten
// below. Basic auth credentials come from the target url and are sent upstream, so
// the browser is never prompted for the supervisor password.
function createSupervisorProxy(supervisorServer) {
  let target
  try {
    target = new URL(supervisorServer)
  } catch (err) {
    Log.log(
      'Invalid supervisor server url: ' +
        supervisorServer +
        ', using the default one'
    )
    target = new URL(SUPERVISOR_DEFAULT_SERVER)
  }

  const auth =
    target.username === ''
      ? undefined
      : decodeURIComponent(target.username) +
        ':' +
        decodeURIComponent(target.password)

  const supervisorAgent = new http.Agent({ keepAlive: true })

  Log.log(
    'Supervisor reverse proxy on ' +
      SUPERVISOR_MOUNT_PATH +
      ' -> ' +
      target.origin +
      '/'
  )

  return withMountedUrl(
    createProxyMiddleware({
      target: target.origin,
      changeOrigin: true,
      ws: false,
      ...(auth ? { auth } : {}),
      pathFilter: mountPathFilter(SUPERVISOR_MOUNT_PATH),
      pathRewrite: { ['^' + SUPERVISOR_MOUNT_PATH]: '' },
      agent: supervisorAgent,
      on: {
        proxyReq: (proxyReq, req, res) => {
          // supervisor only streams 'logtail/<name>' (Tail -f) on a persistent
          // connection (chunked): on 'Connection: close' its medusa server globs
          // the output into 64KB packets, so the tail never shows. A front nginx
          // talks HTTP/1.0 + 'Connection: close' to us, which would be forwarded.
          proxyReq.setHeader('Connection', 'keep-alive')
          // the json/urlencoded body parsers run before this proxy, restore the consumed body
          fixRequestBody(proxyReq, req, res)
        },
        // supervisor answers its POSTs with an absolute Location built from the url
        // it received, which points outside the mount path (and, with changeOrigin,
        // at the supervisor host): put it back under /supervisor.
        proxyRes: (proxyRes) => {
          const location = proxyRes.headers?.location
          if (!location) return
          try {
            const url = new URL(location, target.origin)
            if (url.origin === target.origin)
              proxyRes.headers.location =
                SUPERVISOR_MOUNT_PATH + url.pathname + url.search
          } catch (err) {
            Log.log('Supervisor proxy: unparsable Location ' + location)
          }
        },
        error: (err, req, resOrSocket) => {
          Log.log('Supervisor proxy error: ' + err.message)
          if (typeof resOrSocket?.writeHead === 'function') {
            if (!resOrSocket.headersSent) {
              resOrSocket.writeHead(502, { 'Content-Type': 'text/plain' })
              resOrSocket.end(
                'Supervisor web interface not available. Enable the [inet_http_server] ' +
                  'section of supervisord.conf (port 9000).'
              )
            }
          } else resOrSocket?.destroy?.()
        },
      },
    })
  )
}

// how often an open admin websocket is checked against the current rights
const UPGRADE_RECHECK_MS = 30 * 1000

// A browser always sends the Origin of the page opening a websocket, and a
// page cannot change it: one from another site (also another port of this
// host, which SameSite cookies treat as the same site) is refused. Clients
// that are not browsers send none.
function sameOrigin(req) {
  const origin = req.headers?.origin
  if (origin === undefined) return true
  let host
  try {
    host = new URL(origin).host.toLowerCase()
  } catch (e) {
    return false
  }
  const hosts = [req.headers?.host, req.headers?.['x-forwarded-host']]
    .filter((h) => typeof h === 'string')
    .flatMap((h) => h.split(','))
    .map((h) => h.trim().toLowerCase())
  return hosts.includes(host)
}

// Why an Origin was refused, for the log. A forwarded Host without the port
// of the Origin is the usual cause behind a reverse proxy on another port
// (nginx `proxy_set_header Host $host` drops it, `$http_host` keeps it).
function originRefusalText(req) {
  const origin = String(req.headers?.origin)
  let o
  try {
    o = new URL(origin)
  } catch (e) {
    return 'cross-origin request (invalid Origin)'
  }
  const host = String(req.headers?.host || '').toLowerCase()
  if (o.port && host === o.hostname.toLowerCase())
    return (
      'cross-origin request: Origin ' + o.host + ' vs Host ' + host +
      ' (a reverse proxy dropped the port? use "proxy_set_header Host $http_host" in nginx)'
    )
  return 'cross-origin request: Origin ' + o.host + ' vs Host ' + (host || '(none)')
}

// Proxy the websocket upgrades of a mounted reverse proxy, for admin users.
// Upgrade requests never reach the express middlewares (so verifyToken cannot run on
// them), they are handled on the raw http server and the token is verified here instead.
function attachProxyUpgrade(httpServer, proxy, mountPath, label) {
  const isProxiedRequest = mountPathFilter(mountPath)

  httpServer.on('upgrade', async (req, socket, head) => {
    if (!isProxiedRequest(null, req)) return

    const deny = (status, text) => {
      Log.log(label + ' websocket denied (' + text + ')')
      socket.write('HTTP/1.1 ' + status + '\r\nConnection: close\r\n\r\n')
      socket.destroy()
    }

    if (!sameOrigin(req)) return deny('403 Forbidden', originRefusalText(req))

    const decoded = checkUpgradeToken(req)
    if (decoded === false) return deny('401 Unauthorized', 'absent or invalid access token')

    // the editors and the log viewer are for admin users: check the rights the user
    // has now, not the ones the token was issued with
    let rights = null
    try {
      rights = await UserRights.getCurrentRights(decoded, { fresh: true })
    } catch (err) {
      return deny('503 Service Unavailable', 'user rights unavailable')
    }
    if (!rights) return deny('401 Unauthorized', 'session ended or user not found')
    if (!rights.isAdmin) return deny('403 Forbidden', 'admin rights required')

    // node lowercases incoming header names: replace anything the client sent
    delete req.headers['x-webauth-user']
    if (decoded?.username) req.headers['x-webauth-user'] = decoded.username
    stripSessionToken(req)

    proxy.upgrade(req, socket, head)

    // the connection lasts while the session does: closed when the token
    // expires, is revoked, or the user is no longer an admin (or the rights
    // cannot be read any more)
    const recheck = setInterval(async () => {
      let ok = false
      try {
        const expired = Number(decoded.exp) > 0 && Date.now() >= decoded.exp * 1000
        ok =
          !expired &&
          !!(await UserRights.getCurrentRights(decoded, { fresh: true }))?.isAdmin
      } catch (err) {
        ok = false
      }
      if (!ok) {
        Log.log(label + ' websocket closed (session ended or admin rights removed)')
        clearInterval(recheck)
        socket.destroy()
      }
    }, UPGRADE_RECHECK_MS)
    recheck.unref?.()
    socket.once('close', () => clearInterval(recheck))
  })
}

// Websocket upgrades of the Node-RED editor (<httpAdminRoot>/comms).
function attachNoderedUpgrade(httpServer, noderedProxy) {
  attachProxyUpgrade(httpServer, noderedProxy, NODERED_MOUNT_PATH, 'Node-RED')
}

// Websocket upgrades of the n8n editor push channel (/n8n/rest/push).
function attachN8nUpgrade(httpServer, n8nProxy) {
  attachProxyUpgrade(httpServer, n8nProxy, N8N_MOUNT_PATH, 'n8n')
}

// Websocket upgrades of the log.io ui (/log-io/socket.io).
function attachLogioUpgrade(httpServer, logioProxy) {
  attachProxyUpgrade(httpServer, logioProxy, LOGIO_MOUNT_PATH, 'Log.io')
}

module.exports = {
  stripSessionToken,
  sameOrigin,
  NODERED_MOUNT_PATH,
  NODERED_DEFAULT_SERVER,
  N8N_MOUNT_PATH,
  N8N_DEFAULT_SERVER,
  LOGIO_MOUNT_PATH,
  LOGIO_DEFAULT_SERVER,
  SUPERVISOR_MOUNT_PATH,
  SUPERVISOR_DEFAULT_SERVER,
  createSupervisorProxy,
  mountPathFilter,
  withMountedUrl,
  createNoderedProxy,
  attachNoderedUpgrade,
  createN8nProxy,
  attachN8nUpgrade,
  createLogioProxy,
  attachLogioUpgrade,
}
