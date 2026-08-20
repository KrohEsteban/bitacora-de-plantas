# Bitácora de Plantas — Task 3: fix modals (creando...) + verify plant folder flow

The app exists in this directory (Go + HTMX v4 + Alpine.js, filesystem storage under data/plants/<slug>/). Work ONLY in this directory. UI language: SPANISH. Keep the green theme.

## Bug to fix: modals get stuck during uploads

Current behavior: the create-plant modal (and add-images modal) only closes AFTER the server finishes (form has `hx-on:htmx:after-swap="showCreateModal = false; ..."`). With image uploads + thumbnail generation the request takes a while, so the modal looks frozen ("clavado").

Fix in `templates/index.html`:
1. **Create modal** (`hx-post="/plantas"`):
   - Close the modal IMMEDIATELY when the request starts: add `hx-on:htmx:before-request="showCreateModal = false"`.
   - Add `hx-indicator="#creating-indicator"` to the form.
   - Keep `hx-on:htmx:after-swap="$el.reset()"` (only reset the form; modal already closed).
2. **Add-images modal** (`hx-post="/planta/{slug}/imagenes"`): same pattern — close on `htmx:before-request`, `hx-indicator="#uploading-indicator"`, keep reset on after-swap.
3. **Edit modal** (`hx-put`): same pattern — close on before-request, `hx-indicator="#saving-indicator"`.
4. Add a single **fixed toast indicator** (bottom-center, dark green pill, spinner + text) that HTMX shows during requests:
   - One element with `id="creating-indicator"` → text "⏳ Creando planta…"
   - One with `id="uploading-indicator"` → text "⏳ Subiendo imágenes…"
   - One with `id="saving-indicator"` → text "⏳ Guardando cambios…"
   - All three have class `htmx-indicator` (hidden by default via CSS: `.htmx-indicator { display: none; }` and shown while request runs: `.htmx-request.htmx-indicator, .htmx-indicator.htmx-request { display: flex; }` — add the CSS to `static/style.css` if not already present). Position them fixed bottom-center with a subtle fade, consistent with the green theme.
   - Optional nice touch: also disable the submit button while the request runs (form gets `htmx-request` class — style `.htmx-request .btn[type=submit] { opacity: .6; pointer-events: none; }`).

## Feature to VERIFY (should already work — confirm and fix if broken)

The user wants: from the main collage, clicking a plant card → enter the plant's "folder" (detail view) showing the description AND all its photos, with the edit button available.

- Verify `plant-grid.html` card `hx-get="/planta/{slug}"` → `#detail-container` flow: the detail view must show the full description and ALL images (collage with dates) + "Editar" + "Agregar imágenes" + "Eliminar Planta" buttons.
- If anything in that flow is broken (e.g. description not rendering, images missing, edit modal not opening with the right values), fix it.
- Note: the edit modal's textarea uses `x-text` to fill the description — check that the value actually submits correctly (if `x-text` prevents proper form submission of the textarea, switch to setting `value` via `x-bind:value` or Alpine data model). Make sure editing a plant's description works end to end.

## Verify (mandatory)
- `docker compose up -d --build` (or docker build + restart). Fix compile errors.
- curl tests:
  a. Create a plant with 2 images → 200, grid partial returned.
  b. GET /planta/<slug> → detail contains description + both images.
  c. PUT edit with new description → 200, detail shows the new description.
  d. DELETE the test plant → 200.
- Clean all test data afterwards (`docker exec <container> rm -rf /app/data/plants/*` if root-owned).
- Confirm container healthy + home 200.
- Commit with a clear message.
- Report: changes made, test results, container status.
