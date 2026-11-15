# DOX: src/AdminUI — Web Admin Interface

## Purpose

Vue.js single-page application (SPA) for the JSON-SCADA web administration interface: real-time data visualization, configuration management, SVG synoptic displays, user management, and protocol monitoring.

## Ownership

- AdminUI owns all Vue components, routing, state management, styles, and the Vite build configuration
- All changes to the web interface happen here

## Local Contracts

- **Framework:** Vue 3 + Vuetify 4 with Vite build tool
- **Package manager:** npm (see `package-lock.json`)
- **Linting:** ESLint 10 flat config `eslint.config.mjs` (`@eslint/js` recommended + `eslint-plugin-vue` essential; `npm run lint`, which also fixes) + Prettier (`.prettierrc.json`)
- **Browser targets:** defined in `.browserslistrc`
- **Build output:** `dist/` directory, served by Nginx (see `conf-templates/nginx_*.conf`)
- **Dev server:** `npm run dev` (Vite, http://localhost:3000)
- **Production build:** `npm run build`
- All API calls go through the realtime data server (JWT auth)
- The readable `json-scada-user` cookie (set by `server_realtime_auth` at sign-in) carries the user's flags and `rights.displayList`; when `rights.displayListTruncated` is true the list was too long for a cookie and must be read from `GET /Invoke/auth/userDisplays`. `lib/screenList.js` lists only those displays (unless `rights.isAdmin`; the server also enforces them). A server answer with `unavailable: true` (database down) keeps the session
- Vue component naming: PascalCase. Single-file components with `<script setup>` preferred
- Styles: scoped CSS. Global styles in `src/` root
- Locales: `src/locales/*.json` (13 files) carry the same keys; add every new key to all of them
- Display Viewer engine (`lib/sage/`): `!JSON <tag> <path>` tag references and the `$J(tag, path)` script function read members of a point's `valueJson` (path syntax in `lib/sage/sageJson.js`, mirrored by `src/mcp-json-scada-db/src/jsonscada/json-path.ts`; markup in the svg-scada skill section 10). The engine requests `valueJson` only for those points, through the `_ValueJson` node flag of `opcClient.readPoints` (served by `server_realtime_auth`). Likewise `!STR <tag> <selector>` / `$S(tag, selector)` take parts of the raw `valueString` (returned on every read; selector grammar in `lib/sage/sageStr.js`, parsed and never evaluated, mirrored by `src/mcp-json-scada-db/src/jsonscada/str-select.ts`; skill section 11). Both share `memberState` for display semantics
- Viewers are native Vue pages (no iframed legacy HTML): `/display-viewer` (`?screen=<name>`), `/tabular-viewer`, `/alarms-viewer` (`TabularViewerPage` with `mode: 'ALARMS_VIEWER'`), `/events-viewer` (`?modo=4` historical), `/trend-viewer` (`?point=<key or tag>`; `TrendViewerPage`, history from `opcClient.readHistory` plus 5 s live reads). The former `/*-viewer-new` paths redirect to these
- `?embed=1` on any route hides the app bar (`App.vue` provides `embedded`); pages used inside iframes (Display Viewer hover previews, `#foreign_object`) size to `100vh` with it. The Display Viewer also hides its toolbar and keeps `embed` when it changes screen; `/trend-viewer?controls=0` hides the trend toolbar (hover preview of an analog reading)
- `public/` holds static files only, served at the site root: `images/`, `sounds/`, `favicon.ico`, `release_notes.txt` (About page crawl), `conf/config_viewers.js` (site overrides read by `lib/viewersConfig.js`), `camera.html` + `lib/jsmpeg.min.js` (`#camera` stream page, see `src/camera-onvif`), `scripts/main.js` + `sage-cepel-displays/` (self-contained updater for exported SAGE HTML displays). Do not add legacy jQuery viewers or libraries back there
- `components/CimEmsPage.vue` (route `/cim-ems`, dashboard card, open to any signed-in user) hosts `components/CimEmsTab.vue`, the CIM-EMS UI; it talks only to `/Invoke/auth/cimEms/*` (see `src/server_realtime_auth/AGENTS.md`) and waits for driver-executed requests by polling `listRequests`

## Work Guidance

- Maintain responsive design for mobile/tablet/desktop
- Use existing components and patterns when adding features
- SVG synoptic displays use the `svg-scada` skill conventions (JSON in `inkscape:label`)
- Internationalization (i18n): use established patterns in the codebase

## Verification

- `npm install` — install dependencies
- `npm run build` — production build must succeed with no errors
- `npm run lint` — lint check
