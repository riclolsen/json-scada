const express = require('express')
const httpProxy = require('express-http-proxy')
const fs = require('fs')
const path = require('path')
const {
  legacyCreateProxyMiddleware: createProxyMiddleware,
} = require('http-proxy-middleware')
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
  metabaseServer
) {
  app.use(function (req, res, next) {
    res.header(
      'Access-Control-Allow-Headers',
      'x-access-token, Origin, Content-Type, Accept'
    )
    next()
  })

  // add charset for special sage displays
  app.use(
    '/sage-cepel-displays/',
    [authJwt.verifyToken],
    express.static('../AdminUI/dist/sage-cepel-displays', {
      setHeaders: function (res, path) {
        if (/.*\.html/.test(path)) {
          res.set({ 'content-type': 'text/html; charset=iso-8859-1' })
        }
      },
    })
  )

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

  // reverse proxy for log.io on Windows
  // for docker it will be used Dozzle
  app.use(
    '/log-io',
    [authJwt.verifyToken],
    function (req, _, next) {
      authController.addXWebAuthUser(req)
      next()
    },
    logioServer.indexOf('//dozzle') === -1
      ? httpProxy(logioServer)
      : createProxyMiddleware({
          target: logioServer,
          changeOrigin: true,
        })
  )
  const wsProxy = createProxyMiddleware({
    target: logioServer,
    changeOrigin: true,
    ws: true, // enable websocket proxy
  })
  app.use(
    '/socket.io',
    [authJwt.verifyToken],
    function (req, _, next) {
      authController.addXWebAuthUser(req)
      next()
    },
    wsProxy
  )
  app.on('upgrade', wsProxy.upgrade)
  app.use('/static', express.static('../log-io/ui/build/static'))

  app.post(accessPoint, opcApi) // realtime data API

  app.get(customJsonQueryAP, customJsonQuery) // custom queries returning JSON

  app.get(accessPointGetFile, getFileApi) // get file from mongo API

  app.get(
    accessPoint + 'test/user',
    [authJwt.verifyToken],
    controller.userBoard
  )

  app.get(accessPoint + 'test/admin', [authJwt.isAdmin], controller.adminBoard)

  app.use('/svg', [authJwt.verifyToken], express.static('../../svg'))

  // production
  app.use('/', express.static('../AdminUI/dist'))
  app.use('/login', express.static('../AdminUI/dist'))
  app.use('/dashboard', express.static('../AdminUI/dist'))
  app.use('/admin', express.static('../AdminUI/dist'))

  // Dynamically create routes for custom developments
  try {
    const customDevPath = path.join(__dirname, '..', '..', '..', 'custom-developments')
    const folders = fs
      .readdirSync(customDevPath)
      .filter((file) =>
        fs.statSync(path.join(customDevPath, file)).isDirectory()
      )

    folders.forEach((folder) => {
      const routePath = `/custom-developments/${folder}`
      let folderPath = path.join(customDevPath, folder, 'dist')
      if (!fs.existsSync(folderPath)) {
        folderPath = path.join(customDevPath, folder)
      }
      app.use(routePath, express.static(folderPath))
      Log.log(`Created static route for: ${routePath}`)
    })
  } catch (error) {
    console.error('Error setting up custom development routes:', error)
  }

  app.use("/custom-developments", (req, res) => {
    try {
        const customDevPath = path.join(__dirname, '..', '..', '..', 'custom-developments');
        
        // Read directory contents
        const items = fs.readdirSync(customDevPath, { withFileTypes: true });
        const folders = items.filter(item => item.isDirectory()).map(item => item.name);

        // Generate HTML response
        const html = `
            <!DOCTYPE html>
            <html>
            <head>
                <title>Custom Developments</title>
                <style>
                    body {
                        font-family: Arial, sans-serif;
                        max-width: 800px;
                        margin: 0 auto;
                        padding: 20px;
                    }
                    h1 {
                        color: #333;
                    }
                    .folder-list {
                        list-style: none;
                        padding: 0;
                    }
                    .folder-list li {
                        margin: 10px 0;
                        padding: 10px;
                        background-color: #f5f5f5;
                        border-radius: 4px;
                    }
                    .folder-list li:hover {
                        background-color: #e0e0e0;
                    }
                    a {
                        color: #2196F3;
                        text-decoration: none;
                    }
                    a:hover {
                        text-decoration: underline;
                    }
                </style>
            </head>
            <body>
                <h1>Custom Developments</h1>
                <ul class="folder-list">
                    ${folders.map(folder => `
                        <li>
                            <a href="/custom-developments/${folder}">${folder}</a>
                        </li>
                    `).join('')}
                </ul>
            </body>
            </html>
        `;

        res.send(html);
    } catch (error) {
        console.error('Error reading custom developments directory:', error);
        res.status(500).send('Error reading custom developments directory');
    }
});

  // app.use('/test', httpProxy('localhost:4321/'))

  // development
  //app.use('/', httpProxy('localhost:3000/'))
  //app.use('/login', httpProxy('localhost:3000/'))
  //app.use('/dashboard', httpProxy('localhost:3000/'))
  //app.use('/admin', httpProxy('localhost:3000/'))
}
