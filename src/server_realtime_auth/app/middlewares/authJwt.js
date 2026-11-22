const Log = require('../../simple-logger')
const jwt = require('jsonwebtoken')
const config = require('../config/auth.config.js')
const UserRights = require('../services/user-rights')

const OPC_BAD_IDENTITY_TOKEN_INVALID = 0x80200000
const OPC_BAD_IDENTITY_TOKEN_REJECTED = 0x80210000
const OPC_BAD_SERVER_NOT_CONNECTED = 0x800d0000

// OPC-style answer for the realtime API (the request carries a RequestHandle)
function opcDenied(reqHandle, statusName, text, code) {
  return {
    NamespaceUris: [
      'urn:opcf-apps-01:UA:Quickstarts:ReferenceServer',
      'http://opcfoundation.org/Quickstarts/ReferenceApplications',
      'http://opcfoundation.org/UA/Diagnostics',
    ],
    ServerUris: [],
    ServiceId: 395,
    Body: {
      ResponseHeader: {
        RequestHandle: reqHandle,
        Timestamp: '',
        ServiceDiagnostics: { LocalizedText: 0 },
        StringTable: [statusName, text],
        ServiceResult: code,
      },
    },
  }
}

// tokens are signed by this server with HS256 only (jwt.sign default)
const VERIFY_OPTS = { algorithms: ['HS256'] }

function getToken(req) {
  return req.headers['x-access-token'] || req.cookies?.['x-access-token']
}

// check and decode token (signature and expiry only, see getCurrentRights)
const checkToken = (req) => {
  const token = getToken(req)
  if (!token || typeof token !== 'string') return false
  try {
    return jwt.verify(token, config.secret, VERIFY_OPTS)
  } catch (err) {
    return false
  }
}

// Rights the signed-in user has now (from the database, cached a few seconds), or
// null when the user was deleted or the token was revoked. Takes the request or a
// decoded token. Throws UserRights.RightsUnavailableError on database trouble.
const getCurrentRights = (reqOrToken, opts) => {
  const tok =
    reqOrToken && typeof reqOrToken.headers === 'object' ?
      checkToken(reqOrToken)
    : reqOrToken
  return UserRights.getCurrentRights(tok || null, opts)
}

const isUnavailable = (err) => err?.name === 'RightsUnavailableError'

const verifyToken = (req, res, next) => {
  Log.log('Verify ' + req.originalUrl, Log.levelDetailed)

  let reqHandle = null
  if (req.method === 'POST')
    reqHandle = req?.body?.Body?.RequestHeader?.RequestHandle

  const deny = (statusName, text, code, message) => {
    if (reqHandle) {
      return res
        .status(200)
        .send(opcDenied(reqHandle, statusName, text, code))
    }
    return res.status(200).send({ ok: false, message: message })
  }

  const token = getToken(req)
  if (!token || typeof token !== 'string')
    return deny(
      'BadIdentityTokenInvalid',
      'Access denied (absent access token)!',
      OPC_BAD_IDENTITY_TOKEN_INVALID,
      'Access not allowed. No token provided'
    )

  jwt.verify(token, config.secret, VERIFY_OPTS, (err, decoded) => {
    if (err)
      return deny(
        'BadIdentityTokenRejected',
        'Access denied (access token rejected)!',
        OPC_BAD_IDENTITY_TOKEN_REJECTED,
        'Access not allowed. ' + err
      )

    // the user must still exist and the token still be valid; the rights are
    // the current ones
    UserRights.getCurrentRights(decoded)
      .then((rights) => {
        if (!rights)
          return deny(
            'BadIdentityTokenRejected',
            'Access denied (session ended or user not found)!',
            OPC_BAD_IDENTITY_TOKEN_REJECTED,
            'Access not allowed. Session ended or user not found'
          )
        req.userId = decoded.id
        req.token = decoded
        req.rights = rights
        next()
      })
      .catch((e) => {
        Log.log('verifyToken: ' + e)
        // not a rejection: the session stays, the client may retry
        if (reqHandle)
          return res
            .status(200)
            .send(
              opcDenied(
                reqHandle,
                'BadServerNotConnected',
                'Database disconnected!',
                OPC_BAD_SERVER_NOT_CONNECTED
              )
            )
        res.status(503).send({
          ok: false,
          unavailable: true,
          message: 'Authentication service unavailable, try again later',
        })
      })
  })
}

