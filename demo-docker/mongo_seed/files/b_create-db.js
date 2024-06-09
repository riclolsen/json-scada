db.createCollection( "commandsQueue" )
db.createCollection( "protocolDriverInstances" )
db.createCollection( "protocolConnections" )
db.createCollection( "processInstances" )
db.protocolDriverInstances.createIndex( { protocolDriver: 1, protocolDriverInstanceNumber: 1 }, {name: "protocolDriverInstancesIndex", unique: true} )
db.processInstances.createIndex( { processName: 1, processInstanceNumber: 1 }, {name: "processInstancesIndex", unique: true} )
db.protocolConnections.createIndex( { protocolConnectionNumber: 1 }, {name: "protocolConnectionNumberIndex", unique: true} )
db.createCollection( "realtimeData" )
db.realtimeData.createIndex( { tag: 1 }, {name: "tagIndex", unique: true} )
db.realtimeData.createIndex( { protocolSourceConnectionNumber: 1, protocolSourceCommonAddress: 1, protocolSourceObjectAddress:1 } );
db.realtimeData.createIndex( { invalid: 1, alarmed: 1 } )
db.createCollection( "soeData", { capped: true, size: 2000000000 } )
db.soeData.createIndex( { timeTag: 1} );
db.soeData.createIndex( { timeTagAtSource: 1} );
db.soeData.createIndex( { group1: 1} );
db.soeData.createIndex( { ack: 1} );
db.createCollection('roles')
db.roles.createIndex({ name: 1 }, { name: 'roleNameIndex', unique: true })
db.createCollection('users')
db.users.createIndex({ username: 1 }, { name: 'userNameIndex', unique: true })
db.createCollection('hist', {
  timeseries: {
    timeField: 'timeTag',
    metaField: 'tag',
    bucketMaxSpanSeconds: 3600,
    bucketRoundingSeconds: 3600,
  },
  expireAfterSeconds: 60 * 60 * 24 * 30 * 2,
})
