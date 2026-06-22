/*
 * Vega chart support for the native Display Viewer (modern Vega 5).
 *
 * Renders a Vega spec attached to a <rect> via a SAGE `script` "vega"/"vega4"
 * binding, injecting the chart SVG into the screen at the rect's position/scale.
 * Each refresh, the data table's PNT#/BAY#/SUB#/TAG#/DCR#/FLG#/LMI#/LMS#
 * placeholders are substituted with the linked points' live values and the chart
 * is re-rendered.
 *
 * {json:scada} - Copyright 2020-2026 - Ricardo L. Olsen
 */

import * as vega from 'vega'
import * as vegaLite from 'vega-lite'

const SVGNS = 'http://www.w3.org/2000/svg'
const PLACEHOLDER = /^(PNT|BAY|SUB|TAG|DCR|FLG|FLR|LMI|LMS)#(\d+)$/

// Initialize a Vega chart bound to `el` (a <rect>) using binding.vegaSpec (spec
// JSON) and binding.vegaPoints (point list). Hides the rect, renders, injects.
// binding.isVegaLite -> compile a Vega-Lite spec to Vega first.
export async function initVegaChart(engine, binding, el) {
  let spec
  try {
    spec = JSON.parse(binding.vegaSpec)
  } catch (e) {
    engine.onStatus('Vega spec parse error: ' + e.message)
    return
  }
  if (el.style) el.style.display = 'none' // hide the placeholder rect

  const isVL = binding.isVegaLite || /vega-lite/.test(spec.$schema || '')
  if (isVL) {
    // Vega-Lite data is an object {name, values}; capture before compiling.
    binding.vgTableName = (spec.data && spec.data.name) || 'source_0'
    binding.vgInitData = JSON.parse(JSON.stringify((spec.data && spec.data.values) || []))
    try {
      spec = vegaLite.compile(spec).spec
    } catch (e) {
      engine.onStatus('Vega-Lite compile error: ' + e.message)
      return
    }
  } else {
    binding.vgTableName = (spec.data && spec.data[0] && spec.data[0].name) || 'table'
    binding.vgInitData = JSON.parse(JSON.stringify((spec.data && spec.data[0] && spec.data[0].values) || []))
  }
  try {
    const view = new vega.View(vega.parse(spec), { renderer: 'none' })
    binding.vw = view
    await view.runAsync()
    const svgStr = await view.toSVG()
    injectVega(engine, binding, el, svgStr)
  } catch (e) {
    engine.onStatus('Vega init error: ' + e.message)
  }
}

// Build/replace the <g id="vg_<id>"> container holding the chart, positioned and
// scaled to the rect.
function injectVega(engine, binding, el, svgStr) {
  const svgEl = engine.svgEl
  const tmp = new DOMParser().parseFromString(svgStr, 'image/svg+xml')
  const vsvg = tmp.documentElement
  const vw = parseFloat(vsvg.getAttribute('width')) || 200
  const vh = parseFloat(vsvg.getAttribute('height')) || 200

  let g = svgEl.getElementById('vg_' + el.id)
  if (!g) {
    g = document.createElementNS(SVGNS, 'g')
    g.setAttribute('id', 'vg_' + el.id)
    el.parentNode.appendChild(g)
  }
  while (g.firstChild) g.removeChild(g.firstChild)
  for (const child of Array.from(vsvg.childNodes)) {
    g.appendChild(document.importNode(child, true))
  }

  const w = parseFloat(el.getAttributeNS(null, 'width')) || vw
  const h = parseFloat(el.getAttributeNS(null, 'height')) || vh
  const x = el.getAttributeNS(null, 'x') || 0
  const y = el.getAttributeNS(null, 'y') || 0
  const base = el.getAttributeNS(null, 'transform') || ''
  g.setAttribute('transform', `${base} translate(${x} ${y}) scale(${w / vw} ${h / vh})`)
  binding.vg = g
}

function resolvePlaceholder(engine, val, points) {
  const m = String(val).match(PLACEHOLDER)
  if (!m) return { ok: true, value: val }
  const pt = points[parseInt(m[2]) - 1]
  if (pt === undefined || pt === '') return { ok: false } // no such point -> drop row
  const st = engine.getState(pt)
  switch (m[1]) {
    case 'PNT': {
      const v = engine.valorTagueado(pt)
      return typeof v === 'number' ? { ok: true, value: v } : { ok: false }
    }
    case 'BAY':
      return { ok: true, value: st ? st.bay : '' }
    case 'SUB':
      return { ok: true, value: st ? st.station : '' }
    case 'TAG':
      return { ok: true, value: st ? st.tag : '' }
    case 'DCR':
      return { ok: true, value: st ? st.descr : '' }
    case 'FLG':
      return { ok: true, value: st ? st.flags : 0 }
    case 'FLR':
      return { ok: true, value: st && st.flags & 0x80 ? 1 : 0 }
    case 'LMI':
      return { ok: true, value: st && isFinite(st.loLimit) ? st.loLimit : 0 }
    case 'LMS':
      return { ok: true, value: st && isFinite(st.hiLimit) ? st.hiLimit : 0 }
    default:
      return { ok: true, value: val }
  }
}

// Substitute placeholders in the data template and re-render the chart.
export async function updateVegaChart(engine, binding) {
  if (!binding.vw || !binding.vgInitData || binding._vegaBusy) return
  binding._vegaBusy = true
  try {
    const newdata = []
    for (const row of binding.vgInitData) {
      const out = {}
      let drop = false
      for (const k in row) {
        const r = resolvePlaceholder(engine, row[k], binding.vegaPoints)
        if (!r.ok) {
          drop = true
          break
        }
        out[k] = r.value
      }
      if (!drop) newdata.push(out)
    }
    binding.vw
      .change(binding.vgTableName, vega.changeset().remove(() => true).insert(newdata))
      .run()
    await binding.vw.runAsync()
    const svgStr = await binding.vw.toSVG()
    injectVega(engine, binding, binding.parent, svgStr)
  } catch (e) {
    /* keep animating */
  } finally {
    binding._vegaBusy = false
  }
}
