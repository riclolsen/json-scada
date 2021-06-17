const express = require('express')
const MongoClient = require('mongodb').MongoClient
const { parse } = require('json2csv')
const Log = require('./simple-logger')
const AppDefs = require('./app-defs')
const LoadConfig = require('./load-config')
const app = express()

app.use(express.json({limit: '200mb'}))
app.use(express.text({limit: '200mb'}))

app.listen(AppDefs.HTTP_PORT, AppDefs.IP_BIND, () => {
    console.log('listening on ' + AppDefs.HTTP_PORT)
  })

const jsConfig = LoadConfig() // load and parse config file
Log.logLevelCurrent = jsConfig.LogLevel
MongoClient.connect(
  // try to (re)connect
  jsConfig.mongoConnectionString,
  getMongoConnectionOptions(jsConfig)
).then(async client => {
  Log.log('MongoDB - Connected correctly to MongoDB server', Log.levelMin)
  const db = client.db(jsConfig.mongoDatabaseName)
  connsCollection = db.collection(jsConfig.ProtocolConnectionsCollectionName)
  rtDataCollection = db.collection(jsConfig.RealtimeDataCollectionName)
  instCollection = db.collection(jsConfig.ProtocolDriverInstancesCollectionName)

  let instFields = [
    '_id',
    'protocolDriver',
    'protocolDriverInstanceNumber',
    'enabled',
    'logLevel',
    'nodeNames',
    'keepProtocolRunningWhileInactive',
  ]
  let instProjection = {}
  instFields.forEach( value => {
    instProjection[value] = 1
  })

  app.get('/excel/protocolDriverInstances.csv', async function (req, res) {
    let results = await instCollection
    .find({}).project(instProjection).sort({_id:1})
    .toArray()
    res.setHeader('content-type', 'text/csv');
    res.setHeader('header', 'present');
    res.send(parse(results,{fields: instFields}))
  })

  app.get('/excel/protocolDriverInstances.json', async function (req, res) {
    let results = await instCollection
    .find({}).project(instProjection).sort({_id:1})
    .toArray()
    res.send(results)
  })

  let connsFields = [
    '_id',
    'protocolDriver',
    'protocolDriverInstanceNumber',
    'protocolConnectionNumber',
    'name',
    'description',
    'enabled',
    'commandsEnabled',
  ]
  let connsProjection = {}
  connsFields.forEach( value => {
    connsProjection[value] = 1
  })

  app.get('/excel/connections.csv', async function (req, res) {
    let results = await connsCollection
    .find({}).project(connsProjection).sort({_id:1})
    .toArray()
    res.setHeader('content-type', 'text/csv');
    res.setHeader('header', 'present');
    res.send(parse(results,{fields: connsFields}))
  })

  app.get('/excel/connections.json', async function (req, res) {
    let results = await connsCollection
    .find({}).project(connsProjection).sort({_id:1})
    .toArray()
    res.send(results)
  })

  let fields = [
      '_id',
      'tag',
      'type',
      'origin',
      'description',
      'ungroupedDescription',
      'group1',
      'group2',
      'group3',
      'valueDefault',
      'priority',
      'frozenDetectTimeout',
      'invalidDetectTimeout',
      'historianDeadBand',
      'historianPeriod',
      'commandOfSupervised',
      'supervisedOfCommand',
      'location',
      'isEvent',
      'unit',
      'alarmState',
      'stateTextTrue',
      'stateTextFalse',
      'eventTextTrue',
      'eventTextFalse',
      'formula',
      'parcels',
      'kconv1',
      'kconv2',
      'zeroDeadband',
      'protocolSourceConnectionNumber',
      'protocolSourceCommonAddress',
      'protocolSourceObjectAddress',
      'protocolSourceASDU',
      'protocolSourceCommandDuration',
      'protocolSourceCommandUseSBO',
      'protocolDestinations',
      'hiLimit',
      'hihiLimit',
      'hihihiLimit',
      'loLimit',
      'loloLimit',
      'lololoLimit',
      'hysteresis',
      'alarmDisabled',
      'commandBlocked',
      'annotation',
      'notes',
      'commissioningRemarks',
  ]
  let projection = {}
  fields.forEach( value => {
    projection[value] = 1
  })

  app.get('/excel/realtimeData.csv', async function (req, res) {
    let results = await rtDataCollection
    .find({_id: {$gt:0}}).project(projection).sort({_id:1})
    .toArray()
    res.setHeader('content-type', 'text/csv');
    res.setHeader('header', 'present');
    res.send(parse(results,{fields}))
  })

  app.get('/excel/realtimeData.json', async function (req, res) {
    let results = await rtDataCollection
    .find({_id: {$gt:0}}).project(projection).sort({_id:1})
    .toArray()
    res.send(results)
  })

  app.post('/excel/realtimeDataUpdate', async function (req, res) {
    console.log(req.body)
    res.sendStatus(200)
  })  

})

function getMongoConnectionOptions (configObj) {
  let connOptions = {
    useNewUrlParser: true,
    useUnifiedTopology: true,
    appname:
      AppDefs.NAME +
      ' Version:' +
      AppDefs.VERSION +
      ' Instance:' +
      configObj.Instance,
    poolSize: 20,
    readPreference: MongoClient.READ_PRIMARY
  }

  if (
    typeof configObj.tlsCaPemFile === 'string' &&
    configObj.tlsCaPemFile.trim() !== ''
  ) {
    configObj.tlsClientKeyPassword = configObj.tlsClientKeyPassword || ''
    configObj.tlsAllowInvalidHostnames =
      configObj.tlsAllowInvalidHostnames || false
    configObj.tlsAllowChainErrors = configObj.tlsAllowChainErrors || false
    configObj.tlsInsecure = configObj.tlsInsecure || false

    connOptions.tls = true
    connOptions.tlsCAFile = configObj.tlsCaPemFile
    connOptions.tlsCertificateKeyFile = configObj.tlsClientPemFile
    connOptions.tlsCertificateKeyFilePassword = configObj.tlsClientKeyPassword
    connOptions.tlsAllowInvalidHostnames = configObj.tlsAllowInvalidHostnames
    connOptions.tlsInsecure = configObj.tlsInsecure
  }

  return connOptions
}
