# Repository Guidelines

## Project Structure & Module Organization

This repository is a Go backend with a Vite/React admin UI. The backend entry point is `cmd/chatgpt2api/main.go`; implementation packages live under `internal/` (`httpapi`, `service`, `protocol`, `backend`, `storage`, `config`, and helpers). Frontend source is in `web/src/`, with pages under `web/src/app/`, shared UI in `web/src/components/`, API helpers in `web/src/lib/`, and stores in `web/src/store/`. Screenshots are in `assets/`; operational notes are in `docs/`.

## Rule
**Do not write compatibility code**

## Build, Test, and Development Commands

- `go test ./...` runs all backend tests.
- `go build -ldflags "-X chatgpt2api/internal/version.Version=1.0.0" -o chatgpt2api ./cmd/chatgpt2api` builds the service binary.
- `CHATGPT2API_AUTH_KEY=your_secret_key ./chatgpt2api` runs the backend locally after build.
- `docker compose up -d` starts the default containerized deployment using `.env`.
- `docker compose -f docker-compose.yml -f docker-compose.build.yml up -d --build` rebuilds the image from local source.
- `cd web && npm run dev` starts the frontend dev server.
- `cd web && npm run build` type-checks and builds the frontend.
- `cd web && npm run lint` runs ESLint.

## Coding Style & Naming Conventions

Use `gofmt` for Go code and keep package names short, lowercase, and domain-oriented. Place tests beside the code they exercise as `*_test.go`. Frontend code uses TypeScript, React 19, Vite, ESLint, Tailwind CSS, and shadcn-style components. Prefer kebab-case filenames for React components (`image-composer.tsx`) and PascalCase exports. Reuse helpers from `web/src/lib/` and primitives from `web/src/components/ui/` before adding abstractions.

## Design Guidelines

For frontend UI and visual design work, consult `DESIGN.md` for the project design system and apply those rules unless the user explicitly requests a different direction.

## Testing Guidelines

Backend coverage is Go test based; add focused unit tests in the relevant `internal/**` package when changing service, protocol, config, or route behavior. Keep test names descriptive, for example `TestRegisterFlow...` or `TestImageTask...`. Frontend changes should pass `npm run build` and `npm run lint`; add UI tests only if a framework is introduced later.

## Commit & Pull Request Guidelines

Recent history uses Conventional Commit-style subjects such as `feat: ...`, `chore: ...`, `feat(httpapi): ...`, and breaking markers like `feat!:`. Keep subjects concise and scoped to intent. Pull requests should include a summary, verification (`go test ./...`, `npm run build`, screenshots for UI changes), linked issues when applicable, and notes for config or deployment changes.

## Security & Configuration Tips

Do not commit real secrets. Start from `.env.example`, set `CHATGPT2API_AUTH_KEY`, and keep account tokens, proxy credentials, and database URLs local or in deployment secrets. Public deployments should add external access control.
