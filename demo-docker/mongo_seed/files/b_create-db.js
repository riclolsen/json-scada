db.createCollection( "commandsQueue" )
db.createCollection( "protocolDriverInstances" )
db.createCollection( "protocolConnections" )
db.protocolConnections.createIndex( { protocolConnectionNumber: 1 }, {name: "protocolConnectionNumberIndex", unique: true} )
db.createCollection( "realtimeData" )
db.realtimeData.createIndex( { tag: 1 }, {name: "tagIndex", unique: true} )
db.realtimeData.createIndex( { protocolSourceConnectionNumber: 1, protocolSourceCommonAddress: 1, protocolSourceObjectAddress:1 } );
db.createCollection( "soeData", { capped: true, size: 500000 } )
db.soeData.createIndex( { timeTag: 1} );
db.soeData.createIndex( { timeTagAtSource: 1} );
db.soeData.createIndex( { group1: 1} );
db.soeData.createIndex( { ack: 1} );
