/*
 * Embedded alarm box for the Display Viewer — native port of the legacy
 * almbox.html iframe (loaded via the screen's #set_filter group1 markup).
 *
 * Lists the substation's currently-alarmed points in a compact box at the
 * top-right corner of the Display Viewer page (placed in the page, never on
 * other pages), over the viewer toolbar's right end,
 * that expands on hover (legacy: ~40px collapsed -> ~105px on mouseover) and is
 * scrollable. Rows are clickable to open point access. The engine feeds it the
 * alarmed points (opc.readFiltered({station, onlyAlarms:true})) each refresh.
 *
 * {json:scada} - Copyright 2020-2026 - Ricardo L. Olsen
 */

function escapeHtml(s) {
  return String(s == null ? '' : s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
}

const COLLAPSED_PX = 42 // ~3 rows
const EXPANDED_PX = 150

export class AlarmBoxPanel {
  // host: element the box is placed in (the viewer page, so it never shows on
  // other pages); without one it is fixed to the window.
  constructor(cfg, onOpenPoint, host) {
    this.cfg = cfg
    this.host = host || null
    this.disposed = false
    this.onOpenPoint = onOpenPoint || (() => {})
    this.box = null
    this.list = null
    this._html = ''
  }

  ensure() {
    if (this.box || this.disposed) return
    const box = document.createElement('div')
    Object.assign(box.style, {
      position: this.host ? 'absolute' : 'fixed',
      top: '0px',
      right: '0px',
      zIndex: '1010', // over the viewer toolbar and the SVG
      width: '372px',
      maxHeight: COLLAPSED_PX + 'px',
      overflowX: 'hidden',
      overflowY: 'auto',
      display: 'none',
      fontFamily: 'tahoma, sans-serif',
      fontSize: '11px',
      boxShadow: '2px 2px 4px #888',
      background: this.cfg.ScreenViewer_AlmBoxGridColor || 'whitesmoke',
      transition: 'max-height 0.15s ease',
    })
    // expand on hover, collapse on leave (legacy almbox grow/shrink)
    box.addEventListener('mouseenter', () => {
      box.style.maxHeight = EXPANDED_PX + 'px'
    })
    box.addEventListener('mouseleave', () => {
      box.style.maxHeight = COLLAPSED_PX + 'px'
    })
    const list = document.createElement('div')
    box.appendChild(list)
    ;(this.host || document.body).appendChild(box)
    this.box = box
    this.list = list
  }

  // items: [{ key, sub, bay, descr, valueStr, qualifier, alarmTime }]
  render(items) {
    this.ensure()
    if (!this.box) return // disposed: a late refresh must not bring it back
    if (!items || items.length === 0) {
      this.box.style.display = 'none'
      this._html = ''
      return
    }
    const tableBg = this.cfg.ScreenViewer_AlmBoxTableColor || '#DCDCEE'
    let html = ''
    for (const it of items) {
      // box is already filtered to one substation, so omit the sub prefix (legacy)
      const line = it.bay ? `${it.bay}~${it.descr}` : it.descr
      html +=
        `<div class="sage-alm-row" data-key="${it.key}" style="display:flex;justify-content:space-between;gap:6px;` +
        `padding:1px 5px;border-bottom:1px solid ${this.cfg.ScreenViewer_AlmBoxGridColor || 'whitesmoke'};` +
        `background:${tableBg};cursor:pointer;white-space:nowrap;overflow:hidden;">` +
        `<span style="overflow:hidden;text-overflow:ellipsis;color:#222">${escapeHtml(line)}</span>` +
        `<span style="color:#c00;font-weight:bold;flex:0 0 auto">${escapeHtml(it.valueStr || '')}</span>` +
        `</div>`
    }
    if (this._html !== html) {
      this.list.innerHTML = html
      this._html = html
      this.list.querySelectorAll('.sage-alm-row').forEach((r) => {
        r.addEventListener('click', () => {
          const k = parseInt(r.getAttribute('data-key'))
          if (!isNaN(k)) this.onOpenPoint(k)
        })
      })
    }
    this.box.style.display = ''
  }

  hide() {
    this._html = ''
    if (this.box) {
      this.box.style.display = 'none'
      this.list.innerHTML = ''
    }
  }

  dispose() {
    this.disposed = true
    if (this.box && this.box.parentNode) this.box.parentNode.removeChild(this.box)
    this.box = null
    this.list = null
  }
}
