/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 */

// Node-RED integration driver entry point. Connects to MongoDB, resolves the single
// connection owned by this instance, runs redundancy, and — only while this node is
// active — hosts the WebSocket server and the four data paths (ingest, publish,
// commands in/out).

import { MongoClient, type Collection, type Db } from 'mongodb'
import Log from './log.js'
import AppDefs from './app-defs.js'
import loadConfig from './load-config.js'
import Redundancy, { type MongoStatus } from './redundancy.js'
import TagManager from './tags.js'
import Ingest from './ingest.js'
import Publisher from './publish.js'
import CommandsIn from './commands-in.js'
import CommandsOut from './commands-out.js'
import { WsServer, type Client, type Handlers } from './ws-server.js'
import type { ConnectionDoc, DriverConfig } from './types.js'

process.on('uncaughtException', (err) =>
  Log.log('Uncaught Exception: ' + (err?.stack || err))
)

const config = loadConfig()
const mongoStatus: MongoStatus = { HintMongoIsConnected: false }

// Everything that only exists while this node is the active one.
interface ActiveSet {
  wsServer: WsServer
  ingest: Ingest
  publisher: Publisher
  commandsIn: CommandsIn
  commandsOut: CommandsOut
}

async function main(): Promise<void> {
  Log.log(AppDefs.MSG + ' Version ' + AppDefs.VERSION)

  let clientMongo: MongoClient | null = null
  let db: Db | null = null
  let connection: ConnectionDoc | null = null
  let active: ActiveSet | null = null
  let lastStatsAt = 0
  let redundancyStarted = false

  // eslint-disable-next-line no-constant-condition
  while (true) {
    if (clientMongo === null) {
      try {
        Log.log('MongoDB - Connecting...')
        clientMongo = await MongoClient.connect(
          config.mongoConnectionString,
          config.MongoConnectionOptions
        )
        db = clientMongo.db(config.mongoDatabaseName)
        mongoStatus.HintMongoIsConnected = true
        Log.log('MongoDB - Connected')

        connection = await resolveConnection(db, config)
        config.ConnectionNumber = connection.protocolConnectionNumber
        Log.log(
          'Config - Connection ' +
            connection.name +
            ' (#' +
            connection.protocolConnectionNumber +
            ')'
        )

        if (!redundancyStarted) {
          Redundancy.Start(
            AppDefs.REDUNDANCY_INTERVAL_MS,
            clientMongo,
            db,
            config,
            mongoStatus
          )
          redundancyStarted = true
        }
      } catch (e) {
        Log.log('MongoDB - Connection error: ' + e)
        mongoStatus.HintMongoIsConnected = false
        if (clientMongo) {
          try {
            await clientMongo.close()
          } catch {
            /* ignore */
          }
        }
        clientMongo = null
        db = null
        await sleep(5000)
        continue
      }
    }

    // health check
    try {
      await db!.command({ ping: 1 })
      mongoStatus.HintMongoIsConnected = true
    } catch {
      Log.log('MongoDB - Ping failed, will reconnect')
      mongoStatus.HintMongoIsConnected = false
      active = await teardown(active)
      try {
        await clientMongo!.close()
      } catch {
        /* ignore */
      }
      clientMongo = null
      db = null
      await sleep(5000)
      continue
    }

    // Active/standby gating: bind the WS server + start data paths only when active.
    const isActive = Redundancy.ProcessStateIsActive()
    if (isActive && active === null && db && connection) {
      active = startActive(db, config, connection)
    } else if (!isActive && active !== null) {
      Log.log('Redundancy - Node inactive, tearing down WS server')
      active = await teardown(active)
    }

    // connection stats every 5 s on the active node: stats.isConnected is true
    // while at least one Node-RED client is authenticated (communication
    // status tags read it)
    if (active !== null && db && connection && Date.now() - lastStatsAt >= 5000) {
      lastStatsAt = Date.now()
      let authed = false
      for (const c of active.wsServer.getClients()) if (c.authed) authed = true
      try {
        await db
          .collection(config.ProtocolConnectionsCollectionName)
          .updateOne(
            { protocolConnectionNumber: connection.protocolConnectionNumber },
            {
              $set: {
                'stats.nodeName': config.nodeName,
                'stats.timeTag': new Date(),
                'stats.isConnected': authed,
              },
            }
          )
      } catch (e) {
        Log.log('MongoDB - Error updating connection stats: ' + e)
      }
    }

    await sleep(1000)
  }
}

