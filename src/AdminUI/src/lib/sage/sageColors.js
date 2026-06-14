/*
 * SAGE color palette resolution for the native Display Viewer.
 * Ports the `-cor-NN` ColorTable and TraduzCor() from the legacy
 * config_viewers_default.js / websage.js.
 *
 * {json:scada} - Copyright 2020-2026 - Ricardo L. Olsen
 */

// Build the 60-entry user color palette. Several entries reference other config
// values (bar/breaker/switch color, background), so it is derived from cfg.
export function buildColorTable(cfg) {
  const bs = cfg.ScreenViewer_BarBreakerSwColor
  const bg = cfg.ScreenViewer_Background
  const t = new Array(60)
  t[0] = 'white'
  t[1] = 'white'
  t[2] = 'white'
  t[3] = bg
  t[4] = bs
  t[5] = bs
  t[6] = 'cornsilk'
  t[7] = 'cornsilk'
  t[8] = bs
  t[9] = '#AAAAAA'
  t[10] = '#6199c7'
  t[11] = 'red'
  t[12] = 'white'
  t[13] = bs
  t[14] = bs
  t[15] = '#777777'
  t[16] = bs
  t[17] = bs
  t[18] = bs
  t[19] = bs
  t[20] = '#6199c7'
  t[21] = '#6199c7'
  t[22] = '#6199c7'
  t[23] = '#6199c7'
  t[24] = '#6199c7'
  t[25] = '#6199c7'
  t[26] = 'gray'
  t[27] = 'gray'
  t[28] = 'darkgreen'
  t[29] = 'darkred'
  t[30] = 'red'
  t[31] = 'lightgray'
  t[32] = 'black'
  t[33] = '#D7D7D7'
  t[34] = 'gray'
  t[35] = bs
  t[36] = 'mediumvioletred'
  t[37] = 'red'
  t[38] = '#999999'
  t[39] = '#DDE8DD'
  t[40] = 'yellow'
  t[41] = 'deepskyblue'
  t[42] = 'red'
  t[43] = '#6199c7'
  t[44] = 'red'
  t[45] = 'yellow'
  t[46] = 'orange'
  t[47] = 'fuchsia'
  t[48] = '#CCCCCC'
  t[49] = '#CCCCCC'
  t[50] = '#505050'
  t[51] = 'lightsteelblue'
  t[52] = 'tan'
  t[53] = '#888888'
  t[54] = 'red'
  t[55] = 'lightsteelblue'
  t[56] = '#777777'
  t[57] = bs
  t[58] = 'crimson'
  t[59] = '#CCCCCC'
  return t
}

// Translate a color shortcut. `-cor-NN` -> ColorTable[NN]; a leading `@` marks
// an interpolation anchor (handled by the caller); otherwise pass through.
export function traduzCor(name, colorTable) {
  if (name == null) return ''
  let s = String(name).trim()
  let prefix = ''
  if (s.charAt(0) === '@') {
    prefix = '@'
    s = s.substring(1)
  }
  const m = s.match(/^-cor-(\d+)$/)
  if (m) {
    const idx = parseInt(m[1])
    return prefix + (colorTable[idx] !== undefined ? colorTable[idx] : '')
  }
  return prefix + s
}
