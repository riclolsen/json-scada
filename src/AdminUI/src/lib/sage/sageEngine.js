/*
 * SAGE Display Viewer engine — native rewrite of the core of websage.js.
 *
 * Parses SAGE/Inkscape-tagged SVG screens (inkscape:label JSON tag-objects),
 * collects referenced points, polls realtime data via the shared opcClient, and
 * animates the SVG each refresh (color/bar/rotate/opac/slider/get-text/text-map/
 * tooltips), plus point-access click wiring, zoom/pan, blink and beep status.
 *
 * Advanced legacy features (Vega charts, live trend plots on <rect>, radar,
 * camera/foreign-object, Time Machine historical replay) are recognized but
 * deferred — see the methods marked DEFERRED.
 *
 * {json:scada} - Copyright 2020-2026 - Ricardo L. Olsen
 */

import * as opc from '../opcClient'
import { OpcStatusCodes, OpcValueTypes } from '../opcCodes'
import { buildColorTable, traduzCor } from './sageColors'
import { printf } from './sageFormat'
import { initVegaChart, updateVegaChart } from './sageVega'
import { initRadarChart, drawRadar, initArcChart, drawArc } from './sageCharts'
import { PreviewOverlay } from './sagePreview'
import { PinnedAnnotationsPanel } from './sagePinned'
import { AlarmBoxPanel } from './sageAlarmBox'

// "security card" badge shape (port of websage.js produzEtiq) shown next to a
// point that carries an annotation or has alarms inhibited.
const SECURITY_CARD_PATH =
  'M0.413787841796875 0.5425290489196777L0.413787841796875 5.095224800109864' +
  'L7.873085021972656 11.345515670776367L12.482757568359375 6.48417896270752' +
  'L5.6939697265625 0.38820165634155274L0.4976043701171875 0.31103796005249024' +
  'L0.413787841796875 0.6196927452087402'

const RETNOK = '?#?'
const SVGNS = 'http://www.w3.org/2000/svg'
const INKNS = 'http://www.inkscape.org/namespaces/inkscape'

// SMIL animation helpers exposed to screen scripts (legacy $W.Animate/RemoveAnimate)
function animateEl(obj, type, attrs) {
  if (!obj || !obj.appendChild) return
  const a = document.createElementNS(SVGNS, type || 'animate')
  if (attrs) for (const k in attrs) a.setAttributeNS(null, k, attrs[k])
  obj.appendChild(a)
  if (typeof a.beginElement === 'function') {
    try {
      a.beginElement()
    } catch (e) {
      /* ignore */
    }
  }
  return a
}
function removeAnimateEl(obj) {
  if (!obj || !obj.querySelectorAll) return
  obj.querySelectorAll('animate, animateTransform, animateMotion, animateColor').forEach((n) => n.remove())
}

// linear RGB mix (chroma.mix substitute for color interpolation anchors)
function rgbMix(a, b, t) {
  const ctx = document.createElement('canvas').getContext('2d')
  const parse = (c) => {
    ctx.fillStyle = c
    const h = ctx.fillStyle
    return [parseInt(h.slice(1, 3), 16), parseInt(h.slice(3, 5), 16), parseInt(h.slice(5, 7), 16)]
  }
  const ca = parse(a)
  const cb = parse(b)
  const tt = Math.max(0, Math.min(1, t))
  const mix = ca.map((v, i) => Math.round(v + (cb[i] - v) * tt))
  return `rgb(${mix[0]},${mix[1]},${mix[2]})`
}

export class SageEngine {
  constructor({ container, cfg, onOpenPoint, onAlarmBeep, onStatus, onScreenLink }) {
    this.container = container
    this.cfg = cfg
    this.onOpenPoint = onOpenPoint || (() => {})
    this.onAlarmBeep = onAlarmBeep || (() => {})
    this.onStatus = onStatus || (() => {})
    this.onScreenLink = onScreenLink || (() => {})
    this.colorTable = buildColorTable(cfg)

    this.svgEl = null
    this.bindings = []
    this.plotBindings = []
    this.annotEls = [] // elements anchoring a pinned-annotation badge
    this._plotsFilled = false
    this.preview = new PreviewOverlay(cfg)
    this.pinnedPanel = new PinnedAnnotationsPanel(cfg)
    this.pinnedTimer = null
    this.alarmBox = new AlarmBoxPanel(cfg, (k) => this.onOpenPoint(k))
    this.screenFilter = '' // group1 filter from #set_filter markup
    this.queryKeys = new Set()
    this.valueByKey = new Map()
    this.keyByTag = new Map()
    this.zoom = {
      x: 0,
      y: 0,
      w: cfg.ScreenViewer_SVGMaxWidth,
      h: cfg.ScreenViewer_SVGMaxHeight,
    }
    this.fitZoom = null
    this.refreshTimer = null
    this.blinkTimer = null
    this.blinkOn = true
    this.blinkList = []
    this.disposed = false
    this.pass = 0
    this.timeMachine = false // historical-replay mode (pauses realtime polling)
    this.tmTime = null

    // per-refresh script bindings (#exec_on_update / script evt exec_on_update)
    this.execOnUpdate = []
    // tooltips whose text contains inline !EVAL ... !END expressions
    this.dynTooltips = []
    this.setupScriptApi()
  }

  // Build the value/flag lookups (V/F/S/T) and the $W/WebSAGE compat object that
  // screen-authored !EVAL expressions and exec/script snippets reference (ported
  // from the legacy global WebSAGE arrays/functions).
  setupScriptApi() {
    const byKey = (k) => (typeof k === 'string' || typeof k === 'number' ? this.valueByKey.get(parseInt(k)) : undefined)
    const proxy = (pick) =>
      new Proxy(
        {},
        {
          get: (_t, k) => {
            const st = byKey(k)
            return st ? pick(st) : pick(null)
          },
        }
      )
    this._V = proxy((st) => (st ? st.value : 0))
    this._F = proxy((st) => (st ? st.flags : 0))
    this._S = proxy((st) => (st ? st.valueStr : ''))
    this._T = proxy((st) => (st ? st.alarmTime : ''))
    this._scriptApi = {
      V: this._V,
      F: this._F,
      S: this._S,
      T: this._T,
      getValue: (t) => this.getValue(t),
      getFlags: (t) => this.getFlags(t),
      valorTagueado: (t, o) => this.valorTagueado(t, o),
      Animate: (obj, type, attrs) => animateEl(obj, type, attrs),
      RemoveAnimate: (obj) => removeAnimateEl(obj),
    }
  }

  // Evaluate a screen expression and return its value (for !EVAL). Returns
  // undefined on error. `el` is exposed as `thisobj`.
  evalValue(expr, el) {
    try {
      // eslint-disable-next-line no-new-func
      const fn = new Function('thisobj', 'SVGDoc', 'V', 'F', 'S', 'T', 'WebSAGE', '$W', 'return (' + expr + ')')
      return fn(el, this.svgEl, this._V, this._F, this._S, this._T, this._scriptApi, this._scriptApi)
    } catch (e) {
      return undefined
    }
  }

  // Run a screen script snippet (statements) with the compat API in scope.
  runScript(src, el) {
    try {
      // eslint-disable-next-line no-new-func
      const fn = new Function('thisobj', 'SVGDoc', 'V', 'F', 'S', 'T', 'WebSAGE', '$W', src)
      fn(el, this.svgEl, this._V, this._F, this._S, this._T, this._scriptApi, this._scriptApi)
    } catch (e) {
      /* ignore screen-script errors */
    }
  }

  // Replace inline `!EVAL <expr> !END <tail>` segments in `text` with the
  // evaluated (numeric→%1.3f) result. Used by dynamic tooltips / text.
  evalInline(text, el) {
    let out = String(text)
    let guard = 0
    while (out.indexOf('!EVAL') !== -1 && guard++ < 32) {
      const pini = out.indexOf('!EVAL')
      const pend = out.indexOf('!END', pini + 1)
      const exprEnd = pend === -1 ? out.length : pend
      const expr = out.substring(pini + 5, exprEnd)
      let val = this.evalValue(expr, el)
      const n = Number(val)
      if (val !== undefined && val !== null && isFinite(n)) val = printf('%1.3f', n)
      const tail = pend === -1 ? '' : out.substring(pend + 4)
      out = out.substring(0, pini) + (val == null ? '' : val) + tail
    }
    return out
  }

