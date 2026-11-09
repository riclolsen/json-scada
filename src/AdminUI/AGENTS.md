# DOX: src/AdminUI — Web Admin Interface

## Purpose

Vue.js single-page application (SPA) for the JSON-SCADA web administration interface: real-time data visualization, configuration management, SVG synoptic displays, user management, and protocol monitoring.

## Ownership

- AdminUI owns all Vue components, routing, state management, styles, and the Vite build configuration
- All changes to the web interface happen here

## Local Contracts

- **Framework:** Vue 3 + Vuetify 4 with Vite build tool
- **Package manager:** npm (see `package-lock.json`)
- **Linting:** ESLint (`npm run lint`) + Prettier (`.prettierrc.json`)
- **Browser targets:** defined in `.browserslistrc`
- **Build output:** `dist/` directory, served by Nginx (see `conf-templates/nginx_*.conf`)
- **Dev server:** `npm run dev` (Vite, http://localhost:3000)
- **Production build:** `npm run build`
- All API calls go through the realtime data server (JWT auth)
- Vue component naming: PascalCase. Single-file components with `<script setup>` preferred
- Styles: scoped CSS. Global styles in `src/` root
- Locales: `src/locales/*.json` (13 files) carry the same keys; add every new key to all of them
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
