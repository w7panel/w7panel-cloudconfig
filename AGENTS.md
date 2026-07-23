# Repository Guidelines

## Project Structure & Module Organization

- `main.go` wires the controller manager, HTTP API, and embedded frontend.
- `api/v1alpha1/` defines the cluster-scoped `CloudConfig` CRD types and generated deep-copy code.
- `controllers/` contains reconciliation, update propagation, and automatic deployment behavior.
- `pkg/configcenter/` owns configuration resolution, validation, precedence, and workload application logic.
- `pkg/httpapi/` exposes OIDC login and `/cloudconfig-api/v1` endpoints.
- `ui/src/` contains the Vue 3 + Arco Design frontend; tests use `*.test.js` beside source files.
- `charts/w7panel-cloudconfig/` contains the Helm chart and CRD manifest. `ui/dist/` and `ui/node_modules/` are generated and must not be committed.

## Build, Test, and Development Commands

```bash
go test ./...                         # run all Go tests
cd ui && npm test                     # run Vitest once
cd ui && npm run dev                  # start Vite on all interfaces
cd ui && npm run build                # create the production frontend
helm lint charts/w7panel-cloudconfig  # validate the chart
helm template test charts/w7panel-cloudconfig --include-crds
git diff --check                      # detect whitespace errors
```

If the default Go cache is read-only, set `GOCACHE`, `GOMODCACHE`, and `GOPATH` to directories under `/tmp`.

## Coding Style & Naming Conventions

Format Go files with `gofmt`; use tabs and standard Go naming (`CloudConfig`, `configName`). Keep packages small and lowercase. Vue and JavaScript use two-space indentation, single quotes, and descriptive camelCase names. Keep API paths under `/cloudconfig-api/v1`. Preserve configuration precedence: inherited values, then current public values, then the selected version.

## Testing Guidelines

Place Go tests in `*_test.go` and frontend tests in `*.test.js`. Add focused regression tests for inheritance, revision/staleness, Kubernetes application order, authentication headers, and legacy CRD compatibility. Run Go, frontend, Helm, and build checks before opening a PR.

## Commit & Pull Request Guidelines

History follows short Conventional Commit-style subjects such as `feat: ...`, `fix: ...`, and `refactor: ...`. Keep commits scoped and imperative. PRs should explain behavior changes, list validation commands, link relevant issues, and include screenshots for UI changes. Call out CRD or Helm upgrade steps explicitly.

## Security & Configuration Tips

Never commit tokens or kubeconfigs. Business APIs require `Authorization-config: Bearer <OIDC ID token>`; W7Panel proxy calls may also require `X-W7Panel-Token`. Helm does not upgrade files under `crds/`, so schema changes require an explicit CRD apply before chart upgrade.
