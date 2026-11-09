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

// CIM-EMS Admin UI backend: lists models, binding revisions, runs and study
// requests, takes model uploads, and queues study requests in cimStudyRequests.
// The CIM-EMS driver (src/cim-ems, cim_ems/api.py) executes the requests on its
// active node; this server never imports models or computes anything itself.
// All routes need isAdmin (auth.routes.js); model, binding and artifact actions
// also need a role with cimEngineering. Every write is logged as a user action.

'use strict'

const path = require('path')
const Log = require('../../simple-logger')
const db = require('../models')
const UserActionsQueue = require('../../userActionsQueue')
const { checkToken, hasCimEngineering } = require('../middlewares/authJwt')

const { GridFSBucket, ObjectId } = db.mongoose.mongo

const MODELS = 'cimModels'
const BINDINGS = 'cimMeasurementBindings'
const RUNS = 'cimRuns'
const REQUESTS = 'cimStudyRequests'
const BUCKET = 'cimArtifacts'

const MODEL_ID_RE = /^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/
const ZIP_NAME_RE = /^[a-zA-Z0-9][a-zA-Z0-9 ._()-]{0,250}\.zip$/i
const MAX_BINDINGS = 200000

// Request types the driver executes, and whether they need cimEngineering.
// `import` is created by uploadModel only; `listElements` (element catalog for
// binding review) is used by the MCP server.
const REQUEST_TYPES = {
  activate: true,
  rollback: true,
  proposeBindings: true,
  listElements: true,
  saveBindings: true,
  rerun: false,
}

const BINDING_FIELDS = {
  kind: 'string',
  element_id: 'string',
  tag: 'string',
  quantity: 'string',
  side: 'int',
  point_key: 'number',
  closed_value: 'number',
  sign: 'number',
  sigma: 'number',
  max_age_seconds: 'number',
  approved: 'boolean',
  confidence: 'number',
  proposed_from: 'string',
}

function mdb() {
  return db.mongoose.connection.db
}

function username(req) {
  const ck = checkToken(req)
  return ck ? ck.username : ''
}

function registerUserAction(req, actionName, properties) {
  UserActionsQueue.enqueue({
    username: username(req),
    properties: properties,
    action: actionName,
    timeTag: new Date(),
  })
}

function positiveInt(v) {
  const n = Number(v)
  return Number.isInteger(n) && n > 0 ? n : null
}

function fail(res, status, msg) {
  res.status(status).send({ error: msg })
}

function sanitizeBinding(b) {
  if (!b || typeof b !== 'object') return null
  const out = {}
  for (const [k, t] of Object.entries(BINDING_FIELDS)) {
    if (!(k in b) || b[k] === null || b[k] === '') continue
    const v = b[k]
    if (t === 'string') out[k] = String(v).slice(0, 512)
    else if (t === 'boolean') out[k] = v === true || v === 'true'
    else {
      const n = Number(v)
      if (!Number.isFinite(n)) return null
      out[k] = t === 'int' ? Math.trunc(n) : n
    }
  }
  if (!out.kind || !out.element_id || !out.tag) return null
  return out
}

async function insertRequest(req, type, params) {
  const doc = {
    type: type,
    params: params,
    status: 'pending',
    createdAt: new Date(),
    createdBy: username(req),
  }
  const r = await mdb().collection(REQUESTS).insertOne(doc)
  registerUserAction(req, 'cimEms:' + type, params)
  return r.insertedId
}

exports.listModels = async (req, res) => {
  try {
    const models = await mdb()
      .collection(MODELS)
      .find({})
      .sort({ modelId: 1, revision: -1 })
      .limit(1000)
      .toArray()
    res.status(200).send({
      models: models,
      canEngineer: await hasCimEngineering(req),
    })
  } catch (err) {
    Log.log('cimEms listModels: ' + err)
    fail(res, 200, String(err))
  }
}