// Resolves (and validates) the single connection this instance owns. Exit codes match
// telegraf-listener so NSSM/supervisord restart behavior is consistent.
async function resolveConnection(
  db: Db,
  cfg: DriverConfig
): Promise<ConnectionDoc> {
  const results = await db
    .collection(cfg.ProtocolConnectionsCollectionName)
    .find({
      protocolDriver: AppDefs.NAME,
      protocolDriverInstanceNumber: cfg.Instance,
    })
    .toArray()

  if (results.length === 0) {
    Log.log('Config - No protocol connection found!')
    process.exit(1)
  }
  const conn = results[0] as unknown as ConnectionDoc
  if (!('protocolConnectionNumber' in conn)) {
    Log.log('Config - No protocolConnectionNumber on record!')
    process.exit(2)
  }
  if (conn.enabled === false) {
    Log.log('Config - Connection disabled, exiting!')
    process.exit(3)
  }
  return conn
}

// Instantiates and starts the active-node data paths, wiring the WS handlers.
function startActive(
  db: Db,
  cfg: DriverConfig,
  conn: ConnectionDoc
): ActiveSet {
  Log.log('Redundancy - Node active, starting WS server and data paths')
  const rtCollection: Collection = db.collection(cfg.RealtimeDataCollectionName)
  const cmdQueue: Collection = db.collection(cfg.CommandsQueueCollectionName)

  const connNumber = conn.protocolConnectionNumber
  const autoCreate = conn.autoCreateTags !== false
  const topics = Array.isArray(conn.topics) ? conn.topics : []

  const tags = new TagManager(connNumber, cfg.GroupSep)
  const ingest = new Ingest(connNumber, autoCreate, tags)
  const commandsOut = new CommandsOut(conn)
  const publisherRef: { p: Publisher | null } = { p: null }
  const commandsInRef: { c: CommandsIn | null } = { c: null }

  const handlers: Handlers = {
    onUpdates: (points, _client) => {
      // Declared commandable points get a command tag too (jsonscada tag out).
      for (const p of points) {
        if (p.commandable && autoCreate && !tags.isCommandKnown(p.address)) {
          void tags.ensureCommandTag(rtCollection, p)
        }
      }
      ingest.enqueue(points)
    },
    onSubscribe: (client: Client) => {
      if (client.sub.snapshot && publisherRef.p)
        void publisherRef.p.sendSnapshot(client)
    },
    onRead: (client, reqTags, reqTopics) => {
      if (publisherRef.p) void publisherRef.p.sendSnapshot(client, reqTags, reqTopics)
    },
    onCommand: (client, ref) => {
      void commandsOut.handle(client, ref)
    },
    onCommandResult: (_client, pointKey, tag, ok) => {
      if (commandsInRef.c) void commandsInRef.c.writeResult(pointKey, tag, ok)
    },
  }

  const wsServer = new WsServer(conn, handlers)
  const publisher = new Publisher(topics, wsServer)
  const commandsIn = new CommandsIn(conn, wsServer)
  publisherRef.p = publisher
  commandsInRef.c = commandsIn

  ingest.start(rtCollection)
  publisher.start(rtCollection)
  commandsIn.start(cmdQueue)
  commandsOut.start(rtCollection, cmdQueue)
  wsServer.start()

  return { wsServer, ingest, publisher, commandsIn, commandsOut }
}

async function teardown(active: ActiveSet | null): Promise<null> {
  if (!active) return null
  active.ingest.stop()
  active.publisher.stop()
  active.commandsIn.stop()
  await active.wsServer.stop()
  return null
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

void main()
