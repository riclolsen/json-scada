const Double = require('@mongoosejs/double');
const config = require('../config/auth.config')
const db = require('../models')
const fs = require('fs')
var path = require('path')
const User = db.user
const Role = db.role
const Tag = db.tag
const ProtocolDriverInstance = db.protocolDriverInstance
const ProtocolConnection = db.protocolConnection
const UserAction = db.userAction
const UserActionsQueue = require('../../userActionsQueue')

var jwt = require('jsonwebtoken')
var bcrypt = require('bcryptjs')

// add header to identify json-scada user for Grafana auto-login (auth.proxy)
exports.addXWebAuthUser = (req) => {
  let ck = checkToken(req)
  if (ck !== false)
    req.headers['X-WEBAUTH-USER'] = ck?.username
}

exports.listUserActions = async (req, res) => {
  console.log('listUserActions')

  let skip = 0
  if ('page' in req.body && 'itemsPerPage' in req.body)
    skip = req.body.itemsPerPage * (req.body.page - 1)
  let filter = {}
  if ('filter' in req.body) filter = req.body.filter

  let limit = req.body.itemsPerPage || 10
  let orderby = {}
  if ('sortBy' in req.body && 'sortDesc' in req.body) {
    for (let i = 0; i < req.body.sortBy.length; i++)
      orderby[req.body.sortBy[i]] = req.body.sortDesc[i] ? -1 : 1
    if (req.body.sortBy.length === 0) orderby = { timeTag: 1 }
  } else orderby = { timeTag: 1 }

  let count = await UserAction.countDocuments(filter)
  UserAction.find(filter)
    .skip(skip)
    .limit(limit)
    .sort(orderby)
    .exec(function (err, userActions) {
      if (err) {
        console.log(err)
        res.status(200).send({ error: err })
        return
      }
      let ret = { userActions: userActions, countTotal: count }
      res.status(200).send(ret)
    })
}

exports.createTag = async (req, res) => {
  // find biggest tag _id
  let biggestTagId = 0
  resBiggest = await Tag.find({ _id: { $gt: 0 } })
    .select( "_id" )
    .sort({ _id: -1 })
    .limit(1)
  if (resBiggest && resBiggest.length > 0) 
  if ("_id" in resBiggest[0])
    biggestTagId = parseFloat(resBiggest[0]._id)
  console.log(biggestTagId)
 
  req.body._id = parseFloat(biggestTagId + 1)
  req.body.tag = "new_tag_" + req.body._id
  console.log(req.body)
  const tag = new Tag(req.body)
  tag.save(err => {
    if (err) {
      console.log(err)
      res.status(200).send({ error: err })
      return
    }
    req.body = { _id: tag._id }
    registerUserAction(req, 'createTag')
    res.status(200).send(tag)
  })
}

exports.updateTag = async (req, res) => {
  registerUserAction(req, 'updateTag')

  if ('_id' in req.body) {
    let _id = req.body._id
    delete req.body._id

    var IsNumberVal = function (value) {
      if(/^(\-|\+)?([0-9]+(\.[0-9]+)?)$/
        .test(value))
        return true;
    return false;
    }

    if ( IsNumberVal(req.body.protocolSourceCommonAddress) )
      req.body.protocolSourceCommonAddress = parseFloat(req.body.protocolSourceCommonAddress)
    if ( IsNumberVal(req.body.protocolSourceObjectAddress) )
      req.body.protocolSourceObjectAddress = parseFloat(req.body.protocolSourceObjectAddress)
    if ( IsNumberVal(req.body.protocolSourceASDU) )
      req.body.protocolSourceASDU = parseFloat(req.body.protocolSourceASDU)

    await Tag.findOneAndUpdate({ _id: _id }, req.body)
    res.status(200).send({ error: false })
  } else res.status(200).send({ error: 'No _id in update request.' })
}

exports.deleteTag = async (req, res) => {
  registerUserAction(req, 'deleteTag')

  if ('_id' in req.body) {
    await Tag.findOneAndDelete({ _id: req.body._id })
    res.status(200).send({ error: false })
  } else res.status(200).send({ error: 'No _id in delete request.' })
}

