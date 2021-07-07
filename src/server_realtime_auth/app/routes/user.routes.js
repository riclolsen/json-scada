const path = require('path')
const express = require('express')
const { authJwt } = require('../middlewares')
const controller = require('../controllers/user.controller')

module.exports = function (app, accessPoint, opcApi, accessPointGetFile, getFileApi) {
  app.use(function (req, res, next) {
    res.header(
      'Access-Control-Allow-Headers',
      'x-access-token, Origin, Content-Type, Accept'
    )
    next()
  })

  function sendFavicon (req, res) {
    res
      .status(200)
      .sendFile(
        path.join(__dirname + '../../../../htdocs-login/images/favicon.ico')
      )
  }

  function redirectLogin (req, res) {
    res.redirect('/login/login.html')
  }

  app.get('/favicon.ico', sendFavicon)
  app.get('/', redirectLogin)
  app.get('/index.html', redirectLogin)
  app.get('/login.html', redirectLogin)
  app.get('/login/', redirectLogin)
  app.get('/login/login.html', controller.login)

  app.use('/login', express.static('../htdocs-login'))

  app.use('/admin', [authJwt.isAdmin], express.static('../htdocs-admin/dist'))

  // add charset for special sage displays
  app.use('/sage-cepel-displays/', [authJwt.verifyToken], express.static('../htdocs/sage-cepel-displays', {
    setHeaders: function(res, path) {
      if (/.*\.html/.test(path)) {
        res.set({ 'content-type': 'text/html; charset=iso-8859-1' });
      }
    }
  }));
  app.use([authJwt.verifyToken], express.static('../htdocs')) // serve static files

  app.post(accessPoint, opcApi) // realtime data API

  app.get(accessPointGetFile, getFileApi) // get file from mongo API

  app.get(
    accessPoint + 'test/user',
    [authJwt.verifyToken],
    controller.userBoard
  )


  app.get(accessPoint + 'test/admin', [authJwt.isAdmin], controller.adminBoard)
}
