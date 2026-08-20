# 🌿 Bitácora de Plantas

A minimalist web app to keep a plant journal: create plant records, track their
growth with photos, and keep care notes — all stored on the filesystem, no
database required.

Built with **Go** (stdlib `net/http`), **HTMX**, **Alpine.js** and **Docker**.

> Full Spanish user guide: [`doc/README.md`](doc/README.md) · Task/planning docs
> live in [`doc/`](doc/).

## Features

- 🖼️ **Circular collage grid** — the home screen renders a fresh, randomized
  scrapbook-style collage per plant on every load.
- 📝 **Plant journal** — create/rename/edit/delete plants, with free-form care
  notes (watering, light, temperature…).
- 📷 **Photo gallery** — upload multiple images per plant (JPG/PNG/WebP); the
  app auto-generates thumbnail and display versions in real JPEG.
- 🔒 **Safety first** — MIME sniffing, filename/path sanitization and
  per-plant unique slugs built in.
- 💾 **No database** — everything is JSON + files on disk in a Docker volume.

## Tech stack

| Layer    | Tech                                                          |
| -------- | ------------------------------------------------------------- |
| Backend  | Go 1.23 (standard library `net/http`)                         |
| Frontend | HTMX 2.x + Alpine.js 3 + custom CSS (no build step)           |
| Images   | `golang.org/x/image` (decode + high-quality resizing)         |
| Runtime  | Docker / Docker Compose                                       |
| Checks   | golangci-lint, Go tests (`testing`/`httptest`), chromedp E2E  |

## Run it

```bash
docker compose up -d --build
```

The app is then available at <http://localhost:8080>.

```bash
docker compose ps            # status
docker compose logs -f bitacora   # logs
docker compose down          # stop (data survives, it's in a volume)
```

## How data is stored

All plant data lives on the **filesystem inside a named Docker volume**
(`bitacora-data`, mounted at `/app/data`), so it survives rebuilds and clean
checkouts:

```
/app/data/
└── plants/
    └── <slug>/              # e.g. rosa-del-jardin
        ├── meta.json        # name, description, created_at
        └── images/
            ├── foto1.png    # originals, never modified
            ├── .thumb/      # generated thumbnails (JPEG)
            └── .display/    # generated display versions (JPEG)
```

The data root is configurable via the `BITACORA_DATA_DIR` environment variable
(`DATA_DIR` is also honored for backwards compatibility), defaulting to `data`.
The repo's `data/` directory stays gitignored.

## Testing

Everything runs inside Docker — no local toolchain needed. `make ci` runs the
exact same pipeline as CI: **lint + unit/integration tests + E2E**, in order.

```bash
make lint        # golangci-lint in a container
make test        # unit + integration tests (go test .)
make test-e2e    # real headless Chromium (chromedp) against the app
make ci          # lint + test + test-e2e
```

The E2E stack is fully isolated (`bitacora-e2e` project, own network/volume) so
it never touches a running app on port 8080.

## Project structure

```
bitacora-plantas/
├── main.go                 # application + storage + image pipeline
├── main_test.go            # unit tests
├── main_integration_test.go # integration tests (httptest, temp data dir)
├── main_integration2_test.go
├── e2e/                    # chromedp end-to-end tests (build tag: e2e)
│   └── e2e_test.go
├── templates/              # index, plant grid, plant detail
├── static/                 # CSS
├── Dockerfile              # production image
├── Dockerfile.test         # multi-stage test toolchain (lint / e2e)
├── docker-compose.yml      # app stack
├── docker-compose.e2e.yml  # isolated E2E stack
├── Makefile                # all checks run inside Docker
├── .golangci.yml           # linter configuration
├── .github/workflows/ci.yml # CI pipeline (runs make ci)
└── doc/                    # Spanish user guide + task/plan documents
```

## Roadmap

- Deploy the single static binary to a lightweight host (e.g. a Raspberry Pi).
- Import/export of a plant journal (backup-friendly formats).
- More collage shapes and photo captions.

See the planning docs in [`doc/`](doc/) (e.g. [`doc/PLAN-AWS.md`](doc/PLAN-AWS.md)).