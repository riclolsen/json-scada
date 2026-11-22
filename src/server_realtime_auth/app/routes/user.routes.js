const express = require('express')
const httpProxy = require('express-http-proxy')
const fs = require('fs')
const path = require('path')
const { createProxyMiddleware } = require('http-proxy-middleware')
const { withMountedUrl, stripSessionToken } = require('../../proxy-utils')
const { authJwt } = require('../middlewares')
const controller = require('../controllers/user.controller')
const authController = require('../controllers/auth.controller')
const Log = require('../../simple-logger')

module.exports = function (
  app,
  accessPoint,
  opcApi,
  accessPointGetFile,
  getFileApi,
  grafanaServer,
  customJsonQueryAP,
  customJsonQuery,
  logioServer,
  metabaseServer,
  noderedProxy,
  supervisorProxy,
  n8nProxy
) {
  app.use(function (req, res, next) {
    res.header(
      'Access-Control-Allow-Headers',
      'x-access-token, Origin, Content-Type, Accept'
    )
    next()
  })

  // reverse proxy for grafana
  app.use(
    '/grafana',
    [authJwt.verifyToken],
    function (req, _, next) {
      authController.addXWebAuthUser(req)
      next()
    },
    httpProxy(grafanaServer)
  )

  // reverse proxy for metabase
  app.use(
    '/metabase',
    [authJwt.verifyToken],
    function (req, _, next) {
      authController.addXWebAuthUser(req)
      next()
    },
    httpProxy(metabaseServer)
  )

  // reverse proxy for the node-red editor, restricted to admin users (flows can run
  // code on the node-red host). Websocket upgrades are proxied on the http server
  // (see attachNoderedUpgrade), with the same restriction
  app.use(
    '/nodered',
    [authJwt.verifyToken, authJwt.isAdmin],
    function (req, _, next) {
      authController.addXWebAuthUser(req)
      next()
    },
    noderedProxy
  )

  // reverse proxy for the n8n editor, restricted to admin users (workflows can run
  // code on the n8n host); n8n keeps its own login on top of this one.
  // websocket upgrades are proxied on the http server (see attachN8nUpgrade)
  app.use(
    '/n8n',
    [authJwt.verifyToken, authJwt.isAdmin],
    function (req, _, next) {
      authController.addXWebAuthUser(req)
      next()
    },
    n8nProxy
  )

  // reverse proxy for log.io on Windows (admin users only: logs show user names,
  // commands and connection details)
  // for docker it will be used Dozzle
  // this handles the http requests (including the socket.io polling transport), the
  // websocket upgrades of /log-io/socket.io never reach these middlewares and are proxied
  // on the http server instead (see attachLogioUpgrade)
  app.use(
    '/log-io',
    [authJwt.verifyToken, authJwt.isAdmin],
    function (req, _, next) {
      authController.addXWebAuthUser(req)
      next()
    },
    logioServer.indexOf('//dozzle') === -1 ?
      httpProxy(logioServer)
      // dozzle serves itself under /log-io (DOZZLE_BASE), so the mount path that
      // express strips from req.url has to be restored before proxying
    : withMountedUrl(
        createProxyMiddleware({
          target: logioServer,
          changeOrigin: true,
        })
      )
  )

  // Supervisor web interface (linux/docker log viewer), mounted on /supervisor.
  // Process control is offered there, so it is restricted to admin users:
  // verifyToken answers the requests without a token, isAdmin the ones of a
  // non-admin user.
  // The pages of the ui reference their assets relatively, so they only resolve
  // when the browser is on '/supervisor/' - send the bare mount path there.
  app.get('/supervisor', (req, res, next) => {
    if (req.originalUrl.split('?')[0] !== '/supervisor') return next()
    const query = req.originalUrl.slice('/supervisor'.length)
    res.redirect('/supervisor/' + query)
  })
  app.use(
    '/supervisor',
    [authJwt.verifyToken, authJwt.isAdmin],
    (req, _, next) => {
      stripSessionToken(req)
      next()
    },
    supervisorProxy
  )

  // Entry point of the AdminUI log viewer: log.io on windows, where it is part of
  // the installation, Dozzle (served on /log-io) when it is the log server, as in
  // the docker compose demo, which has no supervisord, otherwise the supervisor web
  // interface on linux and in the docker image, where supervisord runs the processes.
  app.get('/log-viewer-ui', (req, res) => {
    res.redirect(
      process.platform === 'win32' || logioServer.indexOf('//dozzle') !== -1
        ? '/log-io/'
        : '/supervisor/'
    )
  })

  app.use('/static', express.static('../log-io/ui/build/static'))

  app.post(accessPoint, [authJwt.verifyToken], opcApi) // realtime data API

  app.get(customJsonQueryAP, [authJwt.verifyToken], customJsonQuery) // custom queries returning JSON

  app.get(accessPointGetFile, [authJwt.verifyToken], getFileApi) // get file from mongo API

  app.get(
    accessPoint + 'test/user',
    [authJwt.verifyToken],
    controller.userBoard
  )

  app.get(accessPoint + 'test/admin', [authJwt.isAdmin], controller.adminBoard)

  // displays (svg files) a user may open: the displayList of its roles, all of
  // them when the list is empty or the user is an admin (other files under /svg,
  // like images and scripts, are not restricted)
  const displayGuard = (req, res, next) => {
    const list = req.rights?.displayList
    if (req.rights?.isAdmin || !Array.isArray(list) || list.length === 0)
      return next()
    let name = ''
    try {
      name = decodeURIComponent(path.basename(req.path))
    } catch (e) {
      return res.status(400).send({ error: 'Bad request' })
    }
    // Windows opens 'a.svg.', 'a.svg ' and 'a.svg::$DATA' as 'a.svg': such
    // names would pass the extension test below without being in the list
    if (/[:\\]|[. ]$/.test(name)) {
      Log.log('Display access denied: ' + name)
      return res.status(403).send({ error: 'Access to display denied!' })
    }
    if (!/\.svg$/i.test(name) || list.includes(name)) return next()
    Log.log('Display access denied: ' + name)
    res.status(403).send({ error: 'Access to display denied!' })
  }
  app.use('/svg', [authJwt.verifyToken, displayGuard], express.static('../../svg'))

  app.use(
    '/svgedit',
    [authJwt.verifyToken],
    express.static('../svgedit/dist/editor')
  )

  // production
  // the Vite build names every file in assets/ after its content hash: they
  // never change, so browsers keep them (index.html is revalidated each time)
  app.use(
    '/assets',
    express.static('../AdminUI/dist/assets', {
      maxAge: '365d',
      immutable: true,
      fallthrough: false,
    })
  )
  app.use('/', express.static('../AdminUI/dist'))
  app.use('/login', express.static('../AdminUI/dist'))
  app.use('/dashboard', express.static('../AdminUI/dist'))
  app.use('/admin', express.static('../AdminUI/dist'))

  const escapeHtml = (t) =>
    String(t).replace(
      /[&<>"']/g,
      (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]
    )

  // Directory served by the custom developments routes below.
  const customDevPath = path.join(
    __dirname,
    '..',
    '..',
    '..',
    'custom-developments'
  )

  // Directory to serve for a custom development, or null when it has nothing to
  // serve. The built app (dist) is preferred, the folder itself is accepted for
  // examples that ship plain html. A folder with neither (a work in progress, or
  // an example whose build failed) is not routed and not listed: routing it would
  // fall through to the index listing below and look like the link does nothing.
  function customDevContentPath(folder) {
    return (
      [path.join(customDevPath, folder, 'dist'), path.join(customDevPath, folder)].find(
        (p) => fs.existsSync(path.join(p, 'index.html'))
      ) || null
    )
  }

  // Title, description and build time of a routed custom development, read from
  // the <title> and <meta name="description"> of its index.html (first 64 KB).
  // The Astro template placeholder description is left out.
  function customDevInfo(folder) {
    const info = { folder, title: '', description: '', modified: null }
    try {
      const file = path.join(customDevContentPath(folder), 'index.html')
      info.modified = fs.statSync(file).mtime.toISOString()
      const fd = fs.openSync(file, 'r')
      const buf = Buffer.alloc(65536)
      const n = fs.readSync(fd, buf, 0, buf.length, 0)
      fs.closeSync(fd)
      const head = buf.toString('utf8', 0, n)
      const unescape = (s) =>
        s
          .replace(/&lt;/g, '<')
          .replace(/&gt;/g, '>')
          .replace(/&quot;/g, '"')
          .replace(/&#39;/g, "'")
          .replace(/&amp;/g, '&')
          .trim()
          .slice(0, 300)
      const t = head.match(/<title[^>]*>([^<]*)<\/title>/i)
      if (t) info.title = unescape(t[1])
      const d =
        head.match(/<meta\s+name=["']description["']\s+content=["']([^"']*)["']/i) ||
        head.match(/<meta\s+content=["']([^"']*)["']\s+name=["']description["']/i)
      if (d && !/^astro description$/i.test(d[1].trim())) info.description = unescape(d[1])
    } catch (err) {
      Log.log(`Custom development '${folder}': ${err.message}`)
    }
    return info
  }

  // Dynamically create routes for custom developments
  try {
    const folders = fs
      .readdirSync(customDevPath)
      .filter((file) =>
        fs.statSync(path.join(customDevPath, file)).isDirectory()
      )

    folders.forEach((folder) => {
      const routePath = `/custom-developments/${folder}`
      const folderPath = customDevContentPath(folder)
      if (!folderPath) {
        Log.log(
          `Custom development '${folder}' has no index.html (not built?): route ${routePath} not created`
        )
        return
      }
      app.use(routePath, authJwt.isUserPage, express.static(folderPath))
      Log.log(`Created static route for: ${routePath}`)
    })
  } catch (error) {
    console.error('Error setting up custom development routes:', error)
  }

  // the apps and their list are for signed-in users (the data they read is
  // restricted by the user's rights anyway)
  app.use('/custom-developments', authJwt.isUserPage, (req, res) => {
    try {
      // A request for a specific example that got here matched no static route,
      // i.e. the example is not built. Answer 404 instead of rendering the index
      // again, which would look like the link simply does not open.
      if (req.path !== '/') {
        const name = req.path.split('/')[1] || ''
        return res
          .status(404)
          .send(
            `Custom development '${name}' is not available: no built content found in ` +
              `src/custom-developments/${name}/dist (run 'npm install && npm run build' there).`
          )
      }

      // Read directory contents, listing only what is actually routed above.
      const items = fs.readdirSync(customDevPath, { withFileTypes: true })
      const folders = items
        .filter((item) => item.isDirectory())
        .map((item) => item.name)
        .filter((folder) => customDevContentPath(folder) !== null)
        .sort((a, b) => a.localeCompare(b))
      const apps = folders.map(customDevInfo)

      // the AdminUI page (CustomDevelopmentsPage.vue) reads the list as JSON
      if (req.query.format === 'json') {
        res.set('Cache-Control', 'no-store')
        return res.status(200).json({ apps })
      }

      // plain page for direct visits: follows ?theme=light|dark, else the
      // browser's color scheme
      const theme = req.query.theme === 'dark' || req.query.theme === 'light' ? req.query.theme : ''
      const html = `<!DOCTYPE html>
<html lang="en"${theme ? ` data-theme="${theme}"` : ''}>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Custom Developments | {json:scada}</title>
<style>
  :root { color-scheme: light dark; --bg: #f6f7f9; --card: #fff; --fg: #1b1f24; --muted: #5f6b7a; --line: #dde1e6; --accent: #1f6feb; }
  @media (prefers-color-scheme: dark) { :root:not([data-theme="light"]) { --bg: #121417; --card: #1c1f24; --fg: #e6e8eb; --muted: #9aa4b2; --line: #2d323a; --accent: #6ea8fe; } }
  :root[data-theme="dark"] { --bg: #121417; --card: #1c1f24; --fg: #e6e8eb; --muted: #9aa4b2; --line: #2d323a; --accent: #6ea8fe; }
  * { box-sizing: border-box; }
  body { margin: 0; font: 15px/1.5 system-ui, -apple-system, "Segoe UI", Roboto, sans-serif; background: var(--bg); color: var(--fg); }
  main { max-width: 1100px; margin: 0 auto; padding: 32px 20px; }
  h1 { font-size: 22px; font-weight: 600; margin: 0 0 4px; }
  .sub { color: var(--muted); margin: 0 0 24px; }
  .grid { display: grid; gap: 16px; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); }
  a.card { display: flex; flex-direction: column; gap: 6px; padding: 18px; border: 1px solid var(--line); border-radius: 12px; background: var(--card); color: inherit; text-decoration: none; transition: border-color .15s, transform .15s; }
  a.card:hover, a.card:focus-visible { border-color: var(--accent); transform: translateY(-2px); outline: none; }
  .title { font-weight: 600; }
  .folder { font: 12px ui-monospace, Consolas, monospace; color: var(--accent); }
  .desc { color: var(--muted); font-size: 14px; }
  .empty { padding: 40px; text-align: center; color: var(--muted); border: 1px dashed var(--line); border-radius: 12px; }
</style>
</head>
<body>
<main>
  <h1>Custom Developments</h1>
  <p class="sub">Web apps served from src/custom-developments</p>
  ${
    apps.length
      ? `<div class="grid">${apps
          .map(
            (a) => `
    <a class="card" href="/custom-developments/${encodeURIComponent(a.folder)}/">
      <span class="title">${escapeHtml(a.title || a.folder)}</span>
      <span class="folder">${escapeHtml(a.folder)}</span>
      ${a.description ? `<span class="desc">${escapeHtml(a.description)}</span>` : ''}
    </a>`
          )
          .join('')}
  </div>`
      : `<div class="empty">No custom development is built yet: run 'npm install &amp;&amp; npm run build' in a folder of src/custom-developments and restart server_realtime_auth.</div>`
  }
</main>
</body>
</html>`

      res.send(html)
    } catch (error) {
      console.error('Error reading custom developments directory:', error)
      res.status(500).send('Error reading custom developments directory')
    }
  })

  // app.use('/test', httpProxy('localhost:4321/'))

  // development
  //app.use('/', httpProxy('localhost:3000/'))
  //app.use('/login', httpProxy('localhost:3000/'))
  //app.use('/dashboard', httpProxy('localhost:3000/'))
  //app.use('/admin', httpProxy('localhost:3000/'))
}