exports.listBindings = async (req, res) => {
  try {
    const modelId = String(req.query?.modelId || '')
    if (!MODEL_ID_RE.test(modelId)) return fail(res, 400, 'Invalid modelId!')
    const coll = mdb().collection(BINDINGS)
    const revisions = await coll
      .aggregate([
        { $match: { modelId: modelId } },
        {
          $group: {
            _id: '$bindingRevision',
            count: { $sum: 1 },
            approved: { $sum: { $cond: ['$approved', 1, 0] } },
            createdAt: { $first: '$createdAt' },
            createdBy: { $first: '$createdBy' },
          },
        },
        { $sort: { _id: -1 } },
      ])
      .toArray()
    const rev =
      positiveInt(req.query?.revision) || (revisions.length && revisions[0]._id)
    const bindings =
      rev ?
        await coll
          .find(
            { modelId: modelId, bindingRevision: rev },
            { projection: { _id: 0, modelId: 0, createdAt: 0, createdBy: 0 } }
          )
          .limit(MAX_BINDINGS)
          .toArray()
      : []
    res.status(200).send({
      revisions: revisions.map((r) => ({
        bindingRevision: r._id,
        count: r.count,
        approved: r.approved,
        createdAt: r.createdAt,
        createdBy: r.createdBy,
      })),
      bindingRevision: rev || null,
      bindings: bindings.map((b) => {
        b.element_id = b.cimMeasurementMrid
        delete b.cimMeasurementMrid
        return b
      }),
    })
  } catch (err) {
    Log.log('cimEms listBindings: ' + err)
    fail(res, 200, String(err))
  }
}

exports.listRuns = async (req, res) => {
  try {
    const q = {}
    const conn = positiveInt(req.query?.connectionNumber)
    if (conn) q.connectionNumber = conn
    const limit = Math.min(positiveInt(req.query?.limit) || 100, 1000)
    const runs = await mdb()
      .collection(RUNS)
      .find(q)
      .sort({ createdAt: -1 })
      .limit(limit)
      .toArray()
    res.status(200).send({ runs: runs })
  } catch (err) {
    Log.log('cimEms listRuns: ' + err)
    fail(res, 200, String(err))
  }
}

exports.listRequests = async (req, res) => {
  try {
    const limit = Math.min(positiveInt(req.query?.limit) || 50, 500)
    const requests = await mdb()
      .collection(REQUESTS)
      .find({}, { projection: { 'result.proposal': 0 } })
      .sort({ createdAt: -1 })
      .limit(limit)
      .toArray()
    res.status(200).send({ requests: requests })
  } catch (err) {
    Log.log('cimEms listRequests: ' + err)
    fail(res, 200, String(err))
  }
}

// multipart: packageFile (CGMES zip), modelId
exports.uploadModel = async (req, res) => {
  try {
    const modelId = String(req.body?.modelId || '').trim()
    if (!MODEL_ID_RE.test(modelId)) return fail(res, 400, 'Invalid modelId!')
    const upload = req.files?.packageFile
    if (!upload) return fail(res, 400, 'No model package uploaded!')
    const name = path.basename(String(upload.name || ''))
    if (!ZIP_NAME_RE.test(name)) return fail(res, 400, 'Invalid file name!')
    if (upload.truncated) return fail(res, 413, 'Model package too large!')
    if (!upload.size) return fail(res, 400, 'Empty model package!')
    if (upload.data.length < 4 || upload.data.readUInt32LE(0) !== 0x04034b50)
      return fail(res, 400, 'Not a zip file!')

    const bucket = new GridFSBucket(mdb(), { bucketName: BUCKET })
    const artifactId = await new Promise((resolve, reject) => {
      const up = bucket.openUploadStream(name, {
        metadata: { kind: 'upload', modelId: modelId, by: username(req) },
      })
      up.once('error', reject)
      up.once('finish', () => resolve(up.id))
      up.end(upload.data)
    })
    const id = await insertRequest(req, 'import', {
      modelId: modelId,
      artifactId: artifactId,
      fileName: name,
      size: upload.size,
    })
    res.status(200).send({ error: false, requestId: id })
  } catch (err) {
    Log.log('cimEms uploadModel: ' + err)
    fail(res, 200, String(err))
  }
}

