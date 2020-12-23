const config = require('../config/auth.config')
const db = require('../models')
const fs = require('fs')
var path = require('path')
const User = db.user
const Role = db.role
const Tag = db.tag
const ProtocolDriverInstance = db.protocolDriverInstance
const ProtocolConnection = db.protocolConnection
const UserActionsQueue = require('../../userActionsQueue')

var jwt = require('jsonwebtoken')
var bcrypt = require('bcryptjs')

exports.updateProtocolConnection = async (req, res) => {
  registerUserAction(req, 'updateProtocolConnection')

  // make default bind address for some protocols
  if (
    ['IEC60870-5-104_SERVER', 'DNP3_SERVER'].includes(
      req?.body?.protocolDriver
    ) &&
    ( !('ipAddressLocalBind' in req.body) || req.body.ipAddressLocalBind == '' )
  ) {
    req.body.ipAddressLocalBind = '0.0.0.0'
  }

  await ProtocolConnection.findOneAndUpdate({ _id: req.body._id }, req.body)
  res.status(200).send({})
}

exports.deleteProtocolConnection = async (req, res) => {
  registerUserAction(req, 'deleteProtocolConnection')

  await ProtocolConnection.findOneAndDelete({ _id: req.body._id }, req.body)
  res.status(200).send({ error: false })
}

exports.createProtocolConnection = async (req, res) => {
  // find the biggest connection number and increment for the new connection
  await ProtocolConnection.find({}).exec(function (err, protocolConnections) {
    if (err) {
      res.status(200).send({ error: err })
      return
    }
    let connNumber = 0
    protocolConnections.forEach(element => {
      if (element.protocolConnectionNumber > connNumber)
        connNumber = element.protocolConnectionNumber
    })

    registerUserAction(req, 'createProtocolConnection')
    const protocolConnection = new ProtocolConnection()
    protocolConnection.protocolConnectionNumber = connNumber + 1
    protocolConnection.DriverInstanceNumber = 1
    protocolConnection.save(err => {
      if (err) {
        res.status(200).send({ error: err })
        return
      }
      res.status(200).send({ error: false })
    })
  })
}

exports.listProtocolConnections = (req, res) => {
  console.log('listProtocolConnections')

  ProtocolConnection.find({}).exec(function (err, protocolConnections) {
    if (err) {
      res.status(200).send({ error: err })
      return
    }
    res.status(200).send(protocolConnections)
  })
}

exports.deleteProtocolDriverInstance = async (req, res) => {
  registerUserAction(req, 'deleteProtocolDriverInstance')

  await ProtocolDriverInstance.findOneAndDelete({ _id: req.body._id }, req.body)
  res.status(200).send({ error: false })
}

exports.listNodes = (req, res) => {
  console.log('listNodes')

  ProtocolDriverInstance.find({}).exec(function (err, driverInstances) {
    if (err) {
      res.status(200).send({ error: err })
      return
    }
    let listNodes = []
    driverInstances.map(element => {
      listNodes = listNodes.concat(element.nodeNames)
    })
    res.status(200).send(listNodes)
  })
}

exports.createProtocolDriverInstance = async (req, res) => {
  registerUserAction(req, 'createProtocolDriverInstance')
  const driverInstance = new ProtocolDriverInstance()
  driverInstance.save(err => {
    if (err) {
      res.status(200).send({ error: err })
      return
    }
    res.status(200).send({ error: false })
  })
}

exports.listProtocolDriverInstances = (req, res) => {
  console.log('listProtocolDriverInstances')

  ProtocolDriverInstance.find({}).exec(function (err, driverInstances) {
    if (err) {
      res.status(200).send({ error: err })
      return
    }
    res.status(200).send(driverInstances)
  })
}

exports.updateProtocolDriverInstance = async (req, res) => {
  registerUserAction(req, 'updateProtocolDriverInstance')
  await ProtocolDriverInstance.findOneAndUpdate({ _id: req.body._id }, req.body)
  res.status(200).send({})
}

exports.listUsers = (req, res) => {
  console.log('listUsers')

  User.find({})
    .populate('roles')
    .exec(function (err, users) {
      if (err) {
        res.status(200).send({ error: err })
        return
      }
      users.forEach(user => {
        user.password = null
      })
      res.status(200).send(users)
    })
}

exports.listRoles = (req, res) => {
  console.log('listRoles')

  Role.find({}).exec(function (err, roles) {
    if (err) {
      res.status(200).send({ error: err })
      return
    }
    res.status(200).send(roles)
  })
}

exports.userAddRole = (req, res) => {
  registerUserAction(req, 'userAddRole')
  let role = Role.findOne({ name: req.body.role }).exec(function (err, role) {
    if (err || !role) {
      res.status(200).send({ error: err })
      return
    }
    User.findOne({ username: req.body.username }).exec(function (err, user) {
      if (err || !user) {
        res.status(200).send({ error: err })
        return
      }
      user.roles.push(role._id)
      user.save()
      res.status(200).send({ error: false })
    })
  })
}

