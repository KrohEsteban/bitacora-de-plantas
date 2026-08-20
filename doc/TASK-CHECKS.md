# TASK — Add full checks ecosystem + root README (Bitácora de Plantas)

Work on the current directory (a Go + HTMX v4 + Alpine.js app that stores plant data on the filesystem under data/plants/<slug>). The code currently has NO tests. Add, per Esteban's conventions, a complete check ecosystem: unit tests, integration tests, end-to-end tests, linters, a Makefile (all run inside Docker), CI/CD, and a root README.md.

## Goal (Definition of Done)
Running `make ci` inside Docker passes: lint + unit + integration + E2E tests, together. Same command is what CI runs.

## 1. Unit tests (Go)
- Test the pure logic: slug generation, file naming/validation, image content-type detection, storage helpers (plant CRUD, image list, total size), template date formatting.
- Use Go's stdlib `testing` + `httptest`. Table-driven where it helps.

## 2. Integration tests (Go)
- Test the full HTTP surface against a REAL running server on an EPHEMERAL temp data dir (set via env var, e.g. `BITACORA_DATA_DIR`; the code must read the data path from that env var with a default of `data`).
- Cover the routes: `GET /` (collage grid), `POST /plantas` (create plant with images, multipart), `GET /planta/{slug}` (detail), `PUT /planta/{slug}` (edit), `POST /planta/{slug}/imagenes` (add images), `DELETE /planta/{slug}` and `DELETE /.../imagenes/{file}` (delete); `GET /img/{slug}/{file}?thumb=1&original=1`.
- Assert real behaviors: 200s, slug redirects, 404 for missing plant, image + derived `.thumb`/`.display` files created with correct content types, footer total updates in the HTML, empty state.
- Clean up the temp dir with `t.TempDir()`.

## 3. End-to-end tests (E2E)
- REAL browser automation against the app running in Docker (the app must expose the data dir via env so tests use a temp/volume).
- Use **chromedp** (Go, keeps the stack single-language / no Node). Drive the built app: open the main page, verify the collage renders, create a plant via the modal (multipart upload of a sample image), see it in the collage, open detail, verify description + image, delete the plant, verify empty state.
- The E2E must run headless in a Docker stage (chromium + driver installed) and be triggered by `make test-e2e`.

## 4. Linters
- Use **golangci-lint** with a sensible default `.golangci.yml` (standard linters enabled). Provide a `make lint` target.

## 5. Makefile (all inside Docker)
Targets:
- `make lint` → golangci-lint in a Docker container
- `make test` (unit+integration) → `go test` for `.` in a Docker container (mount the code, ephemeral temp data)
- `make test-e2e` → chromedp E2E against the app in Docker
- `make ci` → runs lint + test + test-e2e in order (this is what CI calls)
- `make up` / `make down` → docker compose helpers
Use a `Dockerfile.test` (multi-stage, golang + tools + chromium for E2E) or docker-compose services. Prefer reusing the existing `docker-compose.yml` for the app when sensible.

## 6. CI/CD (GitHub Actions)
- `.github/workflows/ci.yml`: on push + PR → runs `make ci`. Simple, Go setup + docker compose. Must be valid and testable.

## 7. Root README.md
Create a proper **root `README.md`** (GitHub will render it on the repo page; docs/TASK and PLAN files stay in doc/). Include: what it is, features (collage, plant journal), tech stack (Go, HTMX v4, Alpine, Docker), how to run (`docker compose up -d --build` → http://localhost:8080), how data is stored (filesystem + Docker volume), how to test (`make ci`), project structure, roadmap note, link to docs in doc/.

## Conventions (respect them)
- Merge: used by Esteban (squash). 
- Any TASK/doc `.md` goes in `doc/` (so put this file: `doc/TASK-CHECKS.md`).
- No hardcoded secrets. `data/` stays gitignored.
- Keep the existing app behavior working (don't break the collage/app).

## Deliverable
Create/update all files, run `make ci` and FIX until it passes, and report the exact commands + results. Do not push — a branch is already prepared (`feat/checks`); just apply changes on the working tree. If you need to run the app, use Docker; don't disturb a running container on port 8080 if present (use a different port / isolated compose for tests).