// answers a rights lookup failure
function sendLookupError(res, err) {
  if (isUnavailable(err))
    return res.status(503).send({
      unavailable: true,
      message: 'Authentication service unavailable, try again later',
    })
  Log.log(err)
  res.status(500).send({ message: 'Internal server error' })
}

const isAdmin = async (req, res, next) => {
  Log.log('isAdmin', Log.levelDetailed)

  const tok = checkToken(req)
  if (tok === false) {
    res.status(403).send({ message: 'Require isAdmin rights!' })
    return
  }

  try {
    const rights = await UserRights.getCurrentRights(tok, { fresh: true })
    if (rights && rights.isAdmin) {
      req.userId = tok.id
      req.token = tok
      req.rights = rights
      next()
      return
    }
    res.status(403).send({ message: 'Require isAdmin rights!' })
  } catch (err) {
    sendLookupError(res, err)
  }
}

// any signed-in, existing user (no specific right required)
const isUser = async (req, res, next) => {
  const tok = checkToken(req)
  if (tok === false) {
    res.status(403).send({ message: 'Require sign-in!' })
    return
  }
  try {
    const rights = await UserRights.getCurrentRights(tok)
    if (rights) {
      req.userId = tok.id
      req.token = tok
      req.rights = rights
      next()
      return
    }
    res.status(403).send({ message: 'Require sign-in!' })
  } catch (err) {
    sendLookupError(res, err)
  }
}

// Pages (not API calls) for signed-in users: without a valid session the
// browser is sent to the sign-in page instead of getting a JSON answer.
const isUserPage = async (req, res, next) => {
  const tok = checkToken(req)
  try {
    const rights = tok === false ? null : await UserRights.getCurrentRights(tok)
    if (rights) {
      req.userId = tok.id
      req.token = tok
      req.rights = rights
      return next()
    }
  } catch (err) {
    return sendLookupError(res, err)
  }
  if (req.method === 'GET' || req.method === 'HEAD') return res.redirect('/login')
  res.status(401).send({ ok: false, message: 'Require sign-in!' })
}

// security events viewer: a role must grant viewSecEvents (admin is not
// enough on its own)
const isSecEventsViewer = async (req, res, next) => {
  const tok = checkToken(req)
  if (tok === false) {
    res.status(403).send({ error: 'Require sign-in!' })
    return
  }
  try {
    const rights = await UserRights.getCurrentRights(tok, { fresh: true })
    if (rights && rights.viewSecEvents) {
      req.userId = tok.id
      req.token = tok
      req.rights = rights
      next()
      return
    }
    res.status(403).send({ error: 'Require the viewSecEvents right!' })
  } catch (err) {
    sendLookupError(res, err)
  }
}

// CIM-EMS engineering right (model upload/activation, binding approval, artifact
// download): an admin role is not enough, a role must grant cimEngineering.
const hasCimEngineering = async (req) => {
  const tok = checkToken(req)
  if (tok === false) return false
  try {
    return !!(await UserRights.getCurrentRights(tok))?.cimEngineering
  } catch (err) {
    Log.log(err)
    return false
  }
}

const isCimEngineer = async (req, res, next) => {
  if (await hasCimEngineering(req)) {
    next()
    return
  }
  res.status(403).send({ error: 'Require CIM-EMS engineering rights!' })
}

const authJwt = {
  verifyToken,
  checkToken,
  getCurrentRights,
  isAdmin,
  isUser,
  isUserPage,
  hasCimEngineering,
  isCimEngineer,
  isSecEventsViewer,
}
module.exports = authJwt
