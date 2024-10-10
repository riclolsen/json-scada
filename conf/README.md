# The conf/json-scada.json Config File

The _json-scada.json_ config file is used to instruct processes of a system on how to connect to the MongoDB server/database.

After a process is connected to the server it will load all the configs it needs from the server.

This file must be located in the installPath/conf/ folder.
As you can have multiple JSON-SCADA systems in the same computer server, each installation must go to a different path.

## File format

    {
    "nodeName"  : "mainNode",
    "mongoConnectionString": "mongodb://user:password@localhost:27017/json_scada?replicaSet=rs1&authSource=json_scada",
    "mongoDatabaseName": "json_scada",
    "tlsCaPemFile": "c:\\json-scada\\conf\\rootCa.pem",
    "tlsClientPemFile": "c:\\json-scada\\conf\\mongodb.pem",
    "tlsClientPfxFile": "c:\\json-scada\\conf\\mongodb.pfx",
    "tlsClientKeyPassword": "passw0rd",
    "tlsAllowInvalidHostnames": true,
    "tlsAllowChainErrors": true,
    "tlsInsecure": false
    }

* **_nodeName_** - Unique name for a computer installation. This name will be used to match the node configuration in the database. This name is used also to control processes on redundant computers. **Mandatory parameter**.
* **_mongoConnectionString_** - Standard MongoDB URI connection string pointing to the database server. Please include the database name in this URI string (the same db name from the next parameter) and TLS options. See https://docs.mongodb.com/manual/reference/connection-string/. **Mandatory parameter**
* **_mongoDatabaseName_** - Database name to be accessed in the MongoDB server. **Mandatory parameter**.

The TLS parameters below are necessary for secure connecions, anyway please include the equivalent TLS options directly in the URI connection string. See https://www.mongodb.com/pt-br/docs/manual/reference/connection-string-options/#std-label-connections-connection-options.

If you encounter secure connection problems, please report your findings.

* **_tlsCaPemFile_** - Path/Name of the certificate root CA PEM file. **Optional parameter, required for TLS connection**.
* **_tlsClientPemFile_** - Path/Name of the client certificate PEM file. **Optional parameter, required for TLS connection**.
* **_tlsClientPfxFile_** - Path/Name of the client certificate PFX file. **Optional parameter, required for TLS connection**.
* **_tlsClientKeyPassword_** - Password for the PFX file. **Optional parameter, required for TLS connection**.
* **_tlsAllowInvalidHostnames_** - Do not check for the server hostname in certificates. **Optional parameter**.
* **_tlsAllowChainErrors_** - Allows for certificate chain errors. **Optional parameter**.
* **_tlsInsecure_** - Relax other security checks. **Optional parameter**.

For a local unsecured connection to a database server just the three first parameters are required. **Do not open unsecured MongoDB servers to the outside network!**

To use MongoDB authorization, specify the database user and password in the connection string. This may be enough for protected networks. To connect to the database server using the MongoDB Compass tool it may be necessary to add "authSource=json_scada_db_name" to the end of the connection string.

To encrypt connections to the database with TLS it is necessary to create and specify certificates in the config file that can be accepted by the MongoDB server. This is important for connections over the Internet and untrusted networks.

MongoDB servers can be Community, Enterprise or Atlas Cloud, all on 6.0 or later version. Other servers are not tested or supported.

The hostname(s) used in the connection string must match the hostnames for the MongoDB replica set members. The hostname must be available via DNS or be present in the server's _/etc/hosts_ file. A replica set is mandatory for the change streams to work and must be created with at least one member. A three member replica set is highly recommended for redundancy and high availability.

## Creating self-signed certificates

https://medium.com/@rajanmaharjan/secure-your-mongodb-connections-ssl-tls-92e2addb3c89
