/*
 * Loads the deployment's SVG screen list for the Display Viewer.
 * The generated public/svg/screen_list.js assigns an `optionhtml` string of
 * <optgroup>/<option value=...> markup. We evaluate it in a sandbox, then parse
 * the resulting HTML into {value,title,group} items. Degrades gracefully if absent.
 *
 * {json:scada} - Copyright 2020-2026 - Ricardo L. Olsen
 */

export async function loadScreenList() {
  try {
    const text = await (await fetch('/svg/screen_list.js')).text()
    const body =
      'var optionhtml="";var optval=[],opttxt=[],optgroup=[],optfilt=[];' +
      'function lista_telas(){};\n' +
      text +
      '\n;return optionhtml;'
    // eslint-disable-next-line no-new-func
    const html = new Function(body)()
    if (!html) return []
    const sel = document.createElement('select')
    sel.innerHTML = html
    const list = []
    sel.querySelectorAll('option').forEach((o) => {
      const value = o.getAttribute('value')
      if (!value || o.disabled) return
      const grp = o.closest('optgroup')
      list.push({
        value,
        title: (o.textContent || '').trim(),
        group: grp ? grp.getAttribute('label') || '' : '',
      })
    })
    return list
  } catch (e) {
    return []
  }
}

// Resolve a screen reference (file name, ../svg/x.svg, x.svg) to an absolute URL.
export function screenUrl(ref) {
  if (!ref) return ''
  let r = String(ref).replace(/%2F/g, '/')
  if (r.indexOf('/') === -1) {
    r = '/svg/' + r + (r.endsWith('.svg') ? '' : '.svg')
  } else {
    r = r.replace(/^\.\.\//, '/')
    if (!r.startsWith('/')) r = '/' + r
  }
  return r
}
