/*
 * Viewer configuration defaults — ES-module port of public/config_viewers_default.js
 * (Tabular/Alarms viewer subset). Site overrides in public/conf/config_viewers.js are
 * merged at runtime when present, so existing deployments keep their colors/refresh/selectors.
 *
 * {json:scada} - Copyright 2020-2026 - Ricardo L. Olsen
 */

export const defaultConfig = {
  // colors representing priorities for the alarms/tabular viewers
  ColorOfPriority: [
    'red',
    'yellow',
    'goldenrod',
    'plum',
    'silver',
    'silver',
    'silver',
    'silver',
    'silver',
    'silver',
    'silver',
  ],
  // colors representing the first up to last group1/substation
  ColorOfSubstation: [
    'cadetblue',
    'brown',
    'green',
    'magenta',
    'orange',
    'darkcyan',
    'goldenrod',
    'deepskyblue',
    'indigo',
    'lightseagreen',
  ],
  TabularViewer_Font:
    'Segoe UI,Source Sans Pro,calibri,consolas,arial,helvetica',
  TabularViewer_GridColor: '#BBBBBB',
  TabularViewer_AlmTxtColor: 'rgb(164, 128, 206)', // alarmed (unacked) color
  TabularViewer_FailTxtColor: '#AAAAAA', // failed value color
  TabularViewer_AckTxtColor: 'rgb(40, 130, 136)', // acknowledged alarm color
  TabularViewer_RefreshTime: 3, // refresh time (seconds)
  TabularViewer_LocaleDateTime: '', // e.g. "en-US" (empty = browser default)
  TabularViewer_LocaleDateTimeOptions: {},
  // custom buttons selecting a set of group1 filters on Alarms Viewer
  TabularViewer_CustomFiltersSelectors: [],

  // Events Viewer -------------------------------------------------------------
  EventsViewer_ToolbarColor: '#BBBBBB',
  EventsViewer_Font: 'Segoe UI,Source Sans Pro,calibri,consolas,arial,helvetica',
  EventsViewer_TableColor: '#E8E8E8',
  EventsViewer_GridColor: '#00000011',
  EventsViewer_AlmTxtColor: '#1976D2', // alarmed (unacked) — legible on both themes
  EventsViewer_FailTxtColor: '#C62828', // failed value — legible on both themes
  EventsViewer_AckTxtColor: '#757575', // acknowledged — legible on both themes
  EventsViewer_ElimTxtColor: '#9E9E9E', // eliminated/removed (until gone) — legible on both themes
  EventsViewer_RefreshTime: 12, // seconds
  EventsViewer_MaxRealtimeEvents: 750,
  EventsViewer_MaxHistoricalEvents: 2500,
  // 0=GPS(field), 1=local, 2=choose default GPS, 3=choose default local
  EventsViewer_TimeGPSorLocal: 2,
  EventsViewer_AllowFilter: 1,
  EventsViewer_Notific: 1,
  EventsViewer_PanicModePriorityLimit: 1,
  EventsViewer_LocaleTime: '',
  EventsViewer_LocaleDate: '',
  EventsViewer_LocaleTimeOptions: {},
  EventsViewer_LocaleDateOptions: {},
  // notification click handler (opens the SVG display for the group1)
  EventsViewer_NotificationClick: (nponto, id, group1) => {
    window.open(
      'display.html?SELTELA=../svg/' + group1 + '.svg',
      'screen',
      'dependent=no,height=1000,width=800,toolbar=no,directories=no,status=no,menubar=no,scrollbars=no,resizable=no,modal=no'
    )
  },

  // Display (Screen) Viewer ---------------------------------------------------
  ScreenViewer_RefreshTime: 3, // seconds
  ScreenViewer_SVGMaxWidth: 3840,
  ScreenViewer_SVGMaxHeight: 2160,
  ScreenViewer_Background: '#DDDDDD',
  ScreenViewer_ToolbarColor: 'lightslategray',
  ScreenViewer_RelationColor: 'red',
  ScreenViewer_TagFillColor: 'yellow',
  ScreenViewer_TagStrokeColor: 'red',
  ScreenViewer_TagInhAlmFillColor: 'lightgray',
  ScreenViewer_TagInhAlmStrokeColor: 'gray',
  ScreenViewer_DateColor: 'white',
  ScreenViewer_TimeMachineDateColor: 'orange',
  ScreenViewer_TimeMachineBgColor: 'green',
  ScreenViewer_AlmBoxTableColor: '#DCDCEE',
  ScreenViewer_AlmBoxGridColor: 'whitesmoke',
  ScreenViewer_BarBreakerSwColor: 'steelblue',
  ScreenViewer_ShowScreenNameTB: 1,
  ScreenViewer_DefaultDisplayPreviewWidth: 700,
  ScreenViewer_DefaultDisplayPreviewHeight: 480,
  ScreenViewer_DisplayPreviewZoom: 0.5,
  ScreenViewer_DisplayPreviewTimeout: 1500, // ms hover delay before showing a preview
  ScreenViewer_SecurityCardScale: 1.0, // scale of the pinned annotation badge
  // Pinned documental-annotations panel (#PIN notes filtered by #set_filter group1)
  ScreenViewer_PinnedAnnotationsBGColor: 'slategray',
  ScreenViewer_PinnedAnnotationsTextColor: 'white',
  ScreenViewer_PinnedAnnotationsWidth: '300px',
  ScreenViewer_PinnedAnnotationsBorder: '1px solid black',
  ScreenViewer_PinnedAnnotationsFont: 'calibri',
  ScreenViewer_PinnedAnnotationsFontSize: '12px',
  ScreenViewer_PinnedAnnotationsRefresh: 9.5, // seconds between panel refreshes
  ScreenViewer_SlideShowInterval: 10, // seconds
  ScreenViewer_EnableTimeMachine: 1,
  VisorTelas_BackgroundSVG: '#dddddd',

  // SOE notification matching constants (Viewers_*) ---------------------------
  Viewers_AlmTxtHighlight1: 'OPERATED',
  Viewers_IDTxtHighlight2: 'XCBR',
  Viewers_DescrTxtHighlight2: ':status',
  Viewers_NotificTagText: 'XCBR',
  Viewers_NotificDescrText: ':status',
  Viewers_NotificEventText: 'OFF',
  Viewers_NotificCancelEventText: 'ON',
  Viewers_AddToNotificEventText: 'OPERATED',
}

