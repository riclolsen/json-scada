/*
 * Pinned documental-annotations panel for the Display Viewer — port of
 * websage.js pinnedAnnotations()/getPinnedAnnotations().
 *
 * When a screen carries a `#set_filter` markup (a group1/substation filter),
 * this panel lists the documental annotations (the point `notes` field) of
 * points in that group1 whose text contains "#PIN". It floats at the bottom-
 * right of the display; clicking it collapses to a count chip and back.
 *
 * {json:scada} - Copyright 2020-2026 - Ricardo L. Olsen
 */

function escapeHtml(s) {
  return String(s == null ? '' : s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
}

export class PinnedAnnotationsPanel {
  constructor(cfg) {
    this.cfg = cfg
    this.outer = null
    this.inner = null
    this.collapsed = false
    this._html = ''
  }

  ensure() {
    if (this.outer) return
    const outer = document.createElement('div')
    Object.assign(outer.style, {
      position: 'fixed',
      right: '5px',
      bottom: '5px',
      maxHeight: '80vh',
      overflowY: 'auto',
      zIndex: '8',
      display: 'none',
      cursor: 'pointer',
    })
    const inner = document.createElement('div')
    Object.assign(inner.style, {
      display: 'none',
      justifyContent: 'flex-end',
      flexFlow: 'column',
      whiteSpace: 'pre-line',
      fontFamily: this.cfg.ScreenViewer_PinnedAnnotationsFont || 'tahoma',
      fontSize: this.cfg.ScreenViewer_PinnedAnnotationsFontSize || '12px',
      padding: '5px',
      width: this.cfg.ScreenViewer_PinnedAnnotationsWidth || '300px',
    })
    outer.appendChild(inner)
    // click toggles collapsed (count-chip only) <-> expanded (full list)
    outer.addEventListener('click', () => this.toggle())
    document.body.appendChild(outer)
    this.outer = outer
    this.inner = inner
  }

  rowStyle() {
    return (
      'flex:1;flex-shrink:1;flex-grow:0;white-space:pre-line;margin:2px;font-family:' +
      (this.cfg.ScreenViewer_PinnedAnnotationsFont || 'tahoma') +
      ';font-size:' +
      (this.cfg.ScreenViewer_PinnedAnnotationsFontSize || '12px') +
      ';background-color:' +
      (this.cfg.ScreenViewer_PinnedAnnotationsBGColor || 'slategray') +
      ';color:' +
      (this.cfg.ScreenViewer_PinnedAnnotationsTextColor || 'white') +
      ';border:' +
      (this.cfg.ScreenViewer_PinnedAnnotationsBorder || '1px solid black') +
      ';box-shadow:2px 2px 4px gray;padding:4px;position:relative;'
    )
  }

  // items: [{ sub, bay, descr, note }]
  render(items) {
    this.ensure()
    if (!items || items.length === 0) {
      this.outer.style.display = 'none'
      this.inner.style.display = 'none'
      return
    }
    const st = this.rowStyle()
    let html =
      `<div class="sage-pin-count" style="text-align:center;${st};border-radius:12px;">` +
      items.length +
      '</div>'
    for (const it of items) {
      const text = `:: ${it.sub} | ${it.bay} | ${it.descr}\n${it.note}\n`
      html += `<div style="${st};border-radius:5px;">${escapeHtml(text)}</div>`
    }
    if (this._html !== html) {
      this.inner.innerHTML = html
      this._html = html
    }
    this.outer.style.display = ''
    this.inner.style.display = 'flex'
    this.applyCollapsed()
  }

  toggle() {
    this.collapsed = !this.collapsed
    this.applyCollapsed()
  }

  applyCollapsed() {
    if (!this.outer) return
    if (this.collapsed) {
      this.outer.style.maxHeight = '35px'
      this.outer.style.overflowY = 'hidden'
      this.inner.style.width = '28px'
    } else {
      this.outer.style.maxHeight = '80vh'
      this.outer.style.overflowY = 'auto'
      this.inner.style.width = this.cfg.ScreenViewer_PinnedAnnotationsWidth || '300px'
    }
    // the count chip shows only when collapsed
    const c = this.inner.querySelector('.sage-pin-count')
    if (c) c.style.display = this.collapsed ? '' : 'none'
  }

  hide() {
    this._html = ''
    if (this.outer) {
      this.outer.style.display = 'none'
      this.inner.style.display = 'none'
      this.inner.innerHTML = ''
    }
  }

  dispose() {
    if (this.outer && this.outer.parentNode) this.outer.parentNode.removeChild(this.outer)
    this.outer = null
    this.inner = null
  }
}
