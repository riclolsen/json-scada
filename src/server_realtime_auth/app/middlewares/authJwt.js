const jwt = require('jsonwebtoken')
const config = require('../config/auth.config.js')
const db = require('../models')
const User = db.user
const Role = db.role

let OpcResp = {
  "NamespaceUris": [
    "urn:opcf-apps-01:UA:Quickstarts:ReferenceServer",
    "http://opcfoundation.org/Quickstarts/ReferenceApplications",
    "http://opcfoundation.org/UA/Diagnostics"
  ],
  "ServerUris": [],
  "ServiceId": 395,
  "Body": {
    "ResponseHeader": {
      "RequestHandle": null,
      "Timestamp": "",
      "ServiceDiagnostics": { "LocalizedText": 0 },
      "StringTable": [],
      "ServiceResult": 0
    }
  }
}

verifyToken = (req, res, next) => {
  console.log("Verify " + req.originalUrl)

  let reqHandle = null
  if (req.method === "POST")
    reqHandle = req?.body?.Body?.RequestHeader?.RequestHandle

  let token = req.headers['x-access-token'] || req.cookies['x-access-token']

  if (!token) {
    if (reqHandle) {
      OpcResp.Body.ResponseHeader.RequestHandle = reqHandle
      OpcResp.Body.ResponseHeader.StringTable = [
        "BadIdentityTokenInvalid",
        "Access denied (absent access token)!"
      ]
      OpcResp.Body.ResponseHeader.ServiceResult = 0x80200000 // BadIdentityTokenInvalid
      return res.status(200).send(OpcResp)
    }
    return res.status(200).send({ ok: false, message: "Access not allowed. No token provided" })
  }

  jwt.verify(token, config.secret, (err, decoded) => {
    if (err) {
      if (reqHandle) {
        OpcResp.Body.ResponseHeader.RequestHandle = reqHandle
        OpcResp.Body.ResponseHeader.StringTable = [
          "BadIdentityTokenRejected",
          "Access denied (access token rejected)!"
        ]
        OpcResp.Body.ResponseHeader.ServiceResult = 0x80210000 // BadIdentityTokenRejected
        return res.status(200).send(OpcResp)
      }
      return res.status(200).send({ ok: false, message: "Access not allowed. " + err, decoded: decoded })
    }
    req.userId = decoded.id
    next()
  })
}

isAdmin = (req, res, next) => {
  
  console.log("isAdmin")
  
  let tok = checkToken(req)
  if (tok === false)
    return false

  User.findOne({username: tok.username}).exec((err, user) => {
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

isModerator = (req, res, next) => {
  console.log("isModerator")
  User.findById(req.userId).exec((err, user) => {
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
          if (roles[i].name === 'moderator') {
            next()
            return
          }
        }

        res.status(403).send({ message: 'Require Moderator Role!' })
        return
      }
    )
  })
}

canSendCommands = (req, res, next) => {
  console.log("canSendCommands")
  User.findById(req.userId).exec((err, user) => {
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
          if (roles[i].sendCommands) {
            next()
            return
          }
        }

        res.status(403).send({ message: 'Require sendCommand right' })
        return
      }
    )
  })
}

// check and decoded token
checkToken = req => {
  let res = false

  console.log("Check")
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

// test user right
hasRight = (req, right) => {
  console.log("hasRight")

  let res = this.checkToken(req)
  if ( res === false )
    return res
  
  if (right in res)
    return res[right];

  return false;
}

const authJwt = {
  verifyToken,
  checkToken,
  hasRight,
  isAdmin,
  isModerator
}
module.exports = authJwt
