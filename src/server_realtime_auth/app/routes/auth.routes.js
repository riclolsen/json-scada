const { authJwt, verifySignUp } = require('../middlewares')
const controller = require('../controllers/auth.controller')

module.exports = function (app, accessPoint) {
  app.use(function (req, res, next) {
    res.header(
      'Access-Control-Allow-Headers',
      'x-access-token, Origin, Content-Type, Accept'
    )
    next()
  })

  app.post(
    accessPoint + 'auth/signup',
    [
      verifySignUp.checkDuplicateUsernameOrEmail,
      verifySignUp.checkRolesExisted,
    ],
    controller.signup
  )

  app.post(accessPoint + 'auth/signin', controller.signin)
  app.post(accessPoint + 'auth/signout', controller.signout)

  app.use(
    accessPoint + 'auth/listUserActions',
    [authJwt.isAdmin],
    controller.listUserActions
  )
  app.use(
    accessPoint + 'auth/createTag',
    [authJwt.isAdmin],
    controller.createTag
  )
  app.use(
    accessPoint + 'auth/updateTag',
    [authJwt.isAdmin],
    controller.updateTag
  )
  app.use(
    accessPoint + 'auth/deleteTag',
    [authJwt.isAdmin],
    controller.deleteTag
  )
  app.use(accessPoint + 'auth/listTags', [authJwt.isAdmin], controller.listTags)
  app.post(
    accessPoint + 'auth/updateProtocolConnection',
    [authJwt.isAdmin],
    controller.updateProtocolConnection
  )
  app.post(
    accessPoint + 'auth/deleteProtocolConnection',
    [authJwt.isAdmin],
    controller.deleteProtocolConnection
  )
  app.use(
    accessPoint + 'auth/createProtocolConnection',
    [authJwt.isAdmin],
    controller.createProtocolConnection
  )
  app.use(
    accessPoint + 'auth/listProtocolConnections',
    [authJwt.isAdmin],
    controller.listProtocolConnections
  )
  app.use(
    accessPoint + 'auth/listNodes',
    [authJwt.isAdmin],
    controller.listNodes
  )
  app.post(
    accessPoint + 'auth/deleteProtocolDriverInstance',
    [authJwt.isAdmin],
    controller.deleteProtocolDriverInstance
  )
  app.use(
    accessPoint + 'auth/createProtocolDriverInstance',
    [authJwt.isAdmin],
    controller.createProtocolDriverInstance
  )
  app.use(
    accessPoint + 'auth/listProtocolDriverInstances',
    [authJwt.isAdmin],
    controller.listProtocolDriverInstances
  )
  app.post(
    accessPoint + 'auth/updateProtocolDriverInstance',
    [authJwt.isAdmin],
    controller.updateProtocolDriverInstance
  )
  app.use(
    accessPoint + 'auth/listUsers',
    [authJwt.isAdmin],
    controller.listUsers
  )
  app.use(
    accessPoint + 'auth/listRoles',
    [authJwt.isAdmin],
    controller.listRoles
  )
  app.post(
    accessPoint + 'auth/userAddRole',
    [authJwt.isAdmin],
    controller.userAddRole
  )
  app.post(
    accessPoint + 'auth/userRemoveRole',
    [authJwt.isAdmin],
    controller.userRemoveRole
  )
  app.use(
    accessPoint + 'auth/listGroup1',
    [authJwt.isAdmin],
    controller.listGroup1
  )
  app.use(
    accessPoint + 'auth/listDisplays',
    [authJwt.isAdmin],
    controller.listDisplays
  )
  app.post(
    accessPoint + 'auth/updateRole',
    [authJwt.isAdmin],
    controller.updateRole
  )
  app.post(
    accessPoint + 'auth/createRole',
    [authJwt.isAdmin],
    controller.createRole
  )
  app.post(
    accessPoint + 'auth/deleteRole',
    [authJwt.isAdmin],
    controller.deleteRole
  )
  app.post(
    accessPoint + 'auth/createUser',
    [authJwt.isAdmin],
    controller.createUser
  )
  app.post(
    accessPoint + 'auth/deleteUser',
    [authJwt.isAdmin],
    controller.deleteUser
  )
  app.post(
    accessPoint + 'auth/updateUser',
    [authJwt.isAdmin],
    controller.updateUser
  )
}
