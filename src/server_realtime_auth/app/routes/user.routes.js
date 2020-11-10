const path = require('path')
const express = require('express')
const { authJwt } = require('../middlewares')
const controller = require('../controllers/user.controller')

module.exports = function (app, accessPoint, opcApi) {
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

  app.use([authJwt.verifyToken], express.static('../htdocs')) // serve static files

  app.post(accessPoint, opcApi) // realtime data API

  app.get(
    accessPoint + 'test/user',
    [authJwt.verifyToken],
    controller.userBoard
  )


  app.get(accessPoint + 'test/admin', [authJwt.isAdmin], controller.adminBoard)
}
