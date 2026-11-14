/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

import type { Collection, Document } from 'mongodb'

/** MongoDB PathNotViable: a dotted $set below a null element. */
const PATH_NOT_VIABLE = 28

/**
 * $set stats.<field> on a protocolConnections document. Connection documents
 * are created with stats: null, where a dotted $set fails with error 28:
 * replace the null with an empty document and retry once.
 */
export async function updateConnectionStats(
  coll: Pick<Collection<Document>, 'updateOne'>,
  connNumber: number,
  fields: Record<string, unknown>
): Promise<void> {
  const set: Document = {}
  for (const [k, v] of Object.entries(fields)) set['stats.' + k] = v
  const filter = { protocolConnectionNumber: connNumber }
  try {
    await coll.updateOne(filter, { $set: set })
  } catch (err) {
    if ((err as { code?: unknown })?.code !== PATH_NOT_VIABLE) throw err
    await coll.updateOne({ ...filter, stats: null }, { $set: { stats: {} } })
    await coll.updateOne(filter, { $set: set })
  }
}
