# DOX: src/custom-developments — Custom Development Apps

## Purpose

Web apps (dashboards, equipment faceplates, special views) served by `server_realtime_auth` under `/custom-developments/<folder>/` and listed in the AdminUI under Custom Developments, plus the example templates users copy to build their own.

## Ownership

- custom-developments owns the templates `basic_bargraph`, `advanced_dashboard`, `transformer_with_command` and the user guide `README.md`
- Serving and routing are owned by `server_realtime_auth` (`app/routes/user.routes.js`); the development guide and checks served to AI assistants by `mcp-json-scada-db` (`jsonscada/custom-dev.ts`, `tools/custom-dev.ts`)

## Local Contracts

- Each app is one folder; the folder name is the URL. A folder is routed when it has `dist/index.html` (or a plain `index.html`); routes are created when `server_realtime_auth` starts
- Templates are Astro 5 + React 18 + Tailwind + shadcn/ui (TypeScript). `astro.config.mjs` derives `base: /custom-developments/<folder>` from the folder name: keep that, so a copied template needs no config change
- Data goes only through `src/lib/scadaOpcApi.ts` (same-origin `fetch('/Invoke/')` with the login cookie; the user's role rights apply). Every template holds an identical copy: change it in all templates at once. The MCP server parses its exported functions, JSDoc, data interfaces, `OpcStatusCodes` and `OpcAcknowledge` at request time and compares each app's copy with the templates'
- Components that call the API mount with `client:only="react"`
- The installers ship the templates without `node_modules` (built `dist` included)

## Work Guidance

- Keep `README.md` (user workflow and API list) consistent with `scadaOpcApi.ts` and the MCP guide `CUSTOM_DEV_GUIDE`
- Commands in templates follow the `transformer_with_command` flow: `issueCommand`, then poll `getCommandAckStatus` until Good/Bad or a timeout

## Verification

- `npm install` and `npm run build` in a template (`astro check` then `astro build`)
- MCP `check_custom_developments` (all apps, then `name` for referenced tags): templates served, no issues
