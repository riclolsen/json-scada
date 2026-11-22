/*
 * Navigation menu of the AdminUI side panel (components/AppSidebar.vue).
 *
 * Item fields:
 *   id        NavIcon name and stable key
 *   titleKey  i18n key of the label
 *   route     in-app route opened in the same window ('' = the page cannot run
 *             inside the app, e.g. Metabase refuses to be iframed: it always
 *             opens in a new window)
 *   page      URL opened in a new window
 *   adminOnly shown only to users whose roles grant isAdmin
 *   right     shown only to users whose roles grant this right
 *   rightOrAdmin  shown to admins and to users whose roles grant this right
 *
 * {json:scada} - Copyright 2020-2026 - Ricardo L. Olsen
 */

export const navGroups = [
  {
    key: 'operation',
    items: [
      { id: 'display', titleKey: 'dashboard.displayViewer', route: '/display-viewer', page: '/#/display-viewer' },
      { id: 'alarms', titleKey: 'dashboard.alarmsViewer', route: '/alarms-viewer', page: '/#/alarms-viewer' },
      { id: 'tabular', titleKey: 'dashboard.tabularViewer', route: '/tabular-viewer', page: '/#/tabular-viewer' },
      { id: 'events', titleKey: 'dashboard.eventsViewer', route: '/events-viewer', page: '/#/events-viewer' },
    ],
  },
  {
    key: 'analytics',
    items: [
      { id: 'grafana', titleKey: 'dashboard.grafana', route: '/grafana', page: '/grafana' },
      { id: 'metabase', titleKey: 'dashboard.metabase', route: '', page: '/metabase' },
      {
        id: 'customDevelopments',
        titleKey: 'dashboard.customDevelopments',
        route: '/custom-developments',
        page: '/#/custom-developments',
      },
    ],
  },
  {
    key: 'engineering',
    items: [
      { id: 'admin', titleKey: 'dashboard.admin', route: '/admin', page: '/#/admin', adminOnly: true },
      { id: 'cimEms', titleKey: 'dashboard.cimEms', route: '/cim-ems', page: '/#/cim-ems' },
      {
        id: 'secEvents',
        titleKey: 'dashboard.secEventsViewer',
        route: '/sec-events-viewer',
        page: '/#/sec-events-viewer',
        right: 'viewSecEvents',
      },
      {
        id: 'svgedit',
        titleKey: 'dashboard.svgedit',
        route: '/svg-edit',
        page: '/svgedit',
        rightOrAdmin: 'editDisplays',
      },
      { id: 'nodered', titleKey: 'dashboard.nodered', route: '/nodered', page: '/#/nodered', adminOnly: true },
      { id: 'n8n', titleKey: 'dashboard.n8n', route: '/n8n', page: '/#/n8n', adminOnly: true },
      { id: 'apollo', titleKey: 'dashboard.apollo', route: '/apollo', page: '/#/apollo', adminOnly: true },
      { id: 'logViewer', titleKey: 'dashboard.logViewer', route: '/log-viewer', page: '/log-viewer-ui', adminOnly: true },
    ],
  },
]

export const aboutItem = { id: 'about', titleKey: 'dashboard.about', route: '/about', page: '/#/about' }

export const homeItem = { id: 'home', titleKey: 'nav.home', route: '/dashboard', page: '/#/dashboard' }

// Groups with the items this user may see (empty groups dropped).
export function visibleGroups(isAdmin, rights = {}) {
  const allowed = (it) =>
    (isAdmin || !it.adminOnly) &&
    (!it.right || !!rights?.[it.right]) &&
    (!it.rightOrAdmin || isAdmin || !!rights?.[it.rightOrAdmin])
  return navGroups
    .map((g) => ({ ...g, items: g.items.filter(allowed) }))
    .filter((g) => g.items.length)
}
