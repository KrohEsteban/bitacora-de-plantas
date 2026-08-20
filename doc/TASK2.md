# Bitácora de Plantas — Task 2: collage con fechas + eliminar planta

The app exists in this directory (Go + HTMX v4 + Alpine.js, filesystem storage under data/plants/<slug>/). Work ONLY in this directory. UI language: SPANISH. Keep the existing green theme and design language.

## Changes required

### 1. DELETE plant (new)
- Add route `DELETE /planta/{slug}` in main.go.
- Handler: validate slug (no path traversal, must exist under data/plants/), delete the ENTIRE plant folder (meta.json + images/ + .thumb/ recursively). 404 with Spanish error if the plant doesn't exist. 500 on failure.
- In the detail view (`templates/plant-detail.html`): add a **"Eliminar planta"** button (red/danger style, consistent with theme). It opens an **Alpine.js confirmation modal** showing the plant name ("¿Eliminar la planta 'X'? Se borrarán todas sus imágenes.") with "Cancelar" and "Eliminar" buttons. Confirm → HTMX `hx-delete="/planta/{slug}"` targeting the main content container, with `hx-push-url="/"`, and the server responds with the updated plant grid partial (`plant-grid.html`) so the user lands back on the collage. If the grid would be empty, return the empty-state (whatever index.html shows when there are no plants — check how handleHome/index renders the empty state and reuse it).

### 2. Collage with growth dates (improve)
- The detail view shows all images of a plant. Make it a real **collage**: responsive masonry/grid of images where each image card shows a small **date label** (the upload date, format DD/MM/YYYY) so the user can watch the plant grow over time.
- Implementation: introduce an `Image` struct in Go: `{ Name string; URL string; ModTime time.Time }` (URL = "/img/<slug>/<name>"). When loading a plant's images, stat each file (os.Stat) to get ModTime, sort images **oldest first** (chronological = growth timeline). Exclude the `.thumb` subdirectory from the listing.
- Update `Plant.Images` to `[]Image` (or add a field) and update templates accordingly (plant-grid.html only needs the first image thumbnail; plant-detail.html renders the collage with date labels).
- Keep the delete-image button (🗑) on each image card in the collage.

### 3. Many images at once (raise limits)
- Raise `maxUploadSize` from 10MB to **50MB** (total request body) so the user can upload many photos in one go.
- In the "Agregar imágenes" modal, keep `multiple` on the file input and add a small hint: "Podés seleccionar varias imágenes a la vez" and show the count of selected files with Alpine (`x-text` on the input's change event) — nice dynamic touch.

### 4. Verify (mandatory)
- Rebuild: `docker compose up -d --build` (or `docker build -t bitacora-plantas .` then restart the container). Fix all compile errors.
- Test with curl:
  a. Create a plant with **3 images** in ONE request (multipart, multiple files) → verify all 3 land in data/plants/<slug>/images/ with thumbnails.
  b. GET the detail partial → verify the collage shows 3 images with date labels.
  c. Add 2 more images in a second request → verify 5 total, ordered oldest-first.
  d. DELETE the plant → verify the folder is gone and the response is the grid partial.
- Clean up ALL test data afterwards (use `docker exec <container> rm -rf /app/data/plants/*` if needed — files are root-owned).
- Commit with a clear message.
- Report: what changed, test results, and confirm the container is healthy.
