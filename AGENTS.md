# Project workflow

- Read `.agent/CONTINUITY.md` first. Never inspect or change macOS network configuration or installed software settings.
- Production images are built only by `.github/workflows/ci.yml`. Do not build images locally.
- Preferred development/check environment: `docker compose -f compose.dev.yaml run --rm web-check` and `docker compose -f compose.dev.yaml run --rm api-check` (official Node/Go images, no local image builds).
- If Docker is unavailable, use existing Node and Go runtimes and report the limitation. Do not install system tools.
- Frontend: `cd web && npm ci && npm run check && npm run build`.
- Backend: `cd api && go test -race ./... && go vet ./... && go build ./cmd/server`.
- Do not expose oneimg credentials to browser bundles. Keep uploads bounded and validate image content before decoding.