exports.listTags = async (req, res) => {
  console.log('listTags')

  let skip = 0
  if ('page' in req.body && 'itemsPerPage' in req.body)
    skip = req.body.itemsPerPage * (req.body.page - 1)
  let filter = {}
  if ('filter' in req.body) filter = req.body.filter

  let limit = req.body.itemsPerPage || 10
  let orderby = {}
  if ('sortBy' in req.body && 'sortDesc' in req.body) {
    for (let i = 0; i < req.body.sortBy.length; i++)
      orderby[req.body.sortBy[i]] = req.body.sortDesc[i] ? -1 : 1
    if (req.body.sortBy.length === 0) orderby = { tag: 1 }
  } else orderby = { tag: 1 }

  let count = await Tag.countDocuments(filter)
  Tag.find(filter)
    .skip(skip)
    .limit(limit)
    .sort(orderby)
    .exec(function (err, tags) {
      if (err) {
        console.log(err)
        res.status(200).send({ error: err })
        return
      }
      let ret = { tags: tags, countTotal: count }
      res.status(200).send(ret)
    })
}

exports.updateProtocolConnection = async (req, res) => {
  registerUserAction(req, 'updateProtocolConnection')

  // make default bind address for some protocols
  if (
    [
      'IEC60870-5-104_SERVER',
      'DNP3_SERVER',
      'I104M',
      'TELEGRAF-LISTENER',
      'OPC-UA_SERVER'
    ].includes(req?.body?.protocolDriver)
  ) {
    if (
      !('ipAddressLocalBind' in req.body) ||
      req.body.ipAddressLocalBind == ''
    ) {
      req.body.ipAddressLocalBind = '0.0.0.0'
      switch (req?.body?.protocolDriver) {
        case 'OPC-UA_SERVER':
          req.body.ipAddressLocalBind = '0.0.0.0:4840'
          break
        case 'IEC60870-5-104_SERVER':
          req.body.ipAddressLocalBind = '0.0.0.0:2404'
          break
        case 'DNP3_SERVER':
          req.body.ipAddressLocalBind = '0.0.0.0:20000'
          break
        case 'I104M':
          req.body.ipAddressLocalBind = '0.0.0.0:8099'
          break
        case 'TELEGRAF-LISTENER':
          req.body.ipAddressLocalBind = '0.0.0.0:51920'
          break
      }
    }
  }

  if (
    [
      'IEC60870-5-104',
      'IEC60870-5-104_SERVER',
      'DNP3',
      'DNP3_SERVER',
      'PLCTAG',
      'I104M',
      'TELEGRAF-LISTENER',
      'OPC-UA_SERVER'
    ].includes(req?.body?.protocolDriver)
  ) {
    if (!('ipAddresses' in req.body)) {
      req.body.ipAddresses = []
    }
  }

  if (['OPC-UA', 'TELEGRAF-LISTENER', 'MQTT-SPARKPLUG-B'].includes(req?.body?.protocolDriver)) {
    if (!('autoCreateTags' in req.body)) {
      req.body.autoCreateTags = true
    }
  }

  if (['MQTT-SPARKPLUG-B','OPC-UA_SERVER'].includes(req?.body?.protocolDriver)) {
    if (!('topics' in req.body)) {
      req.body.topics = []
    }
    if (!('groupId' in req.body)) {
      req.body.groupId = ''
    }
  }

  if (['MQTT-SPARKPLUG-B', 'IEC60870-5-104_SERVER'].includes(req?.body?.protocolDriver)) {
    if (!('passphrase' in req.body)) {
      req.body.passphrase = ''
    }
  }

  if (['MQTT-SPARKPLUG-B'].includes(req?.body?.protocolDriver)) {
    if (!('topicsAsFiles' in req.body)) {
      req.body.topicsAsFiles = []
    }
    if (!('topicsScripted' in req.body)) {
      req.body.topicsScripted = []
    }
    if (!('clientId' in req.body)) {
      req.body.clientId = ''
    }
    if (!('edgeNodeId' in req.body)) {
      req.body.edgeNodeId = ''
    }
    if (!('deviceId' in req.body)) {
      req.body.deviceId = ''
    }
    if (!('scadaHostId' in req.body)) {
      req.body.scadaHostId = ''
    }
    if (!('publishTopicRoot' in req.body)) {
      req.body.publishTopicRoot = ''
    }
    if (!('username' in req.body)) {
      req.body.username = ''
    }
    if (!('password' in req.body)) {
      req.body.password = ''
    }
    if (!('pfxFilePath' in req.body)) {
      req.body.pfxFilePath = ''
    }
  }

  if (['OPC-UA', 'MQTT-SPARKPLUG-B','OPC-UA_SERVER'].includes(req?.body?.protocolDriver)) {
    if (!('useSecurity' in req.body)) {
      req.body.useSecurity = false
    }
  }

  if (['OPC-UA', 'MQTT-SPARKPLUG-B'].includes(req?.body?.protocolDriver)) {
    if (!('useSecurity' in req.body)) {
      req.body.useSecurity = false
    }
  }

  if (['OPC-UA','OPC-UA_SERVER'].includes(req?.body?.protocolDriver)) {
    if (!('timeoutMs' in req.body)) {
      req.body.timeoutMs = 20000.0
    }
  }

  if (['OPC-UA'].includes(req?.body?.protocolDriver)) {
    if (!('configFileName' in req.body)) {
      req.body.configFileName = '../conf/Opc.Ua.DefaultClient.Config.xml'
    }
    if (!('autoCreateTagPublishingInterval' in req.body)) {
      req.body.autoCreateTagPublishingInterval = 2.5
    }
    if (!('autoCreateTagSamplingInterval' in req.body)) {
      req.body.autoCreateTagSamplingInterval = 0.0
    }
    if (!('autoCreateTagQueueSize' in req.body)) {
      req.body.autoCreateTagQueueSize = 0.0
    }
  }

  if (
    [
      'IEC60870-5-101',
      'IEC60870-5-101_SERVER',
      'IEC60870-5-104',
      'IEC60870-5-104_SERVER',
      'DNP3',
      'DNP3_SERVER',
      'PLCTAG'
    ].includes(req?.body?.protocolDriver)
  ) {
    if (!('localLinkAddress' in req.body)) {
      req.body.localLinkAddress = 1
    }
    if (!('remoteLinkAddress' in req.body)) {
      req.body.remoteLinkAddress = 1
    }
  }

  if (
    ['IEC60870-5-101', 'IEC60870-5-104'].includes(req?.body?.protocolDriver)
  ) {
    if (!('testCommandInterval' in req.body)) {
      req.body.testCommandInterval = 0
    }
  }

  if (
    [
      'IEC60870-5-101',
      'IEC60870-5-104',
      'IEC60870-5-101_SERVER',
      'IEC60870-5-104_SERVER'
    ].includes(req?.body?.protocolDriver)
  ) {
    if (!('sizeOfCOT' in req.body)) {
      switch (req?.body?.protocolDriver) {
        case 'IEC60870-5-104':
        case 'IEC60870-5-104_SERVER':
          req.body.sizeOfCOT = 2
          break
        default:
          req.body.sizeOfCOT = 1
          break
      }
    }
    if (!('sizeOfCA' in req.body)) {
      req.body.sizeOfCA = 2
    }
    if (!('sizeOfIOA' in req.body)) {
      switch (req?.body?.protocolDriver) {
        case 'IEC60870-5-104':
        case 'IEC60870-5-104_SERVER':
          req.body.sizeOfIOA = 3
          break
        default:
          req.body.sizeOfIOA = 2
          break
      }
    }
  }

  if (
    ['IEC60870-5-101', 'IEC60870-5-104', 'DNP3', 'PLCTAG'].includes(
      req?.body?.protocolDriver
    )
  ) {
    if (!('giInterval' in req.body)) {
      req.body.giInterval = 300
    }
    if (!('timeSyncInterval' in req.body)) {
      req.body.timeSyncInterval = 0
    }
  }

  if (['IEC60870-5-104_SERVER'].includes(req?.body?.protocolDriver)) {
    if (!('serverModeMultiActive' in req.body)) {
      req.body.serverModeMultiActive = true
    }
    if (!('maxClientConnections' in req.body)) {
      req.body.maxClientConnections = 1
    }
  }

  if (['IEC60870-5-104', 'IEC60870-5-104_SERVER', 'DNP3', 'MQTT-SPARKPLUG-B','OPC-UA_SERVER'].includes(req?.body?.protocolDriver)) {
    if (!('localCertFilePath' in req.body)) {
      req.body.localCertFilePath = ''
    }
  }

  if (['IEC60870-5-104', 'IEC60870-5-104_SERVER', 'DNP3'].includes(req?.body?.protocolDriver)) {
    if (!('peerCertFilePath' in req.body)) {
      req.body.peerCertFilePath = ''
    }
  }

  if (['IEC60870-5-104_SERVER'].includes(req?.body?.protocolDriver)) {
    if (!('peerCertFilesPaths' in req.body)) {
      req.body.peerCertFilesPaths = ''
    }
  }

  if (['IEC60870-5-104', 'IEC60870-5-104_SERVER', 'MQTT-SPARKPLUG-B'].includes(req?.body?.protocolDriver)) {
    if (!('rootCertFilePath' in req.body)) {
      req.body.rootCertFilePath = ''
    }
    if (!('chainValidation' in req.body)) {
      req.body.chainValidation = false
    }
  }

  if (['IEC60870-5-104', 'IEC60870-5-104_SERVER'].includes(req?.body?.protocolDriver)) {
    if (!('allowOnlySpecificCertificates' in req.body)) {
      req.body.allowOnlySpecificCertificates = false
    }
  }

  if (['DNP3', 'MQTT-SPARKPLUG-B', 'OPC-UA_SERVER'].includes(req?.body?.protocolDriver)) {
    if (!('privateKeyFilePath' in req.body)) {
      req.body.privateKeyFilePath = ''
    }
  }

  if (['DNP3', 'MQTT-SPARKPLUG-B'].includes(req?.body?.protocolDriver)) {
    if (!('allowTLSv10' in req.body)) {
      req.body.allowTLSv10 = false
    }
    if (!('allowTLSv11' in req.body)) {
      req.body.allowTLSv11 = false
    }
    if (!('allowTLSv12' in req.body)) {
      req.body.allowTLSv12 = true
    }
    if (!('allowTLSv13' in req.body)) {
      req.body.allowTLSv13 = true
    }
    if (!('cipherList' in req.body)) {
      req.body.cipherList = ''
    }
  }

  if (['DNP3'].includes(req?.body?.protocolDriver)) {
    if (!('asyncOpenDelay' in req.body)) {
      req.body.asyncOpenDelay = 0.0
    }
    if (!('timeSyncMode' in req.body)) {
      req.body.timeSyncMode = 0.0
    }
    if (!('class0ScanInterval' in req.body)) {
      req.body.class0ScanInterval = 0.0
    }
    if (!('class1ScanInterval' in req.body)) {
      req.body.class1ScanInterval = 0.0
    }
    if (!('class2ScanInterval' in req.body)) {
      req.body.class2ScanInterval = 0.0
    }
    if (!('class3ScanInterval' in req.body)) {
      req.body.class3ScanInterval = 0.0
    }
    if (!('enableUnsolicited' in req.body)) {
      req.body.enableUnsolicited = true
    }
    if (!('rangeScans' in req.body)) {
      req.body.rangeScans = []
    }
  }

  if (
    ['IEC60870-5-101_SERVER', 'IEC60870-5-104_SERVER'].includes(
      req?.body?.protocolDriver
    )
  ) {
    if (!('maxQueueSize' in req.body)) {
      req.body.maxQueueSize = 5000
    }
  }

  if (
    ['IEC60870-5-101', 'IEC60870-5-101_SERVER'].includes(
      req?.body?.protocolDriver
    )
  ) {
    if (!('portName' in req.body)) {
      req.body.portName = ''
    }
    if (!('baudRate ' in req.body)) {
      req.body.baudRate = 9600.0
    }
    if (!('parity' in req.body)) {
      req.body.parity = 'Even'
    }
    if (!('stopBits' in req.body)) {
      req.body.stopBits = ''
    }
    if (!('handshake' in req.body)) {
      req.body.handshake = 'None'
    }
    if (!('timeoutForACK' in req.body)) {
      req.body.timeoutForACK = 1000.0
    }
    if (!('timeoutRepeat' in req.body)) {
      req.body.timeoutRepeat = 1000.0
    }
    if (!('useSingleCharACK' in req.body)) {
      req.body.useSingleCharACK = true
    }
    if (!('sizeOfLinkAddress' in req.body)) {
      req.body.sizeOfLinkAddress = 1.0
    }
  }

  await ProtocolConnection.findOneAndUpdate({ _id: req.body._id }, req.body)
  res.status(200).send({})
}

