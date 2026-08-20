# Bitácora de Plantas — Build Task

Build a complete plant journal web app in the current directory (a fresh git repo). Work ONLY in this directory. The app is called "Bitácora de Plantas" and the UI language must be SPANISH.

## 0. RESEARCH FIRST (mandatory)
Investigate how to use **HTMX version 4** before writing code: read the official docs at https://htmx.org/docs/ and the v4 upgrade/release notes (https://htmx.org/essays/ or the changelog) to learn the v4 syntax and breaking changes vs v3. Then use HTMX **v4** correctly (load it from the v4 CDN). Note any renamed/changed attributes (e.g. hx-on handler syntax, removed deprecated features) and use the v4 way. Mention in your final report which v4-specific things you applied.

## 1. Tech stack
- Backend: Go, standard library only (`net/http` with Go 1.22+ method routing patterns, `html/template`). No web framework. Minimal extra deps: `golang.org/x/image` is allowed for thumbnail scaling.
- Frontend: HTMX v4 (CDN) + Alpine.js 3 (CDN) + custom CSS (no UI framework).
- No database: the filesystem is the source of truth.

## 2. Data model (filesystem-based)
- Each plant = a folder: `data/plants/<slug>/` where `<slug>` is a sanitized version of the plant name (lowercase, accents removed, spaces → hyphens; if the slug exists, append a numeric suffix).
- Inside the folder:
  - `meta.json` — `{ "name": ..., "description": ..., "created_at": ... }`. The description holds care info (how often to water, frost behavior, temperature needs, etc.) — free text.
  - `images/` — the uploaded original images (jpg/png/webp).
  - `images/.thumb/` — generated thumbnails (max ~400px on the long side).
- Creating a plant creates its folder; uploading images writes files to `images/`.

## 3. Routes (single-page app)
- `GET /` — the ONLY full page: green collage grid with one card per plant (thumbnail + plant name). Cards in a responsive grid.
- `GET /planta/{slug}` — HTMX partial: full detail view of one plant: all images, title, description, and action buttons (edit, add images, delete image per image).
- `POST /plantas` — create plant (multipart form: name, description, optional multiple images) → returns updated collage (HTMX swap) or redirects to `/`.
- `PUT /planta/{slug}` — edit plant meta (name, description) → returns updated detail partial.
- `POST /planta/{slug}/imagenes` — add images (multipart, multiple files) → returns updated detail partial.
- `DELETE /planta/{slug}/imagenes/{filename}` — delete one image → returns updated detail partial.
- `GET /img/{slug}/{filename}` — serve an image from that plant's images folder (support a `?thumb=1` query to serve the thumbnail).

## 4. Features
- "Nueva planta" button → Alpine modal with form (nombre, descripción, seleccionar imágenes opcional).
- Clicking a plant card loads the detail view via HTMX (hx-get into a detail container; the collage fades out / the detail slides in — dynamic feel).
- Detail view: all images as a gallery, title, description, "Editar" button (Alpine modal, name+description form, PUT), "Agregar imágenes" button (Alpine modal with multiple file picker), and a small "🗑" button on each image (DELETE).
- After create/edit/upload/delete, the UI updates via HTMX partial swaps — no full page reloads.
- Thumbnails generated at upload time.

## 5. Validation & security
- Max upload: 10 MB per image. Allowed types: image/jpeg, image/png, image/webp (check content type AND sniff magic bytes with http.DetectContentType).
- Sanitize all filenames: keep only safe characters, preserve extension, reject `..` and path traversal; use filepath.Base and clean paths; never let user input escape the plant folder.
- Sanitize the slug for folder names.
- 404 (with a friendly Spanish page/partial) for missing plants or images.
- Reasonable request body size limits (http.MaxBytesReader).

## 6. Design
- Beautiful green theme: leafy green palette (e.g. deep greens for headers, fresh leaf green accents), warm cream/off-white background, rounded cards with soft shadows, subtle hover lift animations on cards, smooth transitions for HTMX swaps (fade/slide), Alpine modals with fade. Responsive (mobile + desktop). Modern, dynamic, "vivo" feel. UI text 100% in Spanish.

## 7. Docker
- `Dockerfile`: multi-stage — builder `golang:1.24-alpine` (or 1.23 if 1.24 unavailable in the image registry used), runtime a small image (alpine + the compiled binary + static/templates copied in). Port 8080. ENV for data dir.
- `docker-compose.yml`: service `bitacora`, build ., ports "8080:8080", volume `./data:/app/data`, healthcheck (curl or wget to /), restart: unless-stopped.
- `README.md` (in Spanish): what it is, how to run (`docker compose up -d`), where data lives.
- `.dockerignore` and `.gitignore` (ignore data/, the binary, .git, etc.).

## 8. Test & verify (mandatory)
- Go is NOT installed locally in this environment. Validate by building the Docker image: `docker build -t bitacora-plantas .` and fix every compile error until it builds clean.
- Then run it: `docker compose up -d` (or `docker run -p 8080:8080 -v $PWD/data:/app/data bitacora-plantas`), wait for health, and verify with `curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/` → expect 200.
- Create one test plant via curl (POST multipart with a tiny generated image) to prove the upload path works, then delete the test data folder so the repo ships clean.
- Report: files created, build/test results, which HTMX v4 features you used, and how to run it.
- Commit everything to git at the end with a clear commit message.
