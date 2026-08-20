# Bitácora de Plantas — Task 5: robust image pipeline + footer total size + docker volume

The app exists in this directory (Go + HTMX v4 + Alpine.js, filesystem storage under data/plants/<slug>/). Work ONLY in this directory. UI language: SPANISH. Green theme.

## Context: known bug (already diagnosed)
AI-generated images from OpenRouter are JPEG data (magic bytes ff d8 ff e0) but get saved with a .png extension. `createThumbnail` decodes by file EXTENSION (png.Decode on JPEG data) → fails with "png: invalid format: not a PNG file" → no thumbnails → mosaic images 404. Logs show this error for all 4 images of the existing "tomate-en-crecimiento" plant.

## Requirements

### 1. Robust image pipeline (fix + improve) — main.go
- **Decode by CONTENT, never by extension**: read the first 512 bytes, `http.DetectContentType`, then decode with the matching decoder (jpeg/png/webp). If content type says JPEG but extension is .png, decode as JPEG.
- Generate TWO derived versions per upload (always saved as REAL JPEG with .jpg extension, quality ~82):
  - **Display version** → `images/.display/<base>.jpg`, max 1200px on the long side — used by the detail collage and as the default served image.
  - **Thumbnail** → `images/.thumb/<base>.jpg`, max 400px — used by the main-page mosaic.
- Keep the ORIGINAL file untouched in `images/` (full resolution).
- Serve via query params on `/img/{slug}/{filename}`:
  - `?thumb=1` → serve the thumbnail
  - default (no param) → serve the display version
  - `?original=1` → serve the original
- **Lazy generation**: if the requested derived version doesn't exist (e.g. legacy plants uploaded before this fix), generate it on the fly from the original and save it, then serve. This automatically fixes the existing "tomate-en-crecimiento" plant without re-uploading. Use a package-level mutex around generation to avoid races.
- **Serve with correct Content-Type based on the actual bytes of the file being served** (detect at serve time), NOT the file extension — legacy files (JPEG content with .png name) must render fine.
- Templates: plant-grid mosaic imgs → `?thumb=1`; plant-detail collage imgs → default (display). Update `Image` struct usage if needed (URL stays the same; query params decide the version).

### 2. Footer with total image weight — templates/index.html + main.go
- Add a footer bar at the bottom of the page (green, consistent with theme): `📦 Peso total de imágenes: X,XX MB`.
- The total = sum of sizes (bytes) of ALL image files under `data/plants/**` (originals + derived versions). Compute in Go (walk data/plants, sum files that are images or inside images/ dirs).
- Implementation: 
  - On `GET /` (handleHome) compute the total and pass it to the template.
  - Give the footer span `id="total-size"` in index.html.
  - EVERY mutating response (create plant, add images, delete plant, delete image, edit — the grid/detail partials) must ALSO include the updated total as an out-of-band swap so the footer updates without a page reload: `<span id="total-size" hx-swap-oob="true">📦 Peso total de imágenes: X,XX MB</span>`. (HTMX v4 out-of-band swap — verify the correct attribute/behavior in v4 and use it; if OOB needs the element inside the returned HTML, place it at the end of the partial.)

### 3. Docker named volume for data — docker-compose.yml + migration
- Change the service to use a **named Docker volume** instead of the host bind mount: define `volumes: bitacora-data:` at the compose top level and mount `bitacora-data:/app/data` on the service. This guarantees images survive redeploys/clean checkouts (the volume lives in Docker's storage, independent of the project directory).
- **Migrate existing data** before recreating the container: copy the current `./data/` content into the volume, e.g.:
  `docker run --rm -v bitacora-data:/app/data -v $PWD/data:/from alpine sh -c 'cp -a /from/. /app/data/'`
  (create the volume first if needed). Then `docker compose up -d --build` recreates the container with the volume.
- Verify the migration worked (the tomate-en-crecimiento plant + its images are present after recreate).
- Update README.md: data lives in the named volume; how to back it up (`docker run --rm -v bitacora-data:/app/data -v $PWD:/backup alpine tar czf /backup/bitacora-data.tar.gz -C /app/data .`); note that `./data` on the host is no longer used (gitignore stays).

### 4. Verify (mandatory)
- `docker compose up -d --build`; fix compile errors.
- After migration: confirm tomate-en-crecimiento exists with its 4 originals.
- curl tests:
  a. GET / → footer shows the total; GET the same img URL twice → derived version generated on first request (check .thumb/.display dirs get populated for the EXISTING plant — lazy fix works).
  b. Upload a NEW plant with images → .display and .thumb .jpg files created; total size in footer increased (OOB span present in the response).
  c. DELETE an image/plant → footer total decreases (OOB present).
  d. Verify served content-type is correct for a legacy JPEG-in-.png file (should be image/jpeg) and for a real PNG upload (image/png original).
- **Browser check**: load the page in a headless browser and verify mosaic images have naturalWidth > 0 (they actually render now), and the footer shows the total. Use `curl` for HTML checks and a headless browser/JS check if available (the project has used headless Chromium before — reuse that approach).
- Clean test data (keep tomate-en-crecimiento). Container healthy. Commit with a clear message.
- Report: changes, migration steps run, test results, container status, and the root cause explanation.
