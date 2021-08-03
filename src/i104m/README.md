# I104M Client

This protocol is a custom protocol used by drivers in the OSHMI project.
It makes it possible to use OSHMI drivers in JSON-SCADA.
ICCP driver that is part of the JSON-SCADA source code uses the I104M protocol.

Messages are passed via UDP using a custom format based on IEC60870-5-104 ASDUs.

The implementation is developed using the Go language.

Basically, it is a process that listen for UDP messages and write incoming data to MongoDB. Also a MongoDB change stream is used to monitor for commands (commandsQueue collection) and forward to the UDP destination.

## Configuration

A driver instance must be created in "protocolDriverInstances" collection:

    db.protocolDriverInstances.insert({ 
        "protocolDriver": "I104M",                    // driver name must be "I104M"
        "protocolDriverInstanceNumber": 1,            // instance number, use 1 or more if needed
        "enabled": true,                              // enable the instance
        "logLevel": 1,                                // adjust log level 0-N
        "nodeNames": ["mainNode", "secondaryNode"],   // list node names that will run the instance
        "keepProtocolRunningWhileInactive": false,    // always use false here
        "activeNodeKeepAliveTimeTag": datetime.now(), // this will be updated by the active drive instance
        "activeNodeName": ""                          // this will be updated by the active drive instance
        })

Multiple nodes can run this protocol driver. List "nodeNames" that will run the driver instance. Only one of node can be active at a time for a instance, so only the active will write data to mongodb and send commands to UDP clients.

A driver instance can have just one connection. If needed multiple connections, it is necessary to run multiple instances of the driver (each must listen on a distinct UDP port when run in the same server).

One connection must be created for each instance in "protocolConnections":

    db.protocolConnections.insert({
        "protocolDriver": "I104M",              // driver name must be "I104M"
        "protocolDriverInstanceNumber": 1,      // instance number, use 1 or more if needed
        "protocolConnectionNumber": 61,         // number of the connection (unique number for collection)
        "name": "I104M-1",                      // name for the connection (documental)
        "description": "I104M Connection",      // description (documental)
        "enabled": true,                        // enable the connection
        "commandsEnabled": true,                // enable commands for the connection (if false, no commands will be forwarded)
        "ipAddressLocalBind": "0.0.0.0:8099",   // bind address and port to listen for UPD messages
        "ipAddresses": ["127.0.0.1:8098"]       // only accept messages from addresses here, deliver commands only to the first two to IP:port of this list
        })


Must reload the driver when changed configuration in protocolDriverInstances or protocolConnections.

To update tags with this data source, set "protocolSourceConnectionNumber" and "protocolSourceObjectAddress" for the tag.
The "protocolSourceObjectAddress" must be unique in the same connection.

    db.realtimeData.update({
        "tag": "SOME-TAG"                              // tag to be updated 
        },{
        "$set": {                                     
            "protocolSourceConnectionNumber": 61,      // connection number that will update this tag
            "protocolSourceObjectAddress": 1001        // object address on protocol
        }
    })



