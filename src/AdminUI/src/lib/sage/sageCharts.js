/*
 * Native SVG chart markups for the Display Viewer: #radar (spider chart) and
 * #arc (donut/gauge arc). Reimplemented without the legacy d3/radar-chart libs.
 *
 * Both are bound to a placeholder <rect>; the rect is hidden and the chart is
 * drawn into a <g> positioned/scaled to it, refreshed with live point values.
 *
 * {json:scada} - Copyright 2020-2026 - Ricardo L. Olsen
 */

const SVGNS = 'http://www.w3.org/2000/svg'

function escapeXml(s) {
  return String(s == null ? '' : s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
}

function elFill(el, fallback) {
  return (el.style && el.style.fill) || el.getAttributeNS(null, 'fill') || fallback
}

// Create/replace a <g id="<prefix>_<elid>"> with the given inner SVG + transform.
function injectChart(engine, el, prefix, innerSvg, transform) {
  const id = prefix + '_' + el.id
  let g = engine.svgEl.getElementById(id)
  if (!g) {
    g = document.createElementNS(SVGNS, 'g')
    g.setAttribute('id', id)
    el.parentNode.appendChild(g)
  }
  g.innerHTML = innerSvg
  g.setAttribute('transform', transform)
  return g
}

// --- #radar (spider chart) ---------------------------------------------------
export function initRadarChart(engine, binding, el) {
  if (el.style) el.style.display = 'none'
  binding.radarPoints = String(binding.src || '')
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)
  try {
    binding.radarConfig = binding.prompt ? JSON.parse(binding.prompt) : {}
  } catch (e) {
    binding.radarConfig = {}
  }
  binding.radarPoints.forEach((p) => engine.collectPoint(p))
  drawRadar(engine, binding, el)
}

export function drawRadar(engine, binding, el) {
  const pts = binding.radarPoints
  if (!pts || pts.length === 0) return
  const cfg = binding.radarConfig || {}
  const N = pts.length
  const size = 200
  const cx = size / 2
  const cy = size / 2
  const R = 70
  const levels = 4
  const values = pts.map((p) => {
    const v = engine.valorTagueado(p)
    return typeof v === 'number' && isFinite(v) ? v : 0
  })
  const max = cfg.maxValue || Math.max(1, ...values)
  const ang = (i) => (Math.PI * 2 * i) / N - Math.PI / 2
  const at = (r, i) => [cx + r * Math.cos(ang(i)), cy + r * Math.sin(ang(i))]
  let svg = ''
  // concentric grid polygons
  for (let l = 1; l <= levels; l++) {
    const rr = (R * l) / levels
    let poly = ''
    for (let i = 0; i < N; i++) {
      const [x, y] = at(rr, i)
      poly += (i ? ' ' : '') + x.toFixed(1) + ',' + y.toFixed(1)
    }
    svg += `<polygon points="${poly}" fill="none" stroke="#bbbbbb" stroke-width="0.5"/>`
  }
  // axes + labels
  for (let i = 0; i < N; i++) {
    const [x, y] = at(R, i)
    svg += `<line x1="${cx}" y1="${cy}" x2="${x.toFixed(1)}" y2="${y.toFixed(1)}" stroke="#bbbbbb" stroke-width="0.5"/>`
    if (cfg.axisText !== false) {
      const st = engine.getState(pts[i])
      const label = st ? st.bay || '' : ''
      const [lx, ly] = at(R + 12, i)
      svg += `<text x="${lx.toFixed(1)}" y="${ly.toFixed(1)}" font-size="9" text-anchor="middle" dominant-baseline="middle" fill="#333333">${escapeXml(label)}</text>`
    }
  }
  // data polygon + vertex dots
  const fill = elFill(el, 'steelblue')
  let dpoly = ''
  const verts = []
  for (let i = 0; i < N; i++) {
    const r = (values[i] / max) * R
    const [x, y] = at(r, i)
    verts.push([x, y])
    dpoly += (i ? ' ' : '') + x.toFixed(1) + ',' + y.toFixed(1)
  }
  svg += `<polygon points="${dpoly}" fill="${fill}" fill-opacity="0.5" stroke="${fill}" stroke-width="1.5"/>`
  for (const [x, y] of verts) {
    svg += `<circle cx="${x.toFixed(1)}" cy="${y.toFixed(1)}" r="2" fill="${fill}"/>`
  }

  const w = parseFloat(el.getAttributeNS(null, 'width')) || size
  const h = parseFloat(el.getAttributeNS(null, 'height')) || size
  const x = el.getAttributeNS(null, 'x') || 0
  const y = el.getAttributeNS(null, 'y') || 0
  const base = el.getAttributeNS(null, 'transform') || ''
  injectChart(engine, el, 'radar', svg, `${base} translate(${x} ${y}) scale(${w / size} ${h / size})`)
}

