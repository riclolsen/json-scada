# JSON-SCADA Admin UI

Single-page web application for operating and configuring JSON-SCADA: real-time
data and events, SVG synoptic displays, point and protocol configuration, user
and role management, and driver process control.

Built with Vue 3, Vuetify 4 and Vite, and served as static files by Nginx from
`src/AdminUI/dist` (see `conf-templates/nginx_http.conf` and
`nginx_https.conf`). All API calls go to the realtime web server
([`src/server_realtime_auth`](../server_realtime_auth/README.md)) through JWT
authentication.

Requires Node.js (see the repository `README.md` for the supported version) and
npm.

## Development Notes

### Install and run

    cd src/AdminUI
    npm install
    npm run dev

The development server starts with hot-reload and is accessible at
[http://localhost:3000](http://localhost:3000). The app calls the API with
same-origin relative URLs (`/Invoke/...`), so to use live data either put the dev
server behind the same reverse proxy as the realtime server or add a Vite
`server.proxy` entry that forwards `/Invoke` and the WebSocket endpoint to
`server_realtime_auth`.

The `dev` script is plain `vite`. If Vuetify's JSON import mapping prints
warnings during development, add `NODE_OPTIONS='--no-warnings'` to the `dev`
script (or `NODE_OPTIONS='--disable-warning=5401'` on Node 21.3.0 or later).

### Build for production

    npm run build

The output is written to `dist/`. On a normal installation Nginx serves that
directory at the site root, so no separate web server or process is needed.

    npm run preview    # serve the production build locally for a quick check
    npm run lint       # ESLint with --fix

### Layout

- `src/components/` — reusable Vue components
- `src/pages/` — route-level pages
- `src/plugins/` — Vuetify, router, i18n and other plugin setup
- `src/styles/` — global and shared styles
- `public/` — static assets, including the display images and the bundled
  `sage-cepel-displays` set

## Related documentation

- [SVG display editor](../svg-display-editor/README.md)
- [MongoDB schema](../../docs/schema.md)
- [GraphQL API](../../.agents/skills/graphql-api-apps/SKILL.md) — client-facing
  API for dashboards, integrations and scripts
