/*
 * printf-style formatting for the native Display Viewer.
 * Compact reimplementation of the C printf used by websage.js interpretaFormatoC.
 * Supports %[flags][width][.precision][diouxXeEfgGsc%].
 *
 * {json:scada} - Copyright 2020-2026 - Ricardo L. Olsen
 */

export function printf(fmt, ...args) {
  let i = 0
  return String(fmt).replace(
    /%([-+ 0#]*)(\d+)?(?:\.(\d+))?([diouxXeEfgGsc%])/g,
    (match, flags, width, prec, conv) => {
      if (conv === '%') return '%'
      const arg = args[i++]
      let s
      const num = Number(arg)
      switch (conv) {
        case 'd':
        case 'i':
          s = String(Math.trunc(isNaN(num) ? 0 : num))
          break
        case 'u':
          s = String(Math.abs(Math.trunc(isNaN(num) ? 0 : num)))
          break
        case 'f':
          s = (isNaN(num) ? 0 : num).toFixed(prec === undefined ? 6 : +prec)
          break
        case 'e':
        case 'E': {
          s = (isNaN(num) ? 0 : num).toExponential(prec === undefined ? 6 : +prec)
          if (conv === 'E') s = s.toUpperCase()
          break
        }
        case 'x':
          s = Math.trunc(isNaN(num) ? 0 : num).toString(16)
          break
        case 'X':
          s = Math.trunc(isNaN(num) ? 0 : num).toString(16).toUpperCase()
          break
        case 'o':
          s = Math.trunc(isNaN(num) ? 0 : num).toString(8)
          break
        case 'g':
        case 'G':
          s = String(isNaN(num) ? arg : num)
          break
        case 's':
          s = arg === undefined || arg === null ? '' : String(arg)
          if (prec !== undefined) s = s.slice(0, +prec)
          break
        case 'c':
          s = String(arg)
          break
        default:
          s = String(arg)
      }

      // sign for numeric conversions
      if ((conv === 'd' || conv === 'i' || conv === 'f' || conv === 'e' || conv === 'E') && num >= 0) {
        if (flags.indexOf('+') >= 0) s = '+' + s
        else if (flags.indexOf(' ') >= 0) s = ' ' + s
      }

      // width / padding
      if (width) {
        const w = +width
        if (s.length < w) {
          if (flags.indexOf('-') >= 0) {
            s = s + ' '.repeat(w - s.length)
          } else if (flags.indexOf('0') >= 0 && (s[0] === '-' || s[0] === '+' || s[0] === ' ')) {
            s = s[0] + '0'.repeat(w - s.length) + s.slice(1)
          } else {
            const pad = flags.indexOf('0') >= 0 ? '0' : ' '
            s = pad.repeat(w - s.length) + s
          }
        }
      }
      return s
    }
  )
}