// Read any window-global overrides exposed by public/conf/config_viewers.js
// (that legacy script assigns plain `var TabularViewer_*` / `ColorOf*` globals).
function readGlobalOverrides() {
  const keys = Object.keys(defaultConfig)
  const out = {}
  for (const k of keys) {
    if (typeof window !== 'undefined' && window[k] !== undefined) out[k] = window[k]
  }
  return out
}

let cached = null

// Load and cache the effective config. Attempts to load the site override script
// (public/conf/config_viewers.js) once; falls back silently to defaults.
export async function loadViewersConfig() {
  if (cached) return cached
  try {
    await loadLegacyConfigScript('/conf/config_viewers.js')
  } catch (e) {
    /* override script optional */
  }
  cached = { ...defaultConfig, ...readGlobalOverrides() }
  return cached
}

export function getViewersConfig() {
  return cached || defaultConfig
}

function loadLegacyConfigScript(src) {
  return new Promise((resolve, reject) => {
    if (typeof document === 'undefined') {
      resolve()
      return
    }
    // Avoid loading twice
    if (document.querySelector(`script[data-viewers-config="${src}"]`)) {
      resolve()
      return
    }
    const s = document.createElement('script')
    s.src = src
    s.async = true
    s.setAttribute('data-viewers-config', src)
    s.onload = () => resolve()
    s.onerror = () => reject(new Error('config_viewers.js not found'))
    document.head.appendChild(s)
  })
}
