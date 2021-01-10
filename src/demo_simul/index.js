const APP_NAME = 'demo substation simul'
const mongo = require('mongodb')
const MongoClient = require('mongodb').MongoClient
let Server = require('mongodb').Server
const fs = require('fs')
const { networkInterfaces } = require('os')

const jsConfigFile = '../../conf/json-scada.json'
const RealtimeDataCollectionName = 'realtimeData'
const CommandsCollectionName = 'commandsQueue'
const ProtocolDriverInstancesCollectionName = 'protocolDriverInstances'

let connOptions = {
  useNewUrlParser: true,
  useUnifiedTopology: true,
  appname: APP_NAME,
  poolSize: 20,
  readPreference: Server.READ_PRIMARY
}

const pipeline = [
  {
    $project: { documentKey: false }
  },
  {
    $match: {
      $and: [
        {
          $or: [{ operationType: 'insert' }]
        }
      ]
    }
  }
]

let rawFileContents = fs.readFileSync(jsConfigFile)
let jsConfig = JSON.parse(rawFileContents)
if (
  typeof jsConfig.mongoConnectionString != 'string' ||
  jsConfig.mongoConnectionString === ''
) {
  console.log('Error reading config file.')
  process.exit()
}

console.log('Connecting to ' + jsConfig.mongoConnectionString)
if (
  typeof jsConfig.tlsCaPemFile === 'string' &&
  jsConfig.tlsCaPemFile.trim() !== ''
) {
  jsConfig.tlsClientKeyPassword = jsConfig.tlsClientKeyPassword || ''
  jsConfig.tlsAllowInvalidHostnames = jsConfig.tlsAllowInvalidHostnames || false
  jsConfig.tlsAllowChainErrors = jsConfig.tlsAllowChainErrors || false
  jsConfig.tlsInsecure = jsConfig.tlsInsecure || false

  connOptions.tls = true
  connOptions.tlsCAFile = jsConfig.tlsCaPemFile
  connOptions.tlsCertificateKeyFile = jsConfig.tlsClientPemFile
  connOptions.tlsCertificateKeyFilePassword = jsConfig.tlsClientKeyPassword
  connOptions.tlsAllowInvalidHostnames = jsConfig.tlsAllowInvalidHostnames
  connOptions.tlsInsecure = jsConfig.tlsInsecure
}