// --- #arc (donut / gauge arc) ------------------------------------------------
export function initArcChart(engine, binding, el) {
  if (el.style) el.style.display = 'none'
  binding.arcPoint = String(binding.src || '').trim()
  const params = String(binding.prompt || '').split(',')
  binding.arcMin = parseFloat(params[0]) || 0
  binding.arcMax = parseFloat(params[1])
  if (isNaN(binding.arcMax)) binding.arcMax = 100
  binding.arcInner = parseFloat(params[2]) || 0
  engine.collectPoint(binding.arcPoint)
  drawArc(engine, binding, el)
}

export function drawArc(engine, binding, el) {
  const v = engine.valorTagueado(binding.arcPoint)
  const val = typeof v === 'number' && isFinite(v) ? v : binding.arcMin
  let prop = (val - binding.arcMin) / (binding.arcMax - binding.arcMin)
  if (prop < 0) prop = 0
  if (prop > 1) prop = 1

  // modern dashboard ring gauge: light track + colored value arc + centered value
  const outerR = 100
  const innerR = binding.arcInner > 0 ? binding.arcInner : 64
  const midR = (innerR + outerR) / 2
  const sw = outerR - innerR
  const circ = 2 * Math.PI * midR
  const fill = elFill(el, '#4575b4')
  const st = engine.getState(binding.arcPoint)
  const label = st ? st.bay || '' : ''
  const numText = Math.abs(val) >= 100 ? Math.round(val) : Math.round(val * 10) / 10

  let svg = `<circle cx="0" cy="0" r="${midR.toFixed(1)}" fill="none" stroke="#e6e6e6" stroke-width="${sw}"/>`
  if (prop > 0) {
    svg +=
      `<circle cx="0" cy="0" r="${midR.toFixed(1)}" fill="none" stroke="${fill}" stroke-width="${sw}"` +
      ` stroke-linecap="round" stroke-dasharray="${(prop * circ).toFixed(2)} ${circ.toFixed(2)}" transform="rotate(-90)"/>`
  }
  svg += `<text x="0" y="0" font-size="${innerR * 0.62}" font-weight="bold" text-anchor="middle" dominant-baseline="central" fill="#222222">${numText}</text>`
  if (label) {
    svg += `<text x="0" y="${(outerR + 22).toFixed(0)}" font-size="22" text-anchor="middle" fill="#666666">${escapeXml(label)}</text>`
  }

  // use the rect's attributes (works even though the rect is hidden via display:none)
  const w = parseFloat(el.getAttributeNS(null, 'width')) || 200
  const h = parseFloat(el.getAttributeNS(null, 'height')) || 200
  const x = parseFloat(el.getAttributeNS(null, 'x')) || 0
  const y = parseFloat(el.getAttributeNS(null, 'y')) || 0
  const base = el.getAttributeNS(null, 'transform') || ''
  const cx = x + w / 2
  const cy = y + h / 2
  // native art spans ~±100 plus the label below; scale to fit the rect with margin
  injectChart(engine, el, 'arc', svg, `${base} translate(${cx} ${cy}) scale(${w / 250} ${h / 250})`)
}