exports.deleteProtocolConnection = async (req, res) => {
  registerUserAction(req, 'deleteProtocolConnection')

  await ProtocolConnection.findOneAndDelete({ _id: req.body._id })
  res.status(200).send({ error: false })
}

exports.createProtocolConnection = async (req, res) => {
  // find the biggest connection number and increment for the new connection
  await ProtocolConnection.find({}).exec(function (err, protocolConnections) {
    if (err) {
      console.log(err)
      res.status(200).send({ error: err })
      return
    }
    let connNumber = 0
    protocolConnections.forEach(element => {
      if (element.protocolConnectionNumber > connNumber)
        connNumber = element.protocolConnectionNumber
    })
    const protocolConnection = new ProtocolConnection()
    protocolConnection.protocolConnectionNumber = connNumber + 1
    protocolConnection.DriverInstanceNumber = 1
    protocolConnection.save(err => {
      if (err) {
        console.log(err)
        res.status(200).send({ error: err })
        return
      }
      req.body = { _id: protocolConnection._id }
      registerUserAction(req, 'createProtocolConnection')
      res.status(200).send({ error: false })
    })
  })
}

exports.listProtocolConnections = (req, res) => {
  console.log('listProtocolConnections')

  ProtocolConnection.find({}).exec(function (err, protocolConnections) {
    if (err) {
      console.log(err)
      res.status(200).send({ error: err })
      return
    }
    res.status(200).send(protocolConnections)
  })
}

