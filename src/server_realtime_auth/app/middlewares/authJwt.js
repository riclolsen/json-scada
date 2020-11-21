const jwt = require('jsonwebtoken')
const config = require('../config/auth.config.js')
const db = require('../models')
const User = db.user
const Role = db.role

let OpcResp = {
  NamespaceUris: [
    'urn:opcf-apps-01:UA:Quickstarts:ReferenceServer',
    'http://opcfoundation.org/Quickstarts/ReferenceApplications',
    'http://opcfoundation.org/UA/Diagnostics'
  ],
  ServerUris: [],
  ServiceId: 395,
  Body: {
    ResponseHeader: {
      RequestHandle: null,
      Timestamp: '',
      ServiceDiagnostics: { LocalizedText: 0 },
      StringTable: [],
      ServiceResult: 0
    }
  }
}

verifyToken = (req, res, next) => {
  console.log('Verify ' + req.originalUrl)

  let reqHandle = null
  if (req.method === 'POST')
    reqHandle = req?.body?.Body?.RequestHeader?.RequestHandle

  let token = req.headers['x-access-token'] || req.cookies['x-access-token']

  if (!token) {
    if (reqHandle) {
      OpcResp.Body.ResponseHeader.RequestHandle = reqHandle
      OpcResp.Body.ResponseHeader.StringTable = [
        'BadIdentityTokenInvalid',
        'Access denied (absent access token)!'
      ]
      OpcResp.Body.ResponseHeader.ServiceResult = 0x80200000 // BadIdentityTokenInvalid
      return res.status(200).send(OpcResp)
    }
    return res
      .status(200)
      .send({ ok: false, message: 'Access not allowed. No token provided' })
  }

  jwt.verify(token, config.secret, (err, decoded) => {
    if (err) {
      if (reqHandle) {
        OpcResp.Body.ResponseHeader.RequestHandle = reqHandle
        OpcResp.Body.ResponseHeader.StringTable = [
          'BadIdentityTokenRejected',
          'Access denied (access token rejected)!'
        ]
        OpcResp.Body.ResponseHeader.ServiceResult = 0x80210000 // BadIdentityTokenRejected
        return res.status(200).send(OpcResp)
      }
      return res.status(200).send({
        ok: false,
        message: 'Access not allowed. ' + err,
        decoded: decoded
      })
    }
    req.userId = decoded.id
    next()
  })
}

isAdmin = (req, res, next) => {
  console.log('isAdmin')

  let tok = checkToken(req)
  if (tok === false) return false

  User.findOne({ username: tok.username }).exec((err, user) => {
    if (err) {
      res.status(500).send({ message: err })
      return
    }

    Role.find(
      {
        _id: { $in: user.roles }
      },
      (err, roles) => {
        if (err) {
          res.status(500).send({ message: err })
          return
        }

        for (let i = 0; i < roles.length; i++) {
          if (roles[i].isAdmin) {
            next()
            return
          }
        }

        res.status(403).send({ message: 'Require isAdmin rights!' })
        return
      }
    )
  })
}

// check and decoded token
checkToken = req => {
  let res = false

  console.log('CheckToken')
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

// User in request can send commands?
canSendCommands = async req => {
  console.log('canSendCommands?')

  try {
    const user = await User.findById(req.userId).exec()

    const roles = await Role.find({
      _id: { $in: user.roles }
    }).exec()

    for (let i = 0; i < roles.length; i++) {
      if (roles[i].sendCommands) {
        return true
      }
    }
  } catch (err) {
    console.log(err)
  }

  return false
}

// User in request can send commands to a group1 location?
canSendCommandTo = async (req, group1) => {
  console.log('canSendCommandTo?')
  let result = true

  try {
    const user = await User.findById(req.userId).exec()

    const roles = await Role.find({
      _id: { $in: user.roles }
    }).exec()

    if (roles.length == 0) return false

    for (let i = 0; i < roles.length; i++) {
      if (roles[i].group1CommandList.length > 0) {
        // has a list, so in principle deny command
        result = false
      }
      if ( roles[i].group1CommandList.includes(group1) ){
        console.log('User can command!')
        return true
      }
    }
    if (result)
       console.log('User can command!')
    else
       console.log('User has no right to issue commands!')
    return result
  } catch (err) {
    console.log(err)
  }

  console.log('User has no right to issue commands!')
  return false
}

// test user right
hasRight = (req, right) => {
  console.log('hasRight')

  let res = this.checkToken(req)
  if (res === false) return res

  if (right in res) return res[right]

  return false
}

const authJwt = {
  verifyToken,
  checkToken,
  hasRight,
  isAdmin,
  canSendCommands,
  canSendCommandTo
}
module.exports = authJwt
