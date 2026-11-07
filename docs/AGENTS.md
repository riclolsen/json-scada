# DOX: docs — Documentation

## Purpose

Project documentation: installation guides, architecture descriptions, schema documentation, developer guides, integration guides, and report generator docs.

## Ownership

- docs owns all project documentation files
- Each src component's `README.md` is owned by that component

## Local Contracts

- Markdown (`.md`) format for all docs
- Architecture diagrams in PlantUML (`.txt`) and Draw.io (`.drawio`) formats
- PNG images for rendered diagrams
- Key documents:
  - `install.md` — installation guide (primary)
  - `docker_image.md` — Docker deployment
  - `build_windows.md` — Windows build toolchain and installer build
  - `schema.md` — MongoDB/PostgreSQL schema documentation
  - `DEVELOPER_GUIDE.md` — protocol driver development guide
  - `nodered-integration.md` — Node-RED integration guide
  - `n8n-integration.md` — n8n integration guide
  - `report_generators.md` — report generation docs
  - `JSON-SCADA_Arquitecture.txt` — PlantUML architecture diagram
  - `JSON-SCADA_ARCHITECTURE.drawio` — Draw.io architecture diagram
  - `JSON-SCADA_Connections.drawio` — Connections diagram
  - `sync-index.mjs` — regenerates root `index.md` from `README.md` and validates their links/anchors
  - `check-links.mjs` — checks relative links and `#anchors` in all tracked Markdown against the committed tree

## Work Guidance

- Keep docs in sync with code changes
- Architecture diagrams should be updated when protocol drivers or major services change
- Screenshots go in `screenshots/` subdirectory
- Use relative links to reference other docs and source files
- Renaming a heading changes its anchor: search for links to it (`grep -rn '#old-anchor'`) and update them; for README/index links `node docs/sync-index.mjs --check` catches it

## Verification

- `node docs/sync-index.mjs --check` — root `index.md` matches `README.md`; their links and heading anchors resolve
- `node docs/check-links.mjs` — every relative link/anchor in tracked Markdown resolves to a committed file (exit 1 otherwise)