  // --- screen load -----------------------------------------------------------
  async loadScreen(svgUrl, opts = {}) {
    this.stop()
    this.bindings = []
    this.plotBindings = []
    this.annotEls = []
    this.execOnUpdate = []
    this.dynTooltips = []
    this._plotsFilled = false
    this.preview.hide()
    this.pinnedPanel.hide()
    this.alarmBox.hide()
    this.screenFilter = ''
    this.queryKeys.clear()
    // Reset to pass 0 so this screen's first refresh does a FULL read (askInfo):
    // group1/group2(bay)/descr/stateText/limits only come back on the full read.
    // Without this, switching screens (engine reused) left pass>0, so new points
    // got value-only reads and labels like BAY#/DCR# stayed blank until a reload.
    this.pass = 0
    this.valueByKey.clear()
    this.keyByTag.clear()
    const resp = await fetch(svgUrl)
    const text = await resp.text()
    this.container.innerHTML = text
    this.svgEl = this.container.querySelector('svg')
    if (!this.svgEl) throw new Error('No <svg> root in ' + svgUrl)

    // initial viewBox from authored size (read BEFORE overriding width/height)
    this.computeInitialZoom(opts)
    this.svgEl.setAttribute('width', '100%')
    this.svgEl.setAttribute('height', '100%')
    this.applyViewBox()
    this.setBackground(this.cfg.VisorTelas_BackgroundSVG || this.cfg.ScreenViewer_Background)

    // expose ShowHideTranslate for screen scripts (e.g. the section "eye" toggles),
    // scoped to this engine's SVG.
    window.ShowHideTranslate = (idorobj, xd, yd) => this.showHideTranslate(idorobj, xd, yd)

    await this.runEmbeddedScripts()
    this.parse()
    this.startBlink()
    await this.refresh()
    this.startPinnedAnnotations()
  }

  // --- pinned documental annotations (#PIN notes) ----------------------------
  // If the screen declared a group1 filter via #set_filter, periodically fetch
  // that group1's points and list those whose `notes` contain "#PIN".
  startPinnedAnnotations() {
    clearInterval(this.pinnedTimer)
    this.pinnedTimer = null
    this.pinnedPanel.hide()
    if (!this.screenFilter) return
    this.refreshPinnedAnnotations()
    const sec = this.cfg.ScreenViewer_PinnedAnnotationsRefresh || 9.5
    this.pinnedTimer = setInterval(() => this.refreshPinnedAnnotations(), sec * 1000)
  }

