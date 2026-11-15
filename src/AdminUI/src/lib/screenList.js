/*
 * Loads the deployment's SVG screen list for the Display Viewer.
 * The generated public/svg/screen_list.js assigns an `optionhtml` string of
 * <optgroup>/<option value=...> markup. We evaluate it in a sandbox, then parse
 * the resulting HTML into {value,title,group} items. When that file is absent or
 * empty we fall back to the server's /Invoke/auth/listDisplays endpoint (an array
 * of svg file names). A user whose roles restrict displays (rights.displayList)
 * gets only those; the server enforces it too (403 on /svg/<file>).
 *
 * {json:scada} - Copyright 2020-2026 - Ricardo L. Olsen
 */

import { getUserObj } from './viewerHelpers'

// Parse an <optgroup>/<option> HTML string into [{value,title,group}].
function parseOptionHtml(html) {
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
}

// Fallback: ask the server for the available svg files.
async function listDisplaysFromServer() {
  try {
    const data = await (await fetch('/Invoke/auth/listDisplays')).json()
    if (!Array.isArray(data)) return []
    const qwerty = 'QWERTYUIOPASDFGHJKLZXCVBNM'
    return data.map((fname, i) => {
      let title = String(fname).replace(/\.svg$/i, '')
      if (i < 9) title += ' [' + (i + 1) + ']'
      else if (i === 9) title += ' [0]'
      if (i < qwerty.length) title += '{' + qwerty.charAt(i) + '}'
      return { value: '../svg/' + fname, title, group: 'Available Displays' }
    })
  } catch (e) {
    return []
  }
}

// Display file names the user may open: rights.displayList of the
// json-scada-user cookie, read from auth/userDisplays when it was too long for
// the cookie. null = not restricted (admin, or an empty list).
async function userDisplayList() {
  const rights = getUserObj()?.rights
  if (!rights || rights.isAdmin) return null
  let list = rights.displayList
  if (rights.displayListTruncated) {
    try {
      const data = await (await fetch('/Invoke/auth/userDisplays')).json()
      list = data && Array.isArray(data.displayList) ? data.displayList : []
    } catch (e) {
      list = []
    }
  }
  return Array.isArray(list) && list.length ? list : null
}

function fileName(value) {
  const base = String(value).split('/').pop()
  try {
    return decodeURIComponent(base)
  } catch (e) {
    return base
  }
}

async function loadAllScreens() {
  // 1) the deployment-generated screen_list.js (preferred)
  try {
    const text = await (await fetch('/svg/screen_list.js')).text()
    const body =
      'var optionhtml="";var optval=[],opttxt=[],optgroup=[],optfilt=[];' +
      'function lista_telas(){};\n' +
      text +
      '\n;return optionhtml;'
    // eslint-disable-next-line no-new-func
    const html = new Function(body)()
    if (html) {
      const list = parseOptionHtml(html)
      if (list.length) return list
    }
  } catch (e) {
    /* fall through to the server fallback */
  }
  // 2) fallback to /Invoke/auth/listDisplays
  return listDisplaysFromServer()
}

export async function loadScreenList() {
  const [all, allowed] = await Promise.all([loadAllScreens(), userDisplayList()])
  if (!allowed) return all
  const list = all.filter((it) => allowed.includes(fileName(it.value)))
  if (list.length) return list
  // the screen list names none of them: list the allowed files themselves
  return allowed.map((f) => ({ value: '../svg/' + f, title: String(f).replace(/\.svg$/i, ''), group: '' }))
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