exports.userRemoveRole = (req, res) => {
  registerUserAction(req, 'userRemoveRole')

  let role = Role.findOne({ name: req.body.role }).exec(function (err, role) {
    if (err || !role) {
      res.status(200).send({ error: err })
      return
    }
    User.findOne({ username: req.body.username }).exec(function (err, user) {
      if (err || !user) {
        res.status(200).send({ error: err })
        return
      }
      user.roles.pull(role._id)
      user.save()
      res.status(200).send({ error: false })
    })
  })
}

exports.listGroup1 = (req, res) => {
  console.log('listGroup1')

  Tag.find().distinct('group1', function (err, groups) {
    if (err) {
      res.status(200).send({ error: err })
      return
    }
    res.status(200).send(groups)
  })
}

exports.listDisplays = (req, res) => {
  console.log('listDisplays')

  fs.readdir('../htdocs/svg', function (err, files) {
    //handling error
    if (err) {
      res.status(200).send({ error: err })
      return
    }

    let svgFiles = files.filter(function (file) {
      return path.extname(file).toLowerCase() === '.svg'
    })

    res.status(200).send(svgFiles)
  })
}

exports.updateRole = async (req, res) => {
  registerUserAction(req, 'updateRole')

  await Role.findOneAndUpdate({ _id: req.body._id }, req.body)
  res.status(200).send({})
}

exports.updateUser = async (req, res) => {
  registerUserAction(req, 'updateUser')
  if (
    'password' in req.body &&
    req.body.password !== '' &&
    req.body.password !== null
  )
    req.body.password = bcrypt.hashSync(req.body.password, 8)
  else delete req.body['password']
  delete req.body['roles']
  await User.findOneAndUpdate({ _id: req.body._id }, req.body)
  res.status(200).send({})
}

exports.createRole = async (req, res) => {
  registerUserAction(req, 'createRole')

  const role = new Role()
  role.save(err => {
    if (err) {
      res.status(200).send({ error: err })
      return
    }
    res.status(200).send({ error: false })
  })
}

exports.createUser = async (req, res) => {
  registerUserAction(req, 'createUser')

  if (req.body.password && req.body.password !== '')
    req.body.password = bcrypt.hashSync(req.body.password, 8)
  const user = new User(req.body)
  user.save(err => {
    if (err) {
      res.status(200).send({ error: err })
      return
    }
    res.status(200).send({ error: false })
  })
}

exports.deleteRole = async (req, res) => {
  registerUserAction(req, 'deleteRole')

  await Role.findOneAndDelete({ _id: req.body._id }, req.body)
  res.status(200).send({ error: false })
}

exports.deleteUser = async (req, res) => {
  registerUserAction(req, 'deleteUser')

  await User.findOneAndDelete({ _id: req.body._id }, req.body)
  res.status(200).send({ error: false })
}

// create user profile passing username, email, password and roles
exports.signup = (req, res) => {
  const user = new User({
    username: req.body.username,
    email: req.body.email,
    password: bcrypt.hashSync(req.body.password, 8)
  })

  registerUserAction(req, 'signup')

  user.save((err, user) => {
    if (err) {
      res.status(500).send({ message: err })
      return
    }

    if (req.body.roles) {
      Role.find(
        {
          name: { $in: req.body.roles }
        },
        (err, roles) => {
          if (err) {
            res.status(500).send({ message: err })
            return
          }

          user.roles = roles.map(role => role._id)
          user.save(err => {
            if (err) {
              res.status(500).send({ message: err })
              return
            }

            res.send({ message: 'User was registered successfully!' })
          })
        }
      )
    } else {
      Role.findOne({ name: 'user' }, (err, role) => {
        if (err) {
          res.status(500).send({ message: err })
          return
        }

        user.roles = [role._id]
        user.save(err => {
          if (err) {
            res.status(500).send({ message: err })
            return
          }

          res.send({ message: 'User was registered successfully!' })
        })
      })
    }
  })
}

