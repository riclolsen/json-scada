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

// JSON-SCADA -> Node-RED (data distribution). Watches realtimeData for processed
// value updates (the final value written by cs_data_processor, not raw
// sourceDataUpdate) and fans them out to subscribed WS clients. Also builds
// snapshots for subscribe/read (integrity, cause of transmission 20).

import {
  type ChangeStream,
  type Collection,
  type Document,
} from 'mongodb'
import Log from './log.js'
import AppDefs from './app-defs.js'
import type { Client, WsServer } from './ws-server.js'
import type { PublishTag } from './types.js'

const PROJECTION = {
  _id: 1,
  tag: 1,
  value: 1,
  valueString: 1,
  valueJson: 1,
  invalid: 1,
  alarmed: 1,
  timeTag: 1,
  timeTagAtSource: 1,
  timeTagAtSourceOk: 1,
  group1: 1,
  group2: 1,
  group3: 1,
  type: 1,
  description: 1,
  unit: 1,
  origin: 1,
} as const

export class Publisher {
  private topics: string[]
  private wsServer: WsServer
  private collection: Collection | null = null
  private changeStream: ChangeStream | null = null
  private pending: PublishTag[] = []
  private flushTimer: NodeJS.Timeout | null = null
  stats = { published: 0, snapshots: 0 }

  constructor(topics: string[], wsServer: WsServer) {
    this.topics = topics
    this.wsServer = wsServer
  }

  start(collection: Collection): void {
    this.collection = collection
    this.startChangeStream()
    if (this.flushTimer) clearInterval(this.flushTimer)
    this.flushTimer = setInterval(
      () => this.flush(),
      AppDefs.PUBLISH_FLUSH_MS
    )
  }

  stop(): void {
    if (this.flushTimer) clearInterval(this.flushTimer)
    this.flushTimer = null
    if (this.changeStream) {
      void this.changeStream.close().catch(() => undefined)
      this.changeStream = null
    }
  }

  private csFilterGroup1(): Document {
    if (this.topics.length === 0) return {}
    return { 'fullDocument.group1': { $in: this.topics } }
  }

  private startChangeStream(): void {
    if (!this.collection) return
    // Publish processed values: updates where sourceDataUpdate is NOT among the
    // changed fields (i.e. cs_data_processor's merged value), plus full replaces.
    // The first $match does not reference fullDocument, so the server applies
    // it before the updateLookup and skips the document read for raw driver
    // writes; only the topic filter needs the looked-up document.
    const pipeline: Document[] = [
      {
        $match: {
          $or: [
            {
              $and: [
                {
                  'updateDescription.updatedFields.sourceDataUpdate': {
                    $exists: false,
                  },
                },
                { 'documentKey._id': { $nin: [-1, -2] } },
                { operationType: 'update' },
              ],
            },
            { operationType: 'replace' },
          ],
        },
      },
    ]
    if (this.topics.length > 0) {
      pipeline.push({
        $match: { $or: [this.csFilterGroup1(), { operationType: 'replace' }] },
      })
    }
    pipeline.push({ $project: { documentKey: false } })

    const cs = this.collection.watch(pipeline, { fullDocument: 'updateLookup' })
    this.changeStream = cs
    cs.on('change', (change: Document) => {
      const doc = (change as { fullDocument?: Document }).fullDocument
      if (!doc) return
      if (doc.origin === 'command') return // don't echo command tags
      this.pending.push(toPublishTag(doc, 3))
      if (this.pending.length >= AppDefs.PUBLISH_FLUSH_MAX) this.flush()
    })
    cs.on('error', (err) => {
      Log.log('Publish - ChangeStream error: ' + err)
      this.restartLater()
    })
    cs.on('close', () => Log.log('Publish - ChangeStream closed', Log.levelDetailed))
    Log.log('Publish - ChangeStream started', Log.levelDetailed)
  }

  private restartLater(): void {
    if (this.changeStream) {
      void this.changeStream.close().catch(() => undefined)
      this.changeStream = null
    }
    setTimeout(() => {
      if (this.collection) this.startChangeStream()
    }, 5000)
  }

  private flush(): void {
    if (this.pending.length === 0) return
    const batch = this.pending
    this.pending = []
    this.wsServer.broadcast(batch)
    this.stats.published += batch.length
  }

  // Sends the initial integrity snapshot to a client per its subscription.
  async sendSnapshot(
    client: Client,
    tags?: string[],
    topics?: string[]
  ): Promise<void> {
    if (!this.collection) return
    const filter = this.buildSnapshotFilter(client, tags, topics)
    if (!filter) return
    try {
      const docs = await this.collection
        .find(filter)
        .project(PROJECTION)
        .toArray()
      const out = docs
        .filter((d) => d.origin !== 'command')
        .map((d) => toPublishTag(d, 20))
      client.enqueue({ type: 'snapshot', tags: out })
      this.stats.snapshots++
      Log.log(
        'Publish - Snapshot of ' + out.length + ' tags to client #' + client.id,
        Log.levelDetailed
      )
    } catch (e) {
      Log.log('Publish - Snapshot error: ' + e)
    }
  }

  // Resolves the Mongo filter for a snapshot, honoring both the connection-level
  // topics and the client's own subscription / explicit read args.
  private buildSnapshotFilter(
    client: Client,
    tags?: string[],
    topics?: string[]
  ): Document | null {
    const conds: Document[] = []

    if (tags && tags.length > 0) {
      conds.push({ tag: { $in: tags } })
    } else if (topics && topics.length > 0) {
      conds.push({ group1: { $in: topics } })
    } else if (client.sub.tags.size > 0) {
      conds.push({ tag: { $in: [...client.sub.tags] } })
    } else if (client.sub.topics.size > 0) {
      conds.push({ group1: { $in: [...client.sub.topics] } })
    } else if (!client.sub.all) {
      return null // nothing selected
    }

    // connection-level group1 restriction always applies
    if (this.topics.length > 0) conds.push({ group1: { $in: this.topics } })

    if (conds.length === 0) return {}
    if (conds.length === 1) return conds[0]!
    return { $and: conds }
  }
}

// Maps a realtimeData document to the wire PublishTag shape.
function toPublishTag(doc: Document, cot: number): PublishTag {
  return {
    tag: String(doc.tag ?? ''),
    pointKey: numOf(doc._id),
    value: numOf(doc.value),
    valueString: doc.valueString != null ? String(doc.valueString) : '',
    valueJson: doc.valueJson ?? null,
    invalid: doc.invalid === true,
    alarmed: doc.alarmed === true,
    timeTag: isoOrNull(doc.timeTag),
    timeTagAtSource: isoOrNull(doc.timeTagAtSource),
    timeTagAtSourceOk: doc.timeTagAtSourceOk === true,
    group1: String(doc.group1 ?? ''),
    group2: String(doc.group2 ?? ''),
    group3: String(doc.group3 ?? ''),
    type: String(doc.type ?? ''),
    description: String(doc.description ?? ''),
    unit: String(doc.unit ?? ''),
    cot,
  }
}

function numOf(v: unknown): number {
  if (typeof v === 'number') return v
  if (v != null && typeof (v as { valueOf?: () => number }).valueOf === 'function') {
    const n = Number((v as { valueOf: () => unknown }).valueOf())
    if (Number.isFinite(n)) return n
  }
  const n = Number(v)
  return Number.isFinite(n) ? n : 0
}

function isoOrNull(v: unknown): string | null {
  if (v instanceof Date) return v.toISOString()
  if (typeof v === 'string') return v
  return null
}

export default Publisher
