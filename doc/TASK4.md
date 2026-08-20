# Bitácora de Plantas — Task 4: irregular dynamic mosaic collage on the main page

The app exists in this directory (Go + HTMX v4 + Alpine.js, filesystem storage under data/plants/<slug>/). Work ONLY in this directory. UI language: SPANISH. Keep the green theme.

## Goal
Replace the current single-thumbnail plant card on the MAIN page with an **irregular photo-collage mosaic** per plant (like a photo collage: one central/big photo surrounded by smaller ones). It must be **dynamic**: every time the page (grid) is rendered, both the layout pattern AND the photos change randomly.

## Requirements

### 1. Mosaic per plant card (main grid)
- Each plant card shows an irregular mosaic of **up to 5 photos**:
  - Plant has 1 photo → mosaic shows 1 (full-card image).
  - 2 photos → shows 2 in an asymmetric layout.
  - 3-4 photos → shows 3-4 in an irregular layout.
  - 5+ photos → shows exactly 5 (random subset).
- Layouts must be IRREGULAR (asymmetric): e.g. one large photo + smaller ones around/beside it, varied aspect ratios via CSS grid `grid-template-areas`. Define at least 4-5 distinct pattern variants (different template-areas) for the 5-photo case and a couple of variants each for 2/3/4 photos. The 5-photo pattern should resemble a collage with a dominant center/left photo surrounded by the rest.
- **Random on every render**: the server picks (per plant, per grid render):
  - a random subset of up to 5 images (shuffle the plant's images, take the first 5) — so reloads show different photos,
  - a random pattern variant (pattern index) — so reloads show different layouts.
  Use `math/rand` seeded per request (or rand.Intn — fine for this app) in the Go handler(s) that render the grid (`handleHome`, `handleCreatePlant`, `handleDeletePlant` — anything that renders `plant-grid.html`).
- Photos use the existing thumbnails (`?thumb=1`) for performance.

### 2. Data flow (main.go)
- Add fields to the Plant struct (or a view struct): `Mosaic []Image` (0-5 picked images, already with URLs) and `Pattern int` (the pattern index to use). Populate them in the grid-render path: after loading plants, for each plant shuffle images, cap at 5, pick pattern. Keep the existing `Images []Image` (all images) for the detail view.
- The detail view (`plant-detail.html`) stays as is (full collage with dates + edit/add/delete).

### 3. Template (plant-grid.html)
- Render the mosaic: `<div class="plant-mosaic pattern-{{.Pattern}}">` with `<img>` children (thumbnails). Title overlay or below: keep the plant name clearly visible (e.g. a soft gradient overlay at the bottom with the name + photo count badge 📸 N). Keep the card's click behavior: `hx-get="/planta/{{.Slug}}"` → detail view (unchanged).
- If a plant has no images, keep the current 🌱 placeholder.

### 4. CSS (static/style.css)
- `.plant-mosaic` base: display grid, gap ~3px, border-radius consistent with theme, overflow hidden, fixed card aspect ratio (e.g. 4:3 or square-ish).
- Pattern classes with `grid-template-areas` for each image count × variant, e.g.:
  - 1 image: `"a"` full.
  - 2 images: `"a b"` / `"a a" "a b"` variants.
  - 3 images: big + two stacked, 2-3 variants.
  - 4 images: big left + 3 right (2x2), variants.
  - 5 images: dominant center photo + 4 around (3x3-ish), or dominant left + 4 right — at least 2 variants.
- Each `img` fills its grid area with `object-fit: cover; width/height 100%`.
- Hover effect on the card (subtle zoom on images / lift), gradient overlay with the plant name at the bottom, small transitions. All consistent with the green theme.
- Guard: if the pattern index for a given photo count doesn't exist, fall back gracefully (CSS should still look OK; the server should clamp the pattern index to the variants available for that count).

### 5. Verify (mandatory)
- `docker compose up -d --build`. Fix compile errors.
- curl tests:
  a. Create a plant with 6+ images → GET / returns a grid where the card shows exactly 5 images in a mosaic (check the HTML: 5 img tags inside `.plant-mosaic`, and the pattern class exists in CSS).
  b. GET / twice → the mosaic img set and/or pattern class differ between loads (randomness works).
  c. Create a plant with 2 images → card shows 2. With 0 → placeholder.
  d. Click flow: GET /planta/<slug> still returns the full detail (description + all images + buttons).
  e. DELETE the test plants → clean state.
- Clean all test data (`docker exec <container> rm -rf /app/data/plants/*` if root-owned).
- Container healthy + home 200. Commit with a clear message.
- Report: patterns designed (describe them), test results, container status.