// User signin request
// Will check for valid user and then create a JWT access token
exports.signin = (req, res) => {
  User.findOne({
    username: req.body.username
  })
    .populate('roles', '-__v')
    .exec((err, user) => {
      if (err) {
        res.status(200).send({ ok: false, message: err })
        return
      }

      if (!user) {
        return res.status(200).send({ ok: false, message: 'User Not found.' })
      }

      var passwordIsValid = bcrypt.compareSync(req.body.password, user.password)

      if (!passwordIsValid) {
        return res
          .status(200)
          .cookie('x-access-token', null)
          .send({
            ok: false,
            message: 'Invalid Password!'
          })
      }

      // Combines all roles rights for the user
      var authorities = []
      var rights = {
        isAdmin: false,
        changePassword: false,
        sendCommands: false,
        enterAnnotations: false,
        enterNotes: false,
        enterManuals: false,
        enterLimits: false,
        substituteValues: false,
        ackEvents: false,
        ackAlarms: false,
        disableAlarms: false,
        group1List: [],
        group1CommandList: [],
        displayList: [],
        maxSessionDays: 0.0
      }
      for (let i = 0; i < user.roles.length; i++) {
        authorities.push(user.roles[i].name)
        if ('isAdmin' in user.roles[i])
          rights.isAdmin = rights.isAdmin || user.roles[i].isAdmin
        if ('changePassword' in user.roles[i])
          rights.changePassword =
            rights.changePassword || user.roles[i].changePassword
        if ('sendCommands' in user.roles[i])
          rights.sendCommands =
            rights.sendCommands || user.roles[i].sendCommands
        if ('enterAnnotations' in user.roles[i])
          rights.enterAnnotations =
            rights.enterAnnotations || user.roles[i].enterAnnotations
        if ('enterNotes' in user.roles[i])
          rights.enterNotes = rights.enterNotes || user.roles[i].enterNotes
        if ('enterManuals' in user.roles[i])
          rights.enterManuals =
            rights.enterManuals || user.roles[i].enterManuals
        if ('enterLimits' in user.roles[i])
          rights.enterLimits = rights.enterLimits || user.roles[i].enterLimits
        if ('substituteValues' in user.roles[i])
          rights.substituteValues =
            rights.substituteValues || user.roles[i].substituteValues
        if ('ackEvents' in user.roles[i])
          rights.ackEvents = rights.ackEvents || user.roles[i].ackEvents
        if ('ackAlarms' in user.roles[i])
          rights.ackAlarms = rights.ackAlarms || user.roles[i].ackAlarms
        if ('disableAlarms' in user.roles[i])
          rights.disableAlarms =
            rights.disableAlarms || user.roles[i].disableAlarms
        if ('group1List' in user.roles[i])
          rights.group1List = rights.group1List.concat(user.roles[i].group1List)
        if ('group1CommandList' in user.roles[i])
          rights.group1CommandList = rights.group1CommandList.concat(
            user.roles[i].group1CommandList
          )
        if ('displayList' in user.roles[i])
          rights.displayList = rights.displayList.concat(
            user.roles[i].displayList
          )
        if ('maxSessionDays' in user.roles[i])
          if (user.roles[i].maxSessionDays > rights.maxSessionDays)
            rights.maxSessionDays = user.roles[i].maxSessionDays
      }

      var token = jwt.sign(
        { id: user.id, username: user.username, rights: rights },
        config.secret,
        {
          expiresIn: rights.maxSessionDays * 86400 // days*24 hours
        }
      )

      // register user action
      registerUserAction(req, 'signin')

      // return the access token in a cookie unaccessible to javascript (http only)
      // also return a cookie with plain user data accessible to the client side scripts
      res
        .status(200)
        .cookie('x-access-token', token, {
          httpOnly: true,
          secure: false,
          maxAge: (1 + rights.maxSessionDays) * 86400 * 1000
        })
        .cookie(
          'json-scada-user',
          JSON.stringify({
            id: user._id,
            username: user.username,
            email: user.email,
            roles: authorities,
            rights: rights
          }),
          {
            httpOnly: false,
            secure: false,
            maxAge: (1 + 2 * rights.maxSessionDays) * 86400 * 1000
          }
        )
        .send({ ok: true, message: 'Signed In' })
    })
}

// Sign out: eliminate the cookie with access token
exports.signout = (req, res) => {
  registerUserAction(req, 'signout')

  return res
    .status(200)
    .cookie('x-access-token', null, {
      httpOnly: true,
      secure: false,
      maxAge: 0
    })
    .send({
      ok: true,
      message: 'Signed Out!'
    })
}

// check and decoded token
checkToken = req => {
  let res = false
  let token = req.headers['x-access-token'] || req.cookies['x-access-token']
  if (!token) {
    return res
  }
  jwt.verify(token, config.secret, (err, decoded) => {
    if (err) {
      return res
    }
    res = decoded
  })
  return res
}

// enqueue use action for later insertion to mongodb
function registerUserAction (req, actionName) {
  let body = {}
  Object.assign(body, req.body)

  let ck = checkToken(req)
  if (ck !== false) {
    console.log(actionName + ' - ' + ck?.username)
    delete body['password']
    // register user action
    UserActionsQueue.enqueue({
      username: ck?.username,
      request: body,
      action: actionName,
      timeTag: new Date()
    })
  } else {
    console.log(actionName + ' - ' + req.body?.username)
    delete body['password']
    // register user action
    UserActionsQueue.enqueue({
      username: req.body?.username,
      request: body,
      action: actionName,
      timeTag: new Date()
    })
  }
}