; (async () => {
  let collection = null

  let cntUpd = 1

  setInterval(async function () {
    if (clientMongo !== null) {


      const db = clientMongo.db(jsConfig.mongoDatabaseName)

      // fake IEC 104 driver running
      await db.collection(ProtocolDriverInstancesCollectionName).updateOne(
        {
          protocolDriver: 'IEC60870-5-104',
        },
        [
          {
            $set: {
              activeNodeKeepAliveTimeTag: '$$NOW'
            }
          }
        ]
      )

      // validates supervised data
      await db.collection(RealtimeDataCollectionName).updateMany(
        {
          origin: 'supervised',
        },
        [
          {
            $set: {
              invalid: false,
              timeTag: '$$NOW',
              'sourceDataUpdate.timeTag': '$$NOW'
            }
          }
        ]
      )


      // simulate protocol writes of digital values

      let res = await db.collection(RealtimeDataCollectionName).updateMany(
        {
          type: 'digital',
          origin: 'supervised',
          $expr: { $eq: [{ $indexOfBytes: ['$tag', 'XSWI'] }, -1] },
          _id: { $mod: [Math.floor(Math.random() * 500) + 500, 0] }
        },
        [
          {
            $set: {
              sourceDataUpdate: {
                valueAtSource: { $toDouble: { $not: '$value' } },
                valueStringAtSource: '',
                asduAtSource: 'M_SP_NA_1',
                causeOfTransmissionAtSource: '3',
                invalid: false,
                timeTag: '$$NOW',
                timeTagAtSource: '$$NOW',
                timeTagAtSourceOk: true,
                substitutedAtSource: false,
                overflowAtSource: false,
                blockedAtSource: false,
                notTopicalAtSource: false,
                CntUpd: cntUpd
              }
            }
          }
        ]
      )
      console.log(
        'Digital matchedCount: ' +
        res.matchedCount +
        ' modifiedCount: ' +
        res.modifiedCount
      )

      await db.collection(RealtimeDataCollectionName).find({
        type: 'digital',
        origin: 'supervised',
        "sourceDataUpdate.CntUpd": cntUpd
      }).toArray(async function (err, resarr) {
        resarr.forEach(async element => {
          element.sourceDataUpdate.CntUpd = 0
          let res2 = await db.collection(RealtimeDataCollectionName).updateOne(
            {
              _id: element._id
            },
            {
              $set: {
                sourceDataUpdate: element.sourceDataUpdate
              }
            }
          )
          console.log(
            'Digital matchedCount: ' +
            res2.matchedCount +
            ' modifiedCount: ' +
            res2.modifiedCount
          )

        });
      })
      cntUpd++
    }
  }, 5777)

  setInterval(async function () {
    if (clientMongo !== null) {
      const db = clientMongo.db(jsConfig.mongoDatabaseName)
      let res = await db.collection(RealtimeDataCollectionName).updateMany(
        {
          type: 'analog',
          origin: 'supervised',
          _id: {
            $mod: [
              1 +
              Math.floor(Math.random() * 50) +
              Math.floor(Math.random() * 10),
              0
            ]
          }
        },
        [
          {
            $set: {
              sourceDataUpdate: {
                valueAtSource: {
                  $multiply: ['$valueDefault', 1 + 0.1 * Math.random() - 0.05]
                },
                valueStringAtSource: '',
                asduAtSource: 'M_ME_NC_1',
                causeOfTransmissionAtSource: '3',
                invalid: false,
                timeTag: '$$NOW',
                timeTagAtSource: '$$NOW',
                timeTagAtSourceOk: true,
                substitutedAtSource: false,
                overflowAtSource: false,
                blockedAtSource: false,
                notTopicalAtSource: false,
                CntUpd: cntUpd
              }
            }
          }
        ]
      )
      console.log(
        'Analog matchedCount: ' +
        res.matchedCount +
        ' modifiedCount: ' +
        res.modifiedCount
      )

      await db.collection(RealtimeDataCollectionName).find({
        type: 'analog',
        origin: 'supervised',
        "sourceDataUpdate.CntUpd": cntUpd
      }).toArray(async function (err, resarr) {
        resarr.forEach(async element => {
          element.sourceDataUpdate.CntUpd = 0
          let res2 = await db.collection(RealtimeDataCollectionName).updateOne(
            {
              _id: element._id
            },
            {
              $set: {
                sourceDataUpdate: element.sourceDataUpdate
              }
            }
          )
          console.log(
            'Analog matchedCount: ' +
            res2.matchedCount +
            ' modifiedCount: ' +
            res2.modifiedCount
          )

        });
      })
      cntUpd++

    }
  }, 2000)

  let clientMongo = null
  while (true) {
    if (clientMongo === null)
      await MongoClient.connect(jsConfig.mongoConnectionString, connOptions)
        .then(async client => {
          clientMongo = client
          console.log('Connected correctly to MongoDB server')

          // specify db and collections
          const db = client.db(jsConfig.mongoDatabaseName)
          collection = db.collection(CommandsCollectionName)

          const changeStream = collection.watch(pipeline, {
            fullDocument: 'updateLookup'
          })

          // start listen to changes
          changeStream.on('change', async change => {
            if (change.operationType === 'delete') return

            if (change.operationType === 'insert') {
              // document inserted
              console.log('INSERT ' + change.fullDocument.tag)

              let data = await db
                .collection(RealtimeDataCollectionName)
                .findOne({ tag: change.fullDocument.tag })
              console.log('Supervised of command: ' + data.supervisedOfCommand)
              console.log('Command value: ' + change.fullDocument.value)
              let val = change.fullDocument.value
              if (change.fullDocument.tag.indexOf("YTAP") !== -1) {
                if (change.fullDocument.value === 0)
                  val = { $add: ['$value', -1] }
                else
                  val = { $add: ['$value', 1] }
              }

              let res = await db
                .collection(RealtimeDataCollectionName)
                .updateOne(
                  { _id: data.supervisedOfCommand },
                  [{
                    $set: {
                      sourceDataUpdate: {
                        valueAtSource: val,
                        valueStringAtSource: '',
                        asduAtSource: 'M_SP_NA_1',
                        causeOfTransmissionAtSource: '3',
                        invalid: false,
                        timeTag: new Date(),
                        timeTagAtSource: new Date(),
                        timeTagAtSourceOk: true,
                        substitutedAtSource: false,
                        overflowAtSource: false,
                        blockedAtSource: false,
                        notTopicalAtSource: false,
                        CntUpd: cntUpd
                      }
                    }
                  }]
                )
              console.log(res.matchedCount)
              console.log(res.modifiedCount)
              if (res.matchedCount > 0) {
                console.log('ACK')
                db.collection(CommandsCollectionName).updateOne(
                  { _id: change.fullDocument._id },
                  { $set: { ack: true, ackTimeTag: new Date() } }
                )
              }
              
                            await db.collection(RealtimeDataCollectionName).find({
                              origin: 'supervised',
                              "sourceDataUpdate.CntUpd": cntUpd
                            }).toArray(async function (err, resarr){
                              resarr.forEach(async element => {
                                element.sourceDataUpdate.CntUpd = 0
                                let res2 = await db.collection(RealtimeDataCollectionName).updateOne(
                                  { _id: element._id 
                                  },
                                  {
                                    $set: {
                                      sourceDataUpdate: element.sourceDataUpdate
                                    }
                                  }
                                )
                                console.log(
                                  'Digital matchedCount: ' +
                                  res2.matchedCount +
                                  ' modifiedCount: ' +
                                  res2.modifiedCount
                                ) 
                                  
                              });
                            })
                            cntUpd++
              
            }
          })
        })
        .catch(function (err) {
          if (clientMongo) clientMongo.close()
          clientMongo = null
          console.log(err)
        })

    // wait 5 seconds
    await new Promise(resolve => setTimeout(resolve, 5000))

    // detect connection problems, if error will null the client to later reconnect
    if (clientMongo === undefined) {
      console.log('Disconnected Mongodb!')
      clientMongo = null
    }
    if (clientMongo)
      if (!clientMongo.isConnected()) {
        // not anymore connected, will retry
        console.log('Disconnected Mongodb!')
        clientMongo.close()
        clientMongo = null
      }
  }
})()
