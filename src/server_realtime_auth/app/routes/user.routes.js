const path = require('path')
const express = require('express')
const httpProxy = require('express-http-proxy')
const { legacyCreateProxyMiddleware: createProxyMiddleware } = require('http-proxy-middleware');
const controller = require('../controllers/user.controller')
const authController = require('../controllers/auth.controller')

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
    express.static('../htdocs/sage-cepel-displays', {
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

  // reverse proxy for log.io
  app.use(
    '/log-io',
    [authJwt.verifyToken],
    function (req, _, next) {
      authController.addXWebAuthUser(req)
      next()
    },
    httpProxy(logioServer)
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
  app.use('/static', express.static('../log-io/ui/build/static'));

  app.post(accessPoint, opcApi) // realtime data API

  app.get(customJsonQueryAP, customJsonQuery) // custom queries returning JSON

  app.get(accessPointGetFile, getFileApi) // get file from mongo API

  app.get(
    accessPoint + 'test/user',
    [authJwt.verifyToken],
    controller.userBoard
  )

  app.get(accessPoint + 'test/admin', [authJwt.isAdmin], controller.adminBoard)

  // production
  app.use('/', express.static('../AdminUI/dist'))
  app.use('/login', express.static('../AdminUI/dist'))
  app.use('/dashboard', express.static('../AdminUI/dist'))
  app.use('/admin', express.static('../AdminUI/dist'))
  app.use('/svg', express.static('../../svg'))

  // development
  //app.use('/', httpProxy('localhost:3000/'))
  //app.use('/login', httpProxy('localhost:3000/'))
  //app.use('/dashboard', httpProxy('localhost:3000/'))
  //app.use('/admin', httpProxy('localhost:3000/'))
}
