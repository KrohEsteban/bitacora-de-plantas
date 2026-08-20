# Bitácora de Plantas — Task 6: circular overlapping collage (collage circular)

The app exists in this directory (Go + HTMX v4 + Alpine.js, filesystem storage under data/plants/<slug>/). Work ONLY in this directory. UI language: SPANISH. Green theme.

## Goal
Redesign the main-page plant mosaic from the current square grid-cell look to a **circular, overlapping photo collage** — the classic "collage circular": circular photos (rounded like a circle) arranged around a circle, overlapping each other with slight random rotations, like a scrapbook/PSD collage (think: Magnific/Pinterest circular collage templates). It must stay dynamic: every render, the arrangement rotates/changes.

## Design (precise)

### Layout model (server-side computed, CSS positioned)
- Container: `.plant-mosaic` becomes a square area (aspect-ratio: 1/1), `position: relative`.
- Each photo is a **circle** (`border-radius: 50%`, `object-fit: cover`), with a white/cream border (3-4px) and a soft shadow — the classic circular-collage look.
- Photos are distributed around an imaginary circle inside the container:
  - For n photos (n = 1..5, capped at 5 as today, random subset): `angle_i = startAngle + i * (360/n)` degrees, where `startAngle` is a random value 0..359 chosen per render.
  - Position: `left = 50% + R * cos(angle)` and `top = 50% + R * sin(angle)` (percentages), translated by -size/2 so the photo centers on that point.
  - Radius R ≈ 30-33% of the container.
- **One dominant photo** (random index per render): bigger than the rest (~55-60% of container vs ~40-46% for others), higher z-index, and positioned so the composition stays balanced (e.g. it can be the one at the top arc or near the center — choose what looks best).
- Each photo gets a slight random rotation (-14° to +14°) — scrapbook feel.
- z-index: dominant highest; others ordered so overlaps look natural (alternate or by size).
- Sizes: vary per photo (not all equal) — the overlap must be intentional and pretty, not a mess.
- Per-count behavior:
  - 1 photo → one centered circle (~68% of container).
  - 2 photos → two overlapping circles (e.g. offsets 180° apart, sizes 55% and 48%, dominant random).
  - 3-5 → circle distribution as above.
- No images → keep the 🌱 placeholder.

### Randomness (unchanged requirement)
- Server picks: random subset (up to 5), random dominant index, random startAngle, random per-photo rotations. So every reload → different arrangement. (Already have the pattern-index mechanism — evolve it: replace `Pattern int` with per-photo layout data.)

### Data flow (main.go)
- Replace the `Pattern` field approach with per-image layout: extend the mosaic view data so each image carries: `Angle` (float64 degrees), `SizePct` (e.g. 42), `Rotate` (float64 degrees), `Z` (int). Compute in the grid-render path with `math/rand` (or keep rand.Intn — fine). Pass to `plant-grid.html`.
- Keep thumbnails (`?thumb=1`) for performance — the mosaic uses thumbs.

### Template (plant-grid.html)
- Render each photo as `<img class="mosaic-photo" style="left:..%; top:..%; width:..%; transform: rotate(..deg); z-index:..">` inside the square `.plant-mosaic` container. Absolute positioning, centered via `translate(-50%,-50%)` combined with the rotation transform.
- Keep the overlay/title + 📷 N badge and the click→detail behavior (hx-get) unchanged.

### CSS (static/style.css)
- `.plant-mosaic`: square (aspect-ratio 1/1), position relative, background transparent or subtle radial gradient; rounded container (border-radius ~18px) so the whole collage reads as a circle-ish shape.
- `.mosaic-photo`: position absolute, border-radius 50%, object-fit cover, border 3px solid #fff (or cream), box-shadow, transition on hover (slight scale 1.03).
- Hover: whole card lifts slightly; photos scale subtly. Keep it green-themed and clean.

### NOT to change
- The detail view collage (grid with dates) stays as-is.
- Footer total size, docker volume, image pipeline (display/thumb), delete plant, edit, etc. — untouched.

## Verify (mandatory)
- `docker compose up -d --build`; fix compile errors.
- curl: create a test plant with 5 images → GET / shows 5 `<img class="mosaic-photo">` with inline `left/top/width/transform/z-index` styles inside `.plant-mosaic`; GET / again → different angle/rotate values (randomness).
- Plants with 1, 2 images → correct photo counts.
- **Headless browser check**: load the page, verify images render (naturalWidth > 0), take a screenshot. Confirm visually (if possible) that photos are circular and overlapping — the screenshot can be inspected by a vision model. Report what the arrangement looks like.
- Delete test plants (keep tomate-en-crecimiento). Container healthy. Commit.
- Report: how the arrangement logic works, test results, screenshot description.
