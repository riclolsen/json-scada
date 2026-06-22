/*
 * Hover preview overlay for the Display Viewer — port of websage.js setPreview().
 *
 * A single floating panel (an <iframe> in a positioned <div>) is shown after a
 * short hover delay when the pointer rests on an element that declares a preview:
 *   - a linked screen (open -> another screen): previews that screen
 *   - a preview:URL source on a popup/open binding: previews that URL
 *   - an analog reading (get): previews the point's trend chart
 * The panel pops in the screen corner opposite the mouse so it never sits under
 * the cursor, and hides on mouse-out of either the source element or the panel.
 *
 * {json:scada} - Copyright 2020-2026 - Ricardo L. Olsen
 */

export class PreviewOverlay {
  constructor(cfg) {
    this.cfg = cfg
    this.timeoutMs = cfg.ScreenViewer_DisplayPreviewTimeout || 1500
    this.timer = 0
    this.div = null
    this.iframe = null
  }

  ensure() {
    if (this.div) return
    const div = document.createElement('div')
    Object.assign(div.style, {
      position: 'fixed',
      display: 'none',
      zIndex: '9999',
      background: '#ffffff',
      border: '2px solid #888888',
      boxShadow: '0 4px 16px rgba(0,0,0,0.4)',
      borderRadius: '4px',
      overflow: 'hidden',
      lineHeight: '0',
    })
    const iframe = document.createElement('iframe')
    iframe.setAttribute('scrolling', 'no')
    Object.assign(iframe.style, { border: 'none', display: 'block' })
    div.appendChild(iframe)
    // hide if the pointer leaves the panel itself
    div.addEventListener('mouseout', () => this.hide())
    document.body.appendChild(div)
    this.div = div
    this.iframe = iframe
  }

  // Attach hover-preview behavior to `el`. `urlOrFn` is the preview URL, or a
  // function returning it (resolved at hover time, e.g. to look up a point key).
  attach(el, urlOrFn, width, height) {
    this.ensure()
    const w = parseInt(width) || this.cfg.ScreenViewer_DefaultDisplayPreviewWidth || 700
    const h = parseInt(height) || this.cfg.ScreenViewer_DefaultDisplayPreviewHeight || 480
    el.addEventListener('mouseover', (evt) => {
      clearTimeout(this.timer)
      const innerW = window.innerWidth
      const innerH = window.innerHeight
      // don't show if the window is too small to fit the panel comfortably
      if (innerW < 100 + w || innerH < 100 + h) return
      const url = typeof urlOrFn === 'function' ? urlOrFn() : urlOrFn
      if (!url) return
      // pop in the corner opposite the mouse
      const left = evt.clientX > innerW / 2
      const top = evt.clientY > innerH / 2
      this.div.style.left = left ? '5px' : ''
      this.div.style.right = left ? '' : '5px'
      this.div.style.top = top ? '5px' : ''
      this.div.style.bottom = top ? '' : '5px'
      this.iframe.width = w
      this.iframe.height = h
      this.timer = setTimeout(() => {
        this.iframe.src = url
        this.div.style.display = ''
      }, this.timeoutMs)
    })
    el.addEventListener('mouseout', () => this.hide())
  }

  hide() {
    clearTimeout(this.timer)
    if (this.div) {
      this.div.style.display = 'none'
      if (this.iframe) this.iframe.src = ''
    }
  }

  dispose() {
    this.hide()
    if (this.div && this.div.parentNode) this.div.parentNode.removeChild(this.div)
    this.div = null
    this.iframe = null
  }
}