exports.deleteProtocolDriverInstance = async (req, res) => {
  registerUserAction(req, 'deleteProtocolDriverInstance')

  await ProtocolDriverInstance.findOneAndDelete({ _id: req.body._id })
  res.status(200).send({ error: false })
}

exports.listNodes = (req, res) => {
  console.log('listNodes')

  ProtocolDriverInstance.find({}).exec(function (err, driverInstances) {
    if (err) {
      console.log(err)
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
  const driverInstance = new ProtocolDriverInstance()
  driverInstance.save(err => {
    if (err) {
      console.log(err)
      res.status(200).send({ error: err })
      return
    }
    req.body = { _id: driverInstance._id }
    registerUserAction(req, 'createProtocolDriverInstance')
    res.status(200).send({ error: false })
  })
}

exports.listProtocolDriverInstances = (req, res) => {
  console.log('listProtocolDriverInstances')

  ProtocolDriverInstance.find({}).exec(function (err, driverInstances) {
    if (err) {
      console.log(err)
      res.status(200).send({ error: err })
      return
    }
    res.status(200).send(driverInstances)
  })
}

exports.updateProtocolDriverInstance = async (req, res) => {
  try {
    await ProtocolDriverInstance.findOneAndUpdate({ _id: req.body._id }, req.body)
  }
  catch (e){
    req.body.protocolDriverInstanceNumber = req.body.protocolDriverInstanceNumber + 1
    await ProtocolDriverInstance.findOneAndUpdate({ _id: req.body._id }, req.body)
  }
  registerUserAction(req, 'updateProtocolDriverInstance')
  res.status(200).send({})
}

exports.listUsers = (req, res) => {
  console.log('listUsers')

  User.find({})
    .populate('roles')
    .exec(function (err, users) {
      if (err) {
        console.log(err)
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
      console.log(err)
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
      console.log(err)
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
      console.log(err)
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
  const role = new Role(req.body)
  role.save(err => {
    if (err) {
      console.log(err)
      res.status(200).send({ error: err })
      return
    }
    req.body._id = role._id
    registerUserAction(req, 'createRole')
    res.status(200).send({ error: false })
  })
}

exports.createUser = async (req, res) => {
  if (req.body.password && req.body.password !== '')
    req.body.password = bcrypt.hashSync(req.body.password, 8)
  const user = new User(req.body)
  user.save(err => {
    if (err) {
      console.log(err)
      res.status(200).send({ error: err })
      return
    }
    req.body._id = user._id
    registerUserAction(req, 'createUser')
    res.status(200).send({ error: false })
  })
}

exports.deleteRole = async (req, res) => {
  registerUserAction(req, 'deleteRole')

  await Role.findOneAndDelete({ _id: req.body._id })
  res.status(200).send({ error: false })
}

exports.deleteUser = async (req, res) => {
  registerUserAction(req, 'deleteUser')

  await User.findOneAndDelete({ _id: req.body._id })
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
      console.log(err)
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
            console.log(err)
            res.status(500).send({ message: err })
            return
          }

          user.roles = roles.map(role => role._id)
          user.save(err => {
            if (err) {
              console.log(err)
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
          console.log(err)
          res.status(500).send({ message: err })
          return
        }

        user.roles = [role._id]
        user.save(err => {
          if (err) {
            console.log(err)
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
        console.log(err)
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
      properties: body,
      action: actionName,
      timeTag: new Date()
    })
  } else {
    console.log(actionName + ' - ' + req.body?.username)
    delete body['password']
    // register user action
    UserActionsQueue.enqueue({
      username: req.body?.username,
      properties: body,
      action: actionName,
      timeTag: new Date()
    })
  }
}