// body: { type, modelId?, revision?, bindings?, connectionNumber? }
exports.createRequest = async (req, res) => {
  try {
    const type = String(req.body?.type || '')
    if (!(type in REQUEST_TYPES)) return fail(res, 400, 'Unknown request type!')
    if (REQUEST_TYPES[type] && !(await hasCimEngineering(req)))
      return fail(res, 403, 'Require CIM-EMS engineering rights!')

    const params = {}
    if (type === 'rerun') {
      const conn = positiveInt(req.body?.connectionNumber)
      if (!conn) return fail(res, 400, 'Invalid connectionNumber!')
      params.connectionNumber = conn
    } else {
      const modelId = String(req.body?.modelId || '')
      if (!MODEL_ID_RE.test(modelId)) return fail(res, 400, 'Invalid modelId!')
      params.modelId = modelId
      if (
        type === 'activate' ||
        type === 'proposeBindings' ||
        type === 'listElements'
      ) {
        const rev = positiveInt(req.body?.revision)
        if (type === 'activate' && !rev)
          return fail(res, 400, 'Invalid revision!')
        if (rev) params.revision = rev
      }
    }

    if (type === 'saveBindings') {
      const list = req.body?.bindings
      if (!Array.isArray(list) || list.length > MAX_BINDINGS)
        return fail(res, 400, 'Invalid bindings list!')
      const clean = []
      for (const b of list) {
        const s = sanitizeBinding(b)
        if (!s) return fail(res, 400, 'Invalid binding entry!')
        clean.push(s)
      }
      // the list can exceed the 16 MB document limit: keep it as an artifact
      const bucket = new GridFSBucket(mdb(), { bucketName: BUCKET })
      params.artifactId = await new Promise((resolve, reject) => {
        const up = bucket.openUploadStream(
          'bindings_' + params.modelId + '.json',
          {
            metadata: {
              kind: 'binding-set',
              modelId: params.modelId,
              by: username(req),
            },
          }
        )
        up.once('error', reject)
        up.once('finish', () => resolve(up.id))
        up.end(Buffer.from(JSON.stringify(clean)))
      })
      params.count = clean.length
      params.approved = clean.filter((b) => b.approved).length
    }

    const id = await insertRequest(req, type, params)
    res.status(200).send({ error: false, requestId: id })
  } catch (err) {
    Log.log('cimEms createRequest: ' + err)
    fail(res, 200, String(err))
  }
}

// GET ?id=<artifact ObjectId>: streams one cimArtifacts file as an attachment
exports.downloadArtifact = async (req, res) => {
  try {
    const raw = String(req.query?.id || '')
    if (!ObjectId.isValid(raw) || raw.length !== 24)
      return fail(res, 400, 'Invalid artifact id!')
    const id = new ObjectId(raw)
    const bucket = new GridFSBucket(mdb(), { bucketName: BUCKET })
    const files = await bucket.find({ _id: id }).toArray()
    if (!files.length) return fail(res, 404, 'Artifact not found!')
    const name = path
      .basename(files[0].filename || raw)
      .replace(/[^A-Za-z0-9._-]/g, '_')
    registerUserAction(req, 'cimEms:downloadArtifact', {
      id: raw,
      filename: name,
    })
    res.type(
      name.toLowerCase().endsWith('.json') ?
        'application/json'
      : 'application/octet-stream'
    )
    res.setHeader('Content-Disposition', 'attachment; filename="' + name + '"')
    res.setHeader('Cache-Control', 'no-store')
    bucket
      .openDownloadStream(id)
      .on('error', () => res.end())
      .pipe(res)
  } catch (err) {
    Log.log('cimEms downloadArtifact: ' + err)
    fail(res, 200, String(err))
  }
}
