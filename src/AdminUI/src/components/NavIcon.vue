<template>
  <svg
    class="nav-icon"
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.75"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
    v-html="ICONS[name] || ICONS.dot"
  ></svg>
</template>

<script setup>
// JSON-SCADA navigation icon set: 24x24 grid, 1.75 stroke, round caps/joins,
// drawn with currentColor so they follow the theme and the active-item color.
defineProps({
  name: { type: String, required: true },
  size: { type: [Number, String], default: 22 },
})

const ICONS = {
  // home / welcome
  home: '<path d="M3.5 10.5 12 4l8.5 6.5"/><path d="M5.5 9v10.5h13V9"/><path d="M10 19.5v-5h4v5"/>',
  // display viewer: monitor showing a live process signal
  display:
    '<rect x="2.5" y="3.5" width="19" height="13.5" rx="1.75"/><path d="M9 21h6M12 17v4"/>' +
    '<path d="M5.5 11h3l1.5-3.5 2.5 6.5 1.5-3h4.5"/>',
  // alarms viewer: bell with alert waves
  alarms:
    '<path d="M6.5 16.5V11a5.5 5.5 0 0 1 11 0v5.5l1.5 2h-14z"/><path d="M10 21a2.2 2.2 0 0 0 4 0"/>' +
    '<path d="M3.5 8a9 9 0 0 1 2.4-4M20.5 8a9 9 0 0 0-2.4-4"/>',
  // tabular viewer: data grid with header row
  tabular:
    '<rect x="3" y="4" width="18" height="16" rx="1.75"/><path d="M3 9h18M3 14.5h18M9.5 9v11"/>',
  // events viewer: event list with clock
  events:
    '<path d="M3 6h10M3 11h7M3 16h5.5"/><circle cx="16.5" cy="15" r="5"/><path d="M16.5 12.5V15l1.8 1.2"/>',
  // CIM-EMS: power network (buses and branches)
  cimEms:
    '<circle cx="5" cy="12" r="2.2"/><circle cx="19" cy="5.5" r="2.2"/><circle cx="19" cy="18.5" r="2.2"/>' +
    '<path d="M7.2 12h4.3M11.5 12l5.6-5.2M11.5 12l5.6 5.2"/><path d="M11.5 9.5v5"/>',
  // Grafana: trend chart on axes
  grafana: '<path d="M3.5 3.5v17h17"/><path d="m7 15 4-5 3 3 5.5-7"/><circle cx="19.5" cy="6" r=".6"/>',
  // Metabase: database / BI
  metabase:
    '<ellipse cx="12" cy="5.5" rx="7" ry="2.5"/><path d="M5 5.5v13c0 1.4 3.1 2.5 7 2.5s7-1.1 7-2.5v-13"/>' +
    '<path d="M5 12c0 1.4 3.1 2.5 7 2.5s7-1.1 7-2.5"/>',
  // admin: shield with user
  admin:
    '<path d="M12 3l7.5 3v5.5c0 4.6-3.2 8.2-7.5 9.5-4.3-1.3-7.5-4.9-7.5-9.5V6z"/>' +
    '<circle cx="12" cy="10.2" r="2.2"/><path d="M8.6 16.3c.8-1.5 2-2.3 3.4-2.3s2.6.8 3.4 2.3"/>',
  // Node-RED: connected flow nodes
  nodered:
    '<rect x="2.5" y="4" width="7" height="5" rx="1.5"/><rect x="14.5" y="15" width="7" height="5" rx="1.5"/>' +
    '<path d="M9.5 6.5c4.5 0 .5 11 5 11"/>',
  // n8n: branching automation workflow
  n8n:
    '<circle cx="4.5" cy="12" r="2"/><circle cx="12" cy="12" r="2"/><circle cx="19.5" cy="6.5" r="2"/>' +
    '<circle cx="19.5" cy="17.5" r="2"/><path d="M6.5 12H10M14 12c2 0 1.5-5.5 3.5-5.5M14 12c2 0 1.5 5.5 3.5 5.5"/>',
  // Apollo / GraphQL API: hexagon with hub
  apollo:
    '<path d="M12 2.8l8 4.6v9.2l-8 4.6-8-4.6V7.4z"/><circle cx="12" cy="12" r="2"/>' +
    '<path d="M12 10V2.8M10.3 13 4 16.6M13.7 13l6.3 3.6"/>',
  // SVG display editor: bezier pen
  svgedit:
    '<path d="M4 20c1.5-6 5-9.5 10-10.5"/><rect x="2.8" y="18.8" width="2.4" height="2.4" rx=".4"/>' +
    '<rect x="12.8" y="8.3" width="2.4" height="2.4" rx=".4"/><path d="M15.2 8.3 19 4.5"/><circle cx="20" cy="3.8" r="1.2"/>' +
    '<path d="M14 21h6"/>',
  // log viewer: terminal
  logViewer: '<rect x="3" y="4" width="18" height="16" rx="1.75"/><path d="m7 9.5 3 2.5-3 2.5M12.5 15H17"/>',
  // custom developments: code
  customDevelopments: '<path d="m8 7-5 5 5 5M16 7l5 5-5 5M13.5 4.5l-3 15"/>',
  // about: info
  about: '<circle cx="12" cy="12" r="9"/><path d="M12 11v5.5"/><path d="M12 7.6v.1"/>',
  // utility icons
  openNew:
    '<path d="M14 4h6v6M20 4l-8.5 8.5"/><path d="M18 14v4.5a1.5 1.5 0 0 1-1.5 1.5h-11A1.5 1.5 0 0 1 4 18.5v-11A1.5 1.5 0 0 1 5.5 6H10"/>',
  openHere: '<path d="M4 12h12M11 7l5 5-5 5"/><path d="M20 4v16"/>',
  chevronRight: '<path d="m9.5 6 6 6-6 6"/>',
  collapse: '<path d="m11 7-5 5 5 5M18 7l-5 5 5 5"/>',
  expand: '<path d="m13 7 5 5-5 5M6 7l5 5-5 5"/>',
  language:
    '<circle cx="12" cy="12" r="9"/><path d="M3 12h18"/><path d="M12 3c2.5 2.6 3.8 5.6 3.8 9s-1.3 6.4-3.8 9c-2.5-2.6-3.8-5.6-3.8-9S9.5 5.6 12 3z"/>',
  sun:
    '<circle cx="12" cy="12" r="4"/><path d="M12 2.5v2M12 19.5v2M2.5 12h2M19.5 12h2M5.3 5.3l1.4 1.4M17.3 17.3l1.4 1.4M5.3 18.7l1.4-1.4M17.3 6.7l1.4-1.4"/>',
  moon: '<path d="M20 14.5A8 8 0 1 1 9.5 4a6.5 6.5 0 0 0 10.5 10.5z"/>',
  user: '<circle cx="12" cy="8" r="4"/><path d="M4.5 20.5c1.2-3.6 4-5.5 7.5-5.5s6.3 1.9 7.5 5.5"/>',
  logout:
    '<path d="M9 20H5.5A1.5 1.5 0 0 1 4 18.5v-13A1.5 1.5 0 0 1 5.5 4H9"/><path d="m16 16.5 4.5-4.5L16 7.5M20.5 12H9"/>',
  password:
    '<circle cx="8" cy="15" r="4"/><path d="m10.8 12.2 8.7-8.7M16.5 6.5 19 9M14.2 8.8l2 2"/>',
  menu: '<path d="M4 6.5h16M4 12h16M4 17.5h16"/>',
  dot: '<circle cx="12" cy="12" r="3"/>',
}
</script>

<style scoped>
.nav-icon {
  flex: 0 0 auto;
  display: block;
}
</style>