  async refreshPinnedAnnotations() {
    if (this.disposed || !this.screenFilter) return
    try {
      const pts = await opc.readFiltered({ station: this.screenFilter }, this.cfg)
      const items = []
      for (const p of pts) {
        if (typeof p.notes === 'string' && p.notes.indexOf('#PIN') !== -1) {
          let d = p.descr || ''
          if (p.bay && d.indexOf(p.bay + ' | ') === 0) d = d.substring((p.bay + ' | ').length)
          items.push({
            sub: p.station || '',
            bay: p.bay || '',
            descr: d,
            note: p.notes.replace(/#PIN/g, '').trim(),
          })
        }
      }
      this.pinnedPanel.render(items)
    } catch (e) {
      /* keep the last rendered list */
    }
  }

  // Toggle display (and optional translate) of an element by id — port of the
  // legacy global ShowHideTranslate(), scoped to this engine's SVG.
  showHideTranslate(idorobj, xd = 0, yd = 0) {
    if (!this.svgEl) return
    const obj = typeof idorobj === 'object' ? idorobj : this.svgEl.getElementById(idorobj)
    if (!obj) return
    obj.style.display = obj.style.display === 'none' ? 'block' : 'none'
    if (obj._inittransform === undefined) obj._inittransform = obj.getAttributeNS(null, 'transform') || ''
    if (parseFloat(xd) !== 0 || parseFloat(yd) !== 0) {
      obj.setAttributeNS(null, 'transform', obj._inittransform + ' translate(' + parseFloat(xd) + ' ' + parseFloat(yd) + ')')
    }
  }

  // Determine the initial viewBox: explicit opts.zoom, else the SVG's authored
  // viewBox or width/height, else the configured max canvas. Caches it as fitZoom
  // so the "center/fit" control returns here.
  computeInitialZoom(opts) {
    let x = 0
    let y = 0
    let w = 0
    let h = 0
    const vb = this.svgEl.getAttribute('viewBox')
    if (vb) {
      const p = vb.split(/[ ,]+/).map(parseFloat)
      if (p.length === 4 && p[2] > 0) {
        x = p[0]
        y = p[1]
        w = p[2]
        h = p[3]
      }
    }
    if (!w) {
      const pw = parseFloat(this.svgEl.getAttribute('width'))
      const ph = parseFloat(this.svgEl.getAttribute('height'))
      if (pw > 0 && ph > 0) {
        w = pw
        h = ph
      }
    }
    if (!w) {
      w = this.cfg.ScreenViewer_SVGMaxWidth
      h = this.cfg.ScreenViewer_SVGMaxHeight
    }
    // Fit to width for readability (matches the legacy viewer): make the viewBox
    // aspect match the container so content fills the width; taller content is
    // pannable downward. Falls back to the authored box if the container is unsized.
    const cw = this.container ? this.container.clientWidth : 0
    const ch = this.container ? this.container.clientHeight : 0
    if (cw > 0 && ch > 0) {
      h = w * (ch / cw)
    }
    this.fitZoom = { x, y, w, h }
    this.zoom = opts && opts.zoom ? { ...opts.zoom } : { ...this.fitZoom }
  }

  // Execute scripts embedded in the SVG (authored, trusted content). Guarded.
  async runEmbeddedScripts() {
    const scripts = this.svgEl.getElementsByTagName('script')
    let code = ''
    let skippedExternal = 0
    for (let i = 0; i < scripts.length; i++) {
      const href =
        scripts[i].getAttributeNS('http://www.w3.org/1999/xlink', 'href') ||
        scripts[i].getAttribute('href')
      // Do NOT load external <script href=...> libraries: unlike the standalone
      // legacy display.html (which owns the whole page), here the viewer runs
      // inside the AdminUI SPA. Some screens embed full standalone-SVG GUI
      // frameworks (e.g. pergola.js) that seize document.documentElement and
      // collapse the host app to 0x0. Only run the screen's own inline scripts.
      if (href) {
        skippedExternal++
        continue
      }
      try {
        code += scripts[i].textContent + '\n'
      } catch (e) {
        /* ignore individual script errors */
      }
    }
    if (skippedExternal > 0)
      this.onStatus(`Skipped ${skippedExternal} external screen script(s)`)
    if (code.trim() !== '') {
      try {
        // eslint-disable-next-line no-new-func
        new Function('SVGDoc', code)(this.svgEl)
      } catch (e) {
        this.onStatus('Screen script error: ' + e.message)
      }
    }
  }

  // --- tag parsing -----------------------------------------------------------
  parse() {
    const all = this.svgEl.querySelectorAll('*')
    all.forEach((el) => this.parseElement(el))
  }

  getLabel(el) {
    return (
      el.getAttributeNS(INKNS, 'label') ||
      el.getAttribute('inkscape:label') ||
      null
    )
  }

  // Resolve a clone tag like "%n" or "!SLIM%n" against the nearest ancestor
  // group's clone map (["%n=26549", ...]). Returns the tag unchanged if no match.
  resolveCloneTag(el, tag) {
    if (tag == null) return tag
    let s = String(tag)
    if (s.indexOf('%') < 0) return tag
    // Replace EVERY occurrence of EVERY clone-map variable, walking from the
    // nearest ancestor map outward (inner maps win, since their substitution
    // runs first and removes the pattern). Handles multi-var !EVAL expressions
    // like `V[%n]+V[%m]`, not just a single leading %n.
    let node = el.parentNode
    while (node && node !== this.svgEl && node.nodeType === 1) {
      if (node._cloneMap) {
        for (const m of node._cloneMap) {
          const eq = m.indexOf('=')
          if (eq < 0) continue
          const pat = m.substring(0, eq)
          const val = m.substring(eq + 1)
          if (pat && s.indexOf(pat) !== -1) s = s.split(pat).join(val)
        }
      }
      node = node.parentNode
    }
    return s
  }

  parseElement(el) {
    const label = this.getLabel(el)
    if (label) this.parseLabel(el, label)
  }

  parseLabel(el, label) {
    let vec
    try {
      vec = JSON.parse('[' + label + ']')
    } catch (e) {
      return
    }
    // First pass: capture a clone map so descendant %n tags can resolve.
    for (const o of vec) {
      if (o.attr === 'clone' && Array.isArray(o.map)) el._cloneMap = o.map
    }
    for (const tagObj of vec) {
      tagObj.parent = el
      if (tagObj.tag !== undefined) tagObj.tag = this.resolveCloneTag(el, tagObj.tag)
      switch (tagObj.attr) {
        case 'get':
          this.parseGet(el, tagObj)
          break
        case 'color':
          this.parseColor(el, tagObj)
          break
        case 'bar':
          tagObj.initheight = parseFloat(el.getAttributeNS(null, 'height'))
          this.collectPoint(tagObj.tag)
          this.wirePopup(el, tagObj.tag)
          break
        case 'opac':
          this.collectPoint(tagObj.tag)
          this.wirePopup(el, tagObj.tag)
          break
        case 'rotate':
          tagObj.inittransform = el.getAttributeNS(null, 'transform') || ''
          this.collectPoint(tagObj.tag)
          this.wirePopup(el, tagObj.tag)
          break
        case 'slider':
          this.parseSlider(el, tagObj)
          break
        case 'text':
          this.collectPoint(tagObj.tag)
          this.wirePopup(el, tagObj.tag)
          break
        case 'tooltips':
          this.parseTooltip(el, tagObj)
          break
        case 'popup':
          this.parsePopup(el, tagObj)
          break
        case 'open':
          this.parseOpen(el, tagObj)
          break
        case 'zoom':
          this.parseZoomRegion(el)
          break
        case 'set':
        case 'script':
          // DEFERRED: vega/radar/camera/foreign-object/exec — recognized, basic exec only
          this.parseSetScript(el, tagObj)
          break
        default:
          break
      }
      this.bindings.push(tagObj)
    }
    // Faceplate: a clone group carries ONE pinned tag for the whole symbol (its
    // primary %-variable point); its internal objects don't get their own (see
    // registerAnnotation). Done after the loop so notrace/block is already set.
    if (el._cloneMap && el._cloneMap.length && el.children && el.children.length) {
      const first = String(el._cloneMap[0])
      const eq = first.indexOf('=')
      const primary = eq >= 0 ? first.substring(eq + 1).trim() : ''
      if (primary) this.registerAnnotation(el, primary)
    }
  }

  parseGet(el, tagObj) {
    const tc = el.textContent || ''
    if (tc.indexOf('|') >= 0) {
      tagObj.txtOFFON = tc.split('|')
    } else {
      tagObj.formatoC = tc
      // single analog reading: hovering it previews the point's trend chart
      const tag = tagObj.tag
      this.preview.attach(
        el,
        () => {
          const key = this.resolveKey(tag)
          return key !== undefined ? 'trend.html?NPONTO=' + key + '&HIDECTRLS=1' : null
        },
        610,
        340
      )
    }
    this.collectPoint(tagObj.tag)
    this.wirePopup(el, tagObj.tag)
  }

  parseColor(el, tagObj) {
    tagObj.initfill = (el.style && el.style.fill) || el.getAttributeNS(null, 'fill') || ''
    tagObj.initstroke = (el.style && el.style.stroke) || el.getAttributeNS(null, 'stroke') || ''
    if (!Array.isArray(tagObj.list)) return
    tagObj.list.forEach((entry, j) => {
      entry.tag = this.resolveCloneTag(el, entry.tag)
      entry.cfill = ''
      entry.cstroke = ''
      entry.cscript = ''
      entry.cattrib = ''
      entry.cattribval = ''
      const param = entry.param || ''
      if (param.indexOf('attrib: ') === 0) {
        const arr = param.substr(8).split('=')
        if (arr.length > 1) {
          entry.cattrib = arr[0]
          entry.cattribval = arr[1]
        }
      } else if (param.indexOf('script: ') === 0) {
        entry.cscript = param.substr(8)
      } else {
        const arr = param.split('|')
        entry.cfill = traduzCor(arr[0], this.colorTable, this.cfg)
        entry.cstroke = arr.length > 1 ? traduzCor(arr[1], this.colorTable, this.cfg) : entry.cfill
      }
      this.collectPoint(entry.tag)
      if (j === 0) this.wirePopup(el, entry.tag)
    })
  }

  parseSlider(el, tagObj) {
    tagObj.inittransform = el.getAttributeNS(null, 'transform') || ''
    tagObj.min = parseFloat(tagObj.min)
    tagObj.max = parseFloat(tagObj.max)
    // find the <use> clone referencing this element to read the displacement range
    const id = el.getAttributeNS(null, 'id')
    const uses = this.svgEl.getElementsByTagName('use')
    for (let i = 0; i < uses.length; i++) {
      const href =
        uses[i].getAttributeNS('http://www.w3.org/1999/xlink', 'href') ||
        uses[i].getAttributeNS(null, 'href')
      if (href === '#' + id) {
        const ctf = uses[i].getAttributeNS(null, 'transform') || ''
        uses[i].style.display = 'none'
        const s1 = ctf.indexOf('(')
        let s2 = ctf.indexOf(',')
        const s3 = ctf.indexOf(')')
        if (s2 === -1) s2 = ctf.indexOf(' ')
        if (s2 === -1) s2 = s3
        tagObj.rangex = parseFloat(ctf.substring(s1 + 1, s2)) || 0
        tagObj.rangey = parseFloat(ctf.substring(s2 + 1, s3)) || 0
        break
      }
    }
    this.collectPoint(tagObj.tag)
    this.wirePopup(el, tagObj.tag)
  }

  parseTooltip(el, tagObj) {
    const params = Array.isArray(tagObj.param) ? tagObj.param : [tagObj.param]
    let text = this.resolveCloneTag(el, params.join('\n'))
    const title = document.createElementNS(SVGNS, 'title')
    el.appendChild(title)
    if (text.indexOf('!EVAL') !== -1) {
      // live tooltip: re-evaluate the inline !EVAL expressions each refresh
      this.dynTooltips.push({ titleEl: title, el, template: text })
      title.textContent = this.evalInline(text, el)
    } else {
      title.textContent = text
    }
  }

  updateDynamicTooltips() {
    for (const t of this.dynTooltips) {
      const txt = this.evalInline(t.template, t.el)
      if (t.titleEl.textContent !== txt) t.titleEl.textContent = txt
    }
  }

  parsePopup(el, tagObj) {
    const src = this.resolveCloneTag(el, tagObj.src)
    tagObj.src = src
    if (src === 'block') {
      tagObj.blockPopup = 1
      el.blockPopup = 1
      this.unregisterAnnotation(el) // never show a tag symbol on a blocked object
    } else if (src === 'notrace') {
      el.noTrace = 1
      this.unregisterAnnotation(el) // never show a tag symbol on a notrace object
    } else if (typeof src === 'string' && src.indexOf('preview:') === 0) {
      // hover preview of an arbitrary URL
      this.preview.attach(el, src.substr(8), tagObj.width, tagObj.height)
    } else {
      this.collectPoint(src)
      this.wirePopup(el, src)
      el.pontoPopup = src
    }
  }

  parseOpen(el, tagObj) {
    if (tagObj.istag) {
      // live trend plot drawn into a <rect>
      if (el.tagName !== 'rect') return
      const parts = String(this.resolveCloneTag(el, tagObj.src) || '').split('|')
      tagObj.tag = parts[0]
      tagObj.plotValMin = parseFloat(tagObj.y) || 0
      tagObj.plotValSpan = parseFloat(tagObj.height) || 1
      tagObj.windowSec = Math.abs(parseFloat(tagObj.width)) || 1800
      const poly = document.createElementNS(SVGNS, 'polyline')
      const stroke = (el.style && el.style.stroke) || el.getAttributeNS(null, 'stroke') || 'white'
      const sw = (el.style && el.style.strokeWidth) || el.getAttributeNS(null, 'stroke-width') || 2
      poly.setAttribute('style', `fill:none;stroke:${stroke};stroke-width:${sw}`)
      if (parts[1] && parts[1].trim() !== '') poly.setAttribute('style', parts[1])
      const tfm = el.getAttributeNS(null, 'transform')
      if (tfm) poly.setAttribute('transform', tfm)
      el.parentNode.appendChild(poly)
      tagObj.poly = poly
      tagObj.vals = []
      tagObj.times = []
      tagObj.histLoaded = false
      this.collectPoint(tagObj.tag)
      this.wirePopup(el, tagObj.tag)
      this.plotBindings.push(tagObj)
      return
    }
    const src = String(tagObj.src || '')
    if (src.indexOf('new:') === 0) {
      const url = src.substr(4)
      el.style.cursor = 'pointer'
      el.addEventListener('click', () =>
        window.open(url, '', `height=${tagObj.height},width=${tagObj.width}`)
      )
    } else if (src.indexOf('preview:') === 0) {
      // hover preview of an arbitrary URL
      this.preview.attach(el, src.substr(8), tagObj.width, tagObj.height)
    } else {
      // link to another screen (file name without path/extension)
      const screen = src.trim()
      el.style.cursor = 'pointer'
      el.addEventListener('click', () => this.onScreenLink(screen))
      // hovering a screen link previews that screen (zoomed-out, toolbar hidden)
      const name = screen.endsWith('.svg') ? screen : screen + '.svg'
      const zoom = this.cfg.ScreenViewer_DisplayPreviewZoom || 0.5
      const zpw = Math.round(this.cfg.ScreenViewer_SVGMaxWidth / zoom)
      const zph = Math.round(this.cfg.ScreenViewer_SVGMaxHeight / zoom)
      this.preview.attach(
        el,
        `display.html?SELTELA=../svg/${name}&ZPX=0&ZPY=0&ZPW=${zpw}&ZPH=${zph}&HIDETB=1`
      )
    }
  }

  parseZoomRegion(el) {
    el.style.cursor = 'pointer'
    el.addEventListener('click', () => {
      const bb = el.getBoundingClientRect()
      const ctm = this.svgEl.getScreenCTM().inverse()
      const p1 = this.toSvgPoint(bb.left, bb.top, ctm)
      const p2 = this.toSvgPoint(bb.right, bb.bottom, ctm)
      this.zoom = {
        x: p1.x,
        y: p1.y,
        w: (p2.x - p1.x) * 2,
        h: (p2.y - p1.y) * 2,
      }
      this.applyViewBox()
      el.style.display = 'none'
    })
  }

  parseSetScript(el, tagObj) {
    // script: attach DOM event handlers (mouseup/down/over/out/move, keydown) that
    // run the param with `thisobj`=element; exec_once runs once at parse. Used e.g.
    // by the "eye" toggles: window.ShowHideTranslate("gDetalhe230",0,0).
    if (tagObj.attr === 'script' && Array.isArray(tagObj.list)) {
      const mouseEvts = ['mouseup', 'mousedown', 'mouseover', 'mouseout', 'mousemove', 'click']
      const vegaEvts = ['vega', 'vega4', 'vega3', 'vega-lite']
      for (const it of tagObj.list) {
        const evt = it.evt
        const param = it.param || ''
        if (vegaEvts.includes(evt)) {
          // first line of param = comma-separated point list, rest = the spec
          const nl = param.indexOf('\n')
          tagObj.vegaPoints = (nl >= 0 ? param.slice(0, nl) : '')
            .split(',')
            .map((s) => s.trim())
            .filter(Boolean)
          tagObj.vegaSpec = nl >= 0 ? param.slice(nl + 1) : param
          tagObj._vega = true
          tagObj.isVegaLite = evt === 'vega-lite'
          tagObj.vegaPoints.forEach((p) => this.collectPoint(p))
          initVegaChart(this, tagObj, el)
        } else if (mouseEvts.includes(evt) || evt === 'keydown') {
          el.addEventListener(evt, (event) => {
            try {
              // eslint-disable-next-line no-new-func
              new Function('thisobj', 'evt', 'SVGDoc', param)(event.currentTarget, event, this.svgEl)
            } catch (e) {
              this.onStatus('script error: ' + e.message)
            }
          })
          if (evt.indexOf('mouse') >= 0 && el.style && !el.blockPopup) el.style.cursor = 'pointer'
        } else if (evt === 'exec_once') {
          this.runScript(param, el)
        } else if (evt === 'exec_on_update') {
          // run this snippet every refresh
          this.execOnUpdate.push({ el, src: param })
          this.runScript(param, el)
        }
      }
      return
    }
    // #copy_xsac_from: copy the SAGE binding(s) from named model element(s) onto
    // this element, then (re)parse — %n in the copied label resolves against this
    // element's own clone map. This is how cloned measurement blocks get their
    // value bindings from a single model definition.
    if (tagObj.attr === 'set' && tagObj.tag === '#copy_xsac_from' && tagObj.src) {
      const ids = String(tagObj.src).split(',')
      for (const id of ids) {
        const model = this.svgEl.getElementById(id.trim())
        if (model) {
          const ml = this.getLabel(model)
          if (ml) this.parseLabel(el, ml)
        }
      }
      return
    }
    // #exec / #exec_once: run the script once at load with `thisobj` = element.
    // Authored screens use this e.g. to hide detail layers by default
    // (thisobj.style.display="none";) — revealed later on zoom/interaction.
    if (
      tagObj.attr === 'set' &&
      (tagObj.tag === '#exec' || tagObj.tag === '#exec_once') &&
      tagObj.src
    ) {
      this.runScript(tagObj.src, el)
      return
    }
    // #exec_on_update: run the script on every data refresh (thisobj = element)
    if (tagObj.attr === 'set' && tagObj.tag === '#exec_on_update' && tagObj.src) {
      this.execOnUpdate.push({ el, src: tagObj.src })
      this.runScript(tagObj.src, el)
      return
    }
    // #camera / #foreign_object: replace the placeholder <rect> with a
    // <foreignObject> hosting an <iframe> (camera stream or arbitrary URL).
    if (
      tagObj.attr === 'set' &&
      (tagObj.tag === '#camera' || tagObj.tag === '#foreign_object') &&
      el.tagName === 'rect'
    ) {
      const decor =
        tagObj.prompt || 'width="100%" height="100%" frameborder="0" scrolling="no"'
      const url =
        tagObj.tag === '#camera'
          ? 'camera.html?CameraName=' + encodeURIComponent(tagObj.src || 'CAM001')
          : String(tagObj.src || '')
      const fo = document.createElementNS(SVGNS, 'foreignObject')
      for (const a of ['x', 'y', 'width', 'height', 'transform', 'id', 'style', 'class']) {
        const v = el.getAttributeNS(null, a)
        if (v != null) fo.setAttributeNS(null, a, v)
      }
      fo.innerHTML = `<iframe ${decor} src="${url}"></iframe>`
      el.parentNode.replaceChild(fo, el)
      return
    }
    // #radar (spider chart) / #arc (donut gauge)
    if (tagObj.attr === 'set' && tagObj.tag === '#radar' && tagObj.src) {
      initRadarChart(this, tagObj, el)
      return
    }
    if (tagObj.attr === 'set' && tagObj.tag === '#arc' && tagObj.src) {
      initArcChart(this, tagObj, el)
      return
    }
    // #set_filter: station filter for the (deferred) alarm box
    if (tagObj.attr === 'set' && tagObj.tag === '#set_filter' && tagObj.src) {
      this.screenFilter = tagObj.src
    }
  }

  // --- point collection / value resolution -----------------------------------
  collectPoint(tag) {
    if (tag === undefined || tag === null || tag === '') return
    const s = String(tag).trim()
    if (s.charAt(0) === '#' || s.charAt(0) === '%') return // special object / clone
    if (s.charAt(0) === '!') {
      // special code referencing a trailing point
      const m = s.match(/^!(?:SLIM|ILIM|TAG|DCR|STON|STOFF|STVAL|ALR|ALM|TMP)\s*(.+)$/)
      if (m) this.queryKeys.add(m[1].trim())
      return
    }
    this.queryKeys.add(s)
  }

  wirePopup(el, tag) {
    if (el.blockPopup || el.pontoPopup !== undefined) return
    if (tag === undefined || String(tag).charAt(0) === '#') return
    // skip dummy/constant tags (legacy: pnt !== 99999 && pnt !== 0) — these are
    // placeholder points used for static coloring, not clickable.
    const n = parseInt(tag)
    if (n === 99999 || n === 0) return
    el.style.cursor = 'pointer'
    el.addEventListener('click', (ev) => {
      ev.stopPropagation()
      const key = this.resolveKey(tag)
      if (key !== undefined) this.onOpenPoint(key)
    })
    this.registerAnnotation(el, tag)
  }

  // True if `el` or any ancestor is marked popup `notrace`/`block` — those
  // objects (and everything inside them, e.g. clone groups whose children copy
  // bindings via #copy_xsac_from) must never show a pinned tag symbol.
  isTraceBlocked(el) {
    let node = el
    while (node && node !== this.svgEl && node.nodeType === 1) {
      if (node.noTrace || node.blockPopup) return true
      node = node.parentNode
    }
    return false
  }

  // True if `el` sits INSIDE a faceplate (clone group). Such internal objects
  // don't carry their own pinned tag — the faceplate group does (registered in
  // parseLabel), so the symbol shows a single tag instead of one per element.
  insideFaceplate(el) {
    let node = el.parentNode
    while (node && node !== this.svgEl && node.nodeType === 1) {
      if (node._cloneMap) return true
      node = node.parentNode
    }
    return false
  }

  isDummyTag(tag) {
    const n = parseInt(tag)
    return n === 99999 || n === 99989 || n === 0
  }

  // Register `el` as the anchor for `tag`'s pinned-annotation badge. The badge
  // itself is created lazily the first time the point actually carries an
  // annotation or has alarms inhibited (see updateAnnotations). One per element.
  // Skipped for: popup `notrace`/`block` objects (directly or via an ancestor),
  // objects inside a faceplate clone group, and dummy points. parse() runs in
  // document order so a group's flags are set before its children are parsed.
  registerAnnotation(el, tag) {
    if (el._annotTag !== undefined || this.isTraceBlocked(el) || this.insideFaceplate(el)) return
    const s = String(tag).trim()
    if (s === '' || s.charAt(0) === '#' || s.charAt(0) === '%' || this.isDummyTag(s)) return
    el._annotTag = s
    this.annotEls.push(el)
  }

  // Remove any pinned-annotation registration/badge from `el`. Called when an
  // element is (later) marked popup `notrace`/`block` so its tag symbol never
  // appears, regardless of the order its SAGE bindings were parsed.
  unregisterAnnotation(el) {
    const i = this.annotEls.indexOf(el)
    if (i >= 0) this.annotEls.splice(i, 1)
    el._annotTag = undefined
    if (el._annotBadge) {
      if (el._annotBadge.parentNode) el._annotBadge.parentNode.removeChild(el._annotBadge)
      el._annotBadge = null
    }
  }

  // Create the (hidden) annotation badge anchored to the bottom-right of `el`.
  ensureAnnotBadge(el) {
    if (el._annotBadge) return el._annotBadge
    let bb
    try {
      bb = el._bbox || el.getBBox()
      el._bbox = bb
    } catch (e) {
      return null
    }
    let x = bb.x + bb.width
    let y = bb.y + bb.height
    if (x === 0 && y === 0) {
      x = parseFloat(el.getAttributeNS(null, 'x')) || 0
      y = parseFloat(el.getAttributeNS(null, 'y')) || 0
    }
    const xfm = el.getAttributeNS(null, 'transform') || ''
    const scale = this.cfg.ScreenViewer_SecurityCardScale || 1
    const badge = document.createElementNS(SVGNS, 'path')
    badge.setAttributeNS(null, 'd', SECURITY_CARD_PATH)
    badge.setAttributeNS(null, 'stroke-width', '1.1')
    badge.setAttributeNS(null, 'stroke-opacity', '0.9')
    badge.setAttributeNS(null, 'fill-opacity', '0.8')
    badge.setAttributeNS(null, 'cursor', 'pointer')
    badge.setAttributeNS(null, 'transform', `${xfm} translate(${x} ${y}) scale(${scale})`)
    badge.style.display = 'none'
    const title = document.createElementNS(SVGNS, 'title')
    badge.appendChild(title)
    badge.addEventListener('click', (ev) => {
      ev.stopPropagation()
      const key = this.resolveKey(el._annotTag)
      if (key !== undefined) this.onOpenPoint(key)
    })
    el.parentNode.appendChild(badge)
    el._annotBadge = badge
    return badge
  }

  // Per-refresh: show a pinned badge on every point that has an annotation
  // (yellow, tooltip = annotation text) or has alarms inhibited (gray); hide it
  // otherwise. Port of websage.js visibEtiq.
  updateAnnotations() {
    for (const el of this.annotEls) {
      const st = this.getState(el._annotTag)
      if (!st) continue
      const note = st.annotation || ''
      const inhibited = (st.flags & 0x400) !== 0
      if (note === '' && !inhibited) {
        if (el._annotBadge) {
          el._annotBadge.style.display = 'none'
          el._annotBadge.firstElementChild.textContent = ''
        }
        continue
      }
      const badge = this.ensureAnnotBadge(el)
      if (!badge) continue
      if (note !== '') {
        badge.setAttributeNS(null, 'fill', this.cfg.ScreenViewer_TagFillColor)
        badge.setAttributeNS(null, 'stroke', this.cfg.ScreenViewer_TagStrokeColor)
        badge.setAttributeNS(null, 'opacity', '0.8')
        badge.firstElementChild.textContent = note.replace(/[|^]/g, '\n')
      } else {
        badge.setAttributeNS(null, 'fill', this.cfg.ScreenViewer_TagInhAlmFillColor)
        badge.setAttributeNS(null, 'stroke', this.cfg.ScreenViewer_TagInhAlmStrokeColor)
        badge.setAttributeNS(null, 'opacity', '0.5')
        badge.firstElementChild.textContent = ''
      }
      badge.style.display = ''
    }
  }

  resolveKey(tag) {
    const t = parseInt(tag)
    if (!isNaN(t) && this.valueByKey.has(t)) return t
    const s = String(tag).trim()
    if (this.keyByTag.has(s)) return this.keyByTag.get(s)
    return isNaN(t) ? undefined : t
  }

  getState(tag) {
    const t = parseInt(tag)
    if (!isNaN(t)) return this.valueByKey.get(t)
    const s = String(tag).trim()
    if (this.keyByTag.has(s)) return this.valueByKey.get(this.keyByTag.get(s))
    return undefined
  }

  getValue(tag) {
    const st = this.getState(tag)
    return st ? st.value : 0
  }
  getFlags(tag) {
    const st = this.getState(tag)
    return st ? st.flags : undefined
  }

  // Resolve a tag/number/special-code to a value (ports valorTagueado).
  valorTagueado(tag, obj) {
    if (tag === '' || tag === undefined) return RETNOK
    const t = parseInt(tag)
    if (!isNaN(t) && String(tag).indexOf('!') !== 0) {
      const st = this.valueByKey.get(t)
      if (!st) {
        this.markInvalid(obj)
        return RETNOK
      }
      return st.value
    }
    const s = String(tag).trim()
    if (this.keyByTag.has(s)) return this.valueByKey.get(this.keyByTag.get(s)).value
    if (s.charAt(0) === '#' || s.charAt(0) === '%') return RETNOK

    // !EVAL <expr> [!END ...] : evaluate the expression (thisobj = element)
    if (s.indexOf('!EVAL') === 0) {
      const pend = s.indexOf('!END', 5)
      const expr = s.substring(5, pend === -1 ? s.length : pend)
      const v = this.evalValue(expr, obj)
      return v === undefined ? RETNOK : v
    }

    const sub = (n) => {
      let x = s.substr(n).trim()
      if (isNaN(parseInt(x))) x = this.keyByTag.get(x)
      else x = parseInt(x)
      return x
    }
    if (s.indexOf('!SLIM') === 0) {
      const st = this.valueByKey.get(sub(5))
      return st && isFinite(st.hiLimit) ? st.hiLimit : 999999
    }
    if (s.indexOf('!ILIM') === 0) {
      const st = this.valueByKey.get(sub(5))
      return st && isFinite(st.loLimit) ? st.loLimit : -999999
    }
    if (s.indexOf('!TAG') === 0) {
      const st = this.valueByKey.get(sub(4))
      return st ? st.tag : ''
    }
    if (s.indexOf('!DCR') === 0) {
      const st = this.valueByKey.get(sub(4))
      return st ? st.descr : ''
    }
    if (s.indexOf('!STON') === 0) {
      const st = this.valueByKey.get(sub(5))
      return st ? st.stateTextTrue : ''
    }
    if (s.indexOf('!STOFF') === 0) {
      const st = this.valueByKey.get(sub(6))
      return st ? st.stateTextFalse : ''
    }
    if (s.indexOf('!STVAL') === 0) {
      const st = this.valueByKey.get(sub(6))
      if (!st) return ''
      if ((st.flags & 0x03) === 0x02) return st.stateTextTrue
      if ((st.flags & 0x03) === 0x01) return st.stateTextFalse
      return ''
    }
    if (s.indexOf('!ALR') === 0) {
      const st = this.valueByKey.get(sub(4))
      return st && st.flags & 0x100 ? 1 : 0
    }
    if (s.indexOf('!ALM') === 0) {
      const st = this.valueByKey.get(sub(4))
      return st && (st.flags & 0x800 || st.flags & 0x100) ? 1 : 0
    }
    if (s.indexOf('!TMP') === 0) {
      const st = this.valueByKey.get(sub(4))
      return st ? st.alarmTime : ''
    }
    this.markInvalid(obj)
    return RETNOK
  }

  interpretaFormatoC(fmt, tag, obj) {
    const valr = this.valorTagueado(tag, obj)
    if (valr === RETNOK) return valr
    if (fmt === undefined || fmt === '') fmt = isNaN(parseFloat(valr)) ? '%s' : '%1.1f'
    const flg = this.getFlags(tag)
    if (typeof flg !== 'undefined' && flg & 0x20) {
      // directional arrow codes for analogs
      if (/[udrla]\^/.test(fmt)) {
        const v = valr
        fmt = fmt.replace('u^', String.fromCharCode(v >= 0 ? 0x2191 : 0x2193))
        fmt = fmt.replace('d^', String.fromCharCode(v >= 0 ? 0x2193 : 0x2191))
        fmt = fmt.replace('r^', String.fromCharCode(v >= 0 ? 0x21a3 : 0x21a2))
        fmt = fmt.replace('l^', String.fromCharCode(v >= 0 ? 0x21a2 : 0x21a3))
        fmt = fmt.replace('a^', '')
        return printf(fmt, Math.abs(valr))
      }
    }
    if (fmt.indexOf('%') < 0) return String(valr)
    return printf(fmt, valr)
  }

  markInvalid(obj) {
    if (obj && obj.style) obj.style.visibility = 'collapse'
  }

  // --- refresh loop ----------------------------------------------------------
  async refresh() {
    if (this.disposed || this.timeMachine) return // paused during historical replay
    try {
      const keys = [...this.queryKeys]
      if (keys.length > 0) {
        // read full properties (group1/group2/descr/stateText/limits) on the first
        // pass only, values-only after (legacy: askinfo = Pass===0)
        const full = this.pass === 0
        const points = await opc.readPoints(keys, full, this.cfg)
        this.ingest(points, full)
      }
      // after first data load, prefill live-plot histories (needs key->tag map)
      if (!this._plotsFilled && this.plotBindings.length > 0) {
        this._plotsFilled = true
        this.fillAllPlots()
      }
      // beep status
      const beep = await opc.readPoints([opc.BEEP_POINTKEY], true, this.cfg)
      this.onAlarmBeep(beep.length && beep[0].valueRaw ? 1 : 0)
      this.applyBindings()
      this.updateAnnotations()
      this.refreshAlarmBox() // fire-and-forget; current alarms for the #set_filter group1
      this.pass++
      this.onStatus('')
    } catch (e) {
      this.onStatus('Server error')
    }
    this.refreshTimer = setTimeout(
      () => this.refresh(),
      this.cfg.ScreenViewer_RefreshTime * 1000
    )
  }

  // Refresh the embedded alarm box with the current alarms of the screen's
  // group1 filter (#set_filter). No filter -> no box.
  async refreshAlarmBox() {
    if (this.disposed || this.timeMachine || !this.screenFilter) return
    if (this._alarmBusy) return
    this._alarmBusy = true
    try {
      const pts = await opc.readFiltered(
        { station: this.screenFilter, onlyAlarms: true },
        this.cfg
      )
      pts.sort((a, b) => (b.priority || 0) - (a.priority || 0) || (b.key || 0) - (a.key || 0))
      const items = pts.map((p) => {
        let d = p.descr || ''
        if (p.bay && d.indexOf(p.bay + ' | ') === 0) d = d.substring((p.bay + ' | ').length)
        return {
          key: p.key,
          sub: p.station || '',
          bay: p.bay || '',
          descr: d,
          valueStr: p.valueStr || '',
          alarmTime: p.alarmTime || '',
        }
      })
      this.alarmBox.render(items)
    } catch (e) {
      /* keep last list */
    } finally {
      this._alarmBusy = false
    }
  }

  // --- Time Machine (historical replay) --------------------------------------
  // Enter pauses realtime polling so historical snapshots aren't overwritten.
  enterTimeMachine() {
    this.timeMachine = true
    clearTimeout(this.refreshTimer)
    this.refreshTimer = null
    // historical replay shows past values; live-only overlays don't apply
    this.preview.hide()
    this.alarmBox.hide() // alarms are realtime — hidden during replay (legacy)
  }

  // Exit resumes realtime polling (next refresh reloads live values).
  exitTimeMachine() {
    if (!this.timeMachine) return
    this.timeMachine = false
    this.tmTime = null
    this.refresh()
  }

  // Render the screen as it was at instant `date`: snapshot every screen point
  // from the historian and re-apply. Points without a sample show as failed.
  async gotoTime(date) {
    if (this.disposed) return
    this.timeMachine = true
    this.tmTime = date
    const tags = []
    this.valueByKey.forEach((st) => {
      if (st.tag) tags.push(st.tag)
    })
    if (tags.length === 0) return
    // blank to failed first; snapshot then overwrites points that had a sample
    this.valueByKey.forEach((st) => {
      st.value = 0
      st.flags |= 0x83
    })
    try {
      const snap = await opc.readHistorySnapshot(tags, date)
      if (this.tmTime !== date) return // a newer request superseded this one
      for (const s of snap) {
        const key = this.keyByTag.get(s.tag)
        if (key === undefined) continue
        const st = this.valueByKey.get(key)
        if (!st) continue
        const stale = date.getTime() - s.serverTime > 3600 * 1000
        const badQ = (s.quality & 0x80000000) !== 0
        const qflag = stale || badQ ? 0x80 : 0
        if (s.type === OpcValueTypes.Boolean) {
          st.value = s.value ? 0 : 1 // OSHMI: on(true)=0, off(false)=1
          st.flags = (s.value ? 0x02 : 0x01) | qflag
          st.valueStr = st.value === 0 ? st.stateTextTrue || '' : st.stateTextFalse || ''
        } else {
          const num = typeof s.value === 'number' ? s.value : parseFloat(s.value)
          st.value = isNaN(num) ? 0 : num
          st.flags = 0x20 | qflag
          st.valueStr = String(s.value)
        }
      }
      this.onStatus('')
    } catch (e) {
      this.onStatus('Historian error')
    }
    this.applyBindings()
    this.updateAnnotations()
  }

  // Map normalized points into display-semantics value state (V/F arrays).
  // `full` (first pass) carries static props (group/descr/stateText/limits);
  // later value-only passes preserve them.
  ingest(points, full) {
    points.forEach((p) => {
      let value
      let flags = 0
      if (p.type === 'digital') {
        value = p.valueRaw ? 0 : 1 // OSHMI: on(true)=0, off(false)=1
        flags = p.valueRaw ? 0x02 : 0x01
      } else if (p.type === 'analog') {
        value = parseFloat(p.valueRaw)
        flags = 0x20
      } else {
        value = parseFloat(p.valueRaw)
        flags = 0
      }
      if (p.quality !== OpcStatusCodes.Good) flags |= 0x80
      if (p.alarmed) flags |= 0x100
      if (p.alarmDisabled) flags |= 0x400
      if (p.annotation) flags |= 0x200
      if (p.manual) flags |= 0x0c
      if (p.frozen) flags |= 0x1000
      if (p.flags & 0x800) flags |= 0x800
      const prev = this.valueByKey.get(p.key)
      if (prev && !full) {
        // value-only update: keep static fields, refresh dynamic ones. The
        // annotation/alarm-inhibited bits only come back on the full (askInfo)
        // read, so preserve them from the last full pass.
        prev.value = value
        prev.flags = flags | (prev.flags & (0x200 | 0x400))
      } else {
        this.valueByKey.set(p.key, {
          value,
          valueStr: p.valueStr,
          flags,
          stateTextTrue: p.stateTextTrue,
          stateTextFalse: p.stateTextFalse,
          hiLimit: p.hiLimit,
          loLimit: p.loLimit,
          alarmTime: p.alarmTime,
          tag: p.tag,
          descr: p.descr,
          bay: p.bay,
          station: p.station,
          annotation: p.annotation || '',
        })
      }
      this.keyByTag.set(p.tag, p.key)
    })
  }

  // Re-read a single point's FULL properties (askInfo) and re-apply. Used after
  // the point dialog writes an annotation / alarm-inhibit / limits so the change
  // (e.g. the pinned-annotation badge) shows immediately, without waiting for a
  // screen reload — annotation/limits only come back on a full read. A short
  // delay + one retry covers the write's propagation into the realtime store.
  async refreshPoint(key) {
    if (this.disposed || key === undefined || key === null) return
    for (let attempt = 0; attempt < 2 && !this.disposed; attempt++) {
      await new Promise((r) => setTimeout(r, attempt === 0 ? 400 : 1200))
      try {
        const points = await opc.readPoints([key], true, this.cfg)
        if (points.length) {
          this.ingest(points, true)
          this.applyBindings()
          this.updateAnnotations()
          if (points[0].annotation || points[0].alarmDisabled) return // change landed
        }
      } catch (e) {
        /* retry */
      }
    }
  }

  applyBindings() {
    this.blinkList = []
    for (const b of this.bindings) {
      const el = b.parent
      if (!el) continue
      let vt = RETNOK
      if (b.tag !== undefined) vt = this.valorTagueado(b.tag, el)
      if (vt === RETNOK && b.attr !== 'color' && b.attr !== 'set' && b.attr !== 'script') continue
      try {
        this.applyOne(b, el, vt)
      } catch (e) {
        /* keep animating other elements */
      }
    }
    // per-refresh screen scripts and live !EVAL tooltips
    for (const e of this.execOnUpdate) this.runScript(e.src, e.el)
    this.updateDynamicTooltips()
  }

  applyOne(b, el, vt) {
    switch (b.attr) {
      case 'get':
        this.applyGet(b, el)
        break
      case 'color':
        this.applyColor(b, el)
        break
      case 'bar': {
        let h = (b.initheight * (vt - b.min)) / (b.max - b.min)
        if (h < 0) h = 0
        if (h > b.initheight) h = b.initheight
        el.setAttributeNS(null, 'height', h)
        this.blinkIfAlarmed(b.tag, el)
        break
      }
      case 'opac':
        el.style.opacity = (vt - b.min) / (b.max - b.min)
        this.blinkIfAlarmed(b.tag, el)
        break
      case 'rotate': {
        const bb = el.getBBox()
        const tcx =
          parseFloat(el.getAttributeNS(INKNS, 'transform-center-x') || el.getAttributeNS(null, 'inkscape:transform-center-x')) || 0
        const tcy =
          parseFloat(el.getAttributeNS(INKNS, 'transform-center-y') || el.getAttributeNS(null, 'inkscape:transform-center-y')) || 0
        const xcen = bb.x + bb.width / 2 + tcx
        const ycen = bb.y + bb.height / 2 - tcy
        const ang = ((vt - b.min) / (b.max - b.min)) * 360
        el.setAttributeNS(null, 'transform', `${b.inittransform} rotate(${ang} ${xcen} ${ycen}) `)
        this.blinkIfAlarmed(b.tag, el)
        break
      }
      case 'slider': {
        let v = vt
        if (v > b.max) v = b.max
        if (v < b.min) v = b.min
        const prop = (v - b.min) / (b.max - b.min)
        el.setAttributeNS(null, 'transform', `${b.inittransform} translate(${prop * b.rangex} ${prop * b.rangey}) `)
        this.blinkIfAlarmed(b.tag, el)
        break
      }
      case 'text':
        this.applyTextMap(b, el, vt)
        break
      case 'open':
        if (b.istag && b.poly) this.updatePlot(b, el, vt)
        break
      case 'script':
        // vega chart: re-render with current data each refresh
        if (b._vega) updateVegaChart(this, b)
        break
      case 'set':
        if (b.tag === '#radar') {
          drawRadar(this, b, el)
          break
        }
        if (b.tag === '#arc') {
          drawArc(this, b, el)
          break
        }
        // #exec_on_update: re-run the script every refresh
        if (b.tag === '#exec_on_update' && b.src) {
          try {
            // eslint-disable-next-line no-new-func
            new Function('thisobj', 'SVGDoc', b.src)(el, this.svgEl)
          } catch (e) {
            /* keep animating */
          }
        }
        break
      default:
        break
    }
  }

  // --- live trend plots ------------------------------------------------------
  fillAllPlots() {
    this.plotBindings.forEach((b, i) => {
      // stagger the historian queries slightly
      setTimeout(() => this.fillPlotHistory(b), 200 * i)
    })
  }

  async fillPlotHistory(b) {
    let tag = b.tag
    const k = parseInt(tag)
    if (!isNaN(k) && this.valueByKey.has(k)) tag = this.valueByKey.get(k).tag
    try {
      const begin = new Date(Date.now() - b.windowSec * 1000)
      const hist = await opc.readHistory(tag, begin, new Date())
      b.vals = hist.map((h) => h.value)
      b.times = hist.map((h) => h.time)
      b.histLoaded = true
    } catch (e) {
      /* leave empty; realtime samples will accumulate */
    }
  }

  updatePlot(b, el, vt) {
    const now = Date.now()
    b.vals.push(vt)
    b.times.push(now)
    let bb
    try {
      bb = el.getBBox()
    } catch (e) {
      return
    }
    const right = bb.x + bb.width
    const top = bb.y
    const bottom = bb.y + bb.height
    const win = b.windowSec * 1000
    let pts = ''
    for (let k = b.vals.length - 1; k >= 0; k--) {
      const age = now - b.times[k]
      if (age > win) {
        b.vals.splice(k, 1)
        b.times.splice(k, 1)
        continue
      }
      const xx = right - (age / win) * bb.width
      let yy = bottom - ((b.vals[k] - b.plotValMin) / b.plotValSpan) * bb.height
      if (yy > bottom) yy = bottom
      if (yy < top) yy = top
      pts += xx.toFixed(2) + ' ' + yy.toFixed(2) + (k === 0 ? '' : ',')
    }
    b.poly.setAttribute('points', pts)
  }

  applyGet(b, el) {
    let val
    if (b.txtOFFON !== undefined) {
      val = this.valorTagueado(b.tag, el) === 0 ? b.txtOFFON[1] : b.txtOFFON[0]
    } else {
      val = this.interpretaFormatoC(b.formatoC, b.tag, el)
    }
    if (val === RETNOK) return
    if (val !== el.textContent) {
      if (el.firstElementChild && el.firstElementChild.nodeName === 'tspan')
        el.firstElementChild.textContent = val
      else el.textContent = val
    }
    this.blinkIfAlarmed(b.tag, el)
  }

  applyTextMap(b, el, vt) {
    if (!Array.isArray(b.map)) return
    const ft = this.getFlags(b.tag)
    const digital = (ft & 0x20) === 0
    let txt = ''
    for (const entry of b.map) {
      const poseq = entry.indexOf('=')
      const ch = entry.substring(0, 1)
      const thr = entry.substring(0, poseq)
      const rhs = entry.substring(poseq + 1)
      if (digital) {
        const val = parseInt(thr)
        if ((ft & 0x03) >= val || (ch === 'a' && ft & 0x100) || (ch === 'f' && ft & 0x80)) txt = rhs
      } else {
        const val = parseFloat(thr)
        if (vt >= val || (ch === 'a' && ft & 0x100) || (ch === 'f' && ft & 0x80)) txt = rhs
      }
    }
    if (txt !== el.textContent) {
      if (el.firstChild && el.firstChild.tagName === 'tspan') el.firstChild.textContent = txt
      else el.textContent = txt
    }
    this.blinkIfAlarmed(b.tag, el)
  }

  applyColor(b, el) {
    let fill = ''
    let stroke = ''
    let attrib = ''
    let attribval = ''
    let script = ''
    let tag = ''
    let vt = 0
    if (!Array.isArray(b.list)) return
    for (let j = 0; j < b.list.length; j++) {
      const entry = b.list[j]
      if (tag !== entry.tag) {
        tag = entry.tag
        vt = this.valorTagueado(tag, el)
      }
      let ft = this.getFlags(tag)
      if (ft === undefined) ft = vt === RETNOK ? 0x80 | 0x20 : 0x20
      const digital = (ft & 0x20) === 0
      if (vt === RETNOK) continue
      const ch = entry.data
      if (digital) {
        const val = parseInt(ch)
        if (
          (!isNaN(val) && (ft & 0x03) >= val) ||
          (!isNaN(val) && (ft & 0x83) >= (val | 0x80)) ||
          (ch === 'a' && ft & 0x100) ||
          (ch === 'f' && ft & 0x80)
        ) {
          fill = entry.cfill
          stroke = entry.cstroke
          script = entry.cscript
          attrib = entry.cattrib
          attribval = entry.cattribval
        }
      } else {
        const val = parseFloat(ch)
        if (
          (ch === 'n' && ft & 0x800) ||
          (ch === 'c' && ft & 0x1000) ||
          (ch === 'a' && ft & 0x100) ||
          (ch === 'f' && ft & 0x80) ||
          (!isNaN(val) && vt >= val)
        ) {
          const next = b.list[j + 1]
          if (next) {
            fill = this.interpColor(entry.cfill, next.cfill, vt, val, parseFloat(next.data))
            stroke = this.interpColor(entry.cstroke, next.cstroke, vt, val, parseFloat(next.data))
          } else {
            fill = entry.cfill.replace(/^@/, '')
            stroke = entry.cstroke.replace(/^@/, '')
          }
          script = entry.cscript
          attrib = entry.cattrib
          attribval = entry.cattribval
        }
      }
      this.blinkIfAlarmed(tag, el)
    }

    if (attrib !== '') {
      el.setAttributeNS(null, attrib, attribval)
    } else if (script !== '') {
      // per-color inline script: reset to authored colors, then run the snippet
      // (e.g. $W.Animate(thisobj, ...)) with thisobj = element.
      if (el.style) {
        el.style.fill = b.initfill
        el.style.stroke = b.initstroke
      }
      this.runScript(script, el)
    } else if (el.style) {
      el.style.fill = fill !== '' ? fill : b.initfill
      el.style.stroke = stroke !== '' ? stroke : b.initstroke
    }
  }

  interpColor(curr, next, vt, val, proxval) {
    if (next && next[0] === '@') {
      const a = curr[0] === '@' ? curr.substring(1) : curr
      const b = next.substring(1)
      const t = (vt - val) / (proxval - val)
      return rgbMix(a, b, t)
    }
    return curr.replace(/^@/, '')
  }

  // --- blink -----------------------------------------------------------------
  blinkIfAlarmed(tag, el) {
    const f = this.getFlags(tag)
    if (f !== undefined && f & 0x100 && !(f & 0x400)) this.blinkList.push(el)
  }
  startBlink() {
    this.blinkTimer = setInterval(() => {
      this.blinkOn = !this.blinkOn
      this.blinkList.forEach((el) => el.setAttributeNS(null, 'opacity', this.blinkOn ? 1 : 0.25))
    }, 500)
  }

  // --- zoom / pan ------------------------------------------------------------
  toSvgPoint(clientX, clientY, ctm) {
    const pt = this.svgEl.createSVGPoint()
    pt.x = clientX
    pt.y = clientY
    return pt.matrixTransform(ctm || this.svgEl.getScreenCTM().inverse())
  }
  applyViewBox() {
    if (!this.svgEl) return
    const z = this.zoom
    this.svgEl.setAttribute('viewBox', `${z.x} ${z.y} ${z.w} ${z.h}`)
  }
  zoomPan(op, mul = 1) {
    const z = this.zoom
    const W = this.cfg.ScreenViewer_SVGMaxWidth
    switch (op) {
      case 'in':
        z.w *= 0.9
        z.h *= 0.9
        break
      case 'out':
        z.w *= 1.1
        z.h *= 1.1
        break
      case 'up':
        z.y += (mul * 20 * z.w) / W
        break
      case 'down':
        z.y -= (mul * 20 * z.w) / W
        break
      case 'left':
        z.x += (mul * 30 * z.w) / W
        break
      case 'right':
        z.x -= (mul * 30 * z.w) / W
        break
      case 'center':
        this.zoom = this.fitZoom ? { ...this.fitZoom } : { x: 0, y: 0, w: this.cfg.ScreenViewer_SVGMaxWidth, h: this.cfg.ScreenViewer_SVGMaxHeight }
        this.applyViewBox()
        return
      default:
        break
    }
    this.applyViewBox()
  }
  wheelZoom(ev) {
    const ctm = this.svgEl.getScreenCTM().inverse()
    const p = this.toSvgPoint(ev.clientX, ev.clientY, ctm)
    const z = this.zoom
    const factor = ev.deltaY < 0 ? 0.95 : 1.05
    const w = z.w
    const h = z.h
    z.w *= factor
    z.h *= factor
    z.x += (w - z.w) * ((p.x - z.x) / w)
    z.y += (h - z.h) * ((p.y - z.y) / h)
    this.applyViewBox()
  }

  setBackground(color) {
    if (this.svgEl) this.svgEl.style.backgroundColor = color
    if (this.container) this.container.style.backgroundColor = color
  }

  // --- lifecycle -------------------------------------------------------------
  stop() {
    clearTimeout(this.refreshTimer)
    clearInterval(this.blinkTimer)
    clearInterval(this.pinnedTimer)
    this.refreshTimer = null
    this.blinkTimer = null
    this.pinnedTimer = null
  }
  dispose() {
    this.disposed = true
    this.stop()
    this.preview.dispose()
    this.pinnedPanel.dispose()
    this.alarmBox.dispose()
    if (this.container) this.container.innerHTML = ''
  }
}
