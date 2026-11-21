# Ruang Sanad panel (`/panel`)

React 19 + Vite + TypeScript + Tailwind 4 SPA, embedded into the GOWA binary.

- Build output goes to `../src/ui/webapp/dist` (git-ignored except `.gitkeep`); the
  Docker `web` stage builds it before `go build`. Without a build the Go route
  serves a placeholder page.
- The Go server injects `<base href="{APP_BASE_PATH}/panel/">`; `src/lib/api.ts`
  derives the API root and router basename from it. Keep asset URLs relative.
- Styling follows the Ruang Sanad design system (tokens in `src/index.css`). All UI
  copy is Bahasa Indonesia in `src/i18n/id.ts`; times are shown in WIB.
- Never copy code, components or assets from `aldinokemal/gowa-ui` (no licence).
- Restricted (CS) accounts may only reach `GET /panel*`, `/api/app/me` and their own
  device's endpoints; see `src/cmd/device_scope.go`.

Commands (from `web/`): `npm ci`, `npm run dev` (proxy to `go run . rest` on :3000),
`npm run lint`, `npm run typecheck`, `npm test`, `npm run build`.
