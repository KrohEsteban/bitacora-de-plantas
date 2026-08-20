package unit

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntegrationHomeEmptyState(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := get(t, srv.URL+"/")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", resp.StatusCode)
	}
	body := bodyString(t, resp)
	if !strings.Contains(body, `id="plant-grid"`) {
		t.Error("home missing plant grid")
	}
	if !strings.Contains(body, "empty-state") {
		t.Error("fresh install should show the empty state")
	}
	if !strings.Contains(body, "Empieza tu bitácora") {
		t.Error("empty state message missing")
	}
	if !strings.Contains(body, `id="total-size"`) {
		t.Error("home missing footer total")
	}
	if !strings.Contains(body, "0,00 MB") {
		t.Error("empty install should report 0,00 MB")
	}
}

func TestIntegrationCreatePlantWithImages(t *testing.T) {
	srv, dir := newTestServer(t)

	img := testImage(t, "png", 1600, 800)
	body, ct := multipartBody(t, map[string]string{
		"name":        "Rosa del Jardín",
		"description": "Le encanta el sol\nRiego semanal",
	}, []uploadFile{{field: "images", name: "foto.png", content: img}})

	resp := do(t, "POST", srv.URL+"/plantas", body, ct)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /plantas status = %d, want 200 (body: %s)",
			resp.StatusCode, bodyString(t, resp))
	}
	grid := bodyString(t, resp)
	if !strings.Contains(grid, `hx-get="/planta/rosa-del-jardin"`) {
		t.Errorf("grid missing new plant card: %s", grid)
	}
	if !strings.Contains(grid, "Rosa del Jardín") {
		t.Errorf("grid missing plant name: %s", grid)
	}
	if !strings.Contains(grid, `id="total-size" hx-swap-oob="true"`) {
		t.Error("create response missing OOB footer swap")
	}
	if strings.Contains(grid, "0,00 MB") {
		t.Error("footer total should have grown after uploading an image")
	}

	// Metadata persisted with the expected slug.
	metaRaw, err := os.ReadFile(filepath.Join(dir, "plants", "rosa-del-jardin", "meta.json"))
	if err != nil {
		t.Fatalf("reading meta.json: %v", err)
	}
	if !strings.Contains(string(metaRaw), `"name": "Rosa del Jardín"`) {
		t.Errorf("meta.json missing name: %s", metaRaw)
	}

	// Original + derived files exist.
	plantDir := filepath.Join(dir, "plants", "rosa-del-jardin")
	for _, rel := range []string{
		filepath.Join("images", "foto.png"),
		filepath.Join("images", ".thumb", "foto.png.jpg"),
		filepath.Join("images", ".display", "foto.png.jpg"),
	} {
		if _, err := os.Stat(filepath.Join(plantDir, rel)); err != nil {
			t.Errorf("expected file %s: %v", rel, err)
		}
	}

	// Detail view shows name, description and the gallery image.
	resp = get(t, srv.URL+"/planta/rosa-del-jardin")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET detail status = %d, want 200", resp.StatusCode)
	}
	detail := bodyString(t, resp)
	if !strings.Contains(detail, "Rosa del Jardín") || !strings.Contains(detail, "Le encanta el sol") {
		t.Errorf("detail missing plant info: %s", detail)
	}
	if !strings.Contains(detail, `src="/img/rosa-del-jardin/foto.png"`) {
		t.Errorf("detail missing gallery image: %s", detail)
	}
	if !strings.Contains(detail, "Creada el") {
		t.Error("detail missing creation date")
	}

	// Original serves with PNG content type.
	resp = get(t, srv.URL+"/img/rosa-del-jardin/foto.png?original=1")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET original status = %d, want 200", resp.StatusCode)
	}
	closeBody(resp)
	if ct := resp.Header.Get("Content-Type"); ct != "image/png" {
		t.Errorf("original Content-Type = %q, want image/png", ct)
	}

	// Derived versions are real JPEGs.
	for _, q := range []string{"?thumb=1", ""} {
		resp = get(t, srv.URL+"/img/rosa-del-jardin/foto.png"+q)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %q status = %d, want 200", q, resp.StatusCode)
		}
		closeBody(resp)
		if ct := resp.Header.Get("Content-Type"); ct != "image/jpeg" {
			t.Errorf("derived (%q) Content-Type = %q, want image/jpeg", q, ct)
		}
	}

	// thumb takes precedence when both flags are set.
	resp = get(t, srv.URL+"/img/rosa-del-jardin/foto.png?thumb=1&original=1")
	closeBody(resp)
	if ct := resp.Header.Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("thumb|original Content-Type = %q, want image/jpeg", ct)
	}
}

func TestIntegrationLazyDerivedRegeneration(t *testing.T) {
	srv, dir := newTestServer(t)

	img := testImage(t, "png", 800, 600)
	body, ct := multipartBody(t, map[string]string{"name": "Aloe"}, []uploadFile{{field: "images", name: "a.png", content: img}})
	resp := do(t, "POST", srv.URL+"/plantas", body, ct)
	closeBody(resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /plantas status = %d, want 200", resp.StatusCode)
	}

	thumbPath := filepath.Join(dir, "plants", "aloe", "images", ".thumb", "a.png.jpg")
	if err := os.Remove(thumbPath); err != nil {
		t.Fatal(err)
	}

	resp = get(t, srv.URL+"/img/aloe/a.png?thumb=1")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET thumb after removal status = %d, want 200", resp.StatusCode)
	}
	closeBody(resp)
	if _, err := os.Stat(thumbPath); err != nil {
		t.Errorf("thumb not regenerated: %v", err)
	}
}

func TestIntegrationUniqueSlug(t *testing.T) {
	srv, _ := newTestServer(t)

	for _, name := range []string{"Rosa", "Rosa"} {
		body, ct := multipartBody(t, map[string]string{"name": name}, nil)
		resp := do(t, "POST", srv.URL+"/plantas", body, ct)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("create %q status = %d, want 200", name, resp.StatusCode)
		}
		bodyString(t, resp)
	}

	// Second Rosa must become rosa-1.
	resp := get(t, srv.URL+"/planta/rosa-1")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET rosa-1 status = %d, want 200", resp.StatusCode)
	}
	detail := bodyString(t, resp)
	if !strings.Contains(detail, "Rosa") {
		t.Errorf("detail for rosa-1 missing: %s", detail)
	}
}

func TestIntegrationEditPlant(t *testing.T) {
	srv, dir := newTestServer(t)

	body, ct := multipartBody(t, map[string]string{"name": "Tulipán"}, nil)
	resp := do(t, "POST", srv.URL+"/plantas", body, ct)
	closeBody(resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create status = %d", resp.StatusCode)
	}

	form := url.Values{"name": {"Tulipán de Otoño"}, "description": {"Nueva descripción con acentos: ñ, á"}}
	resp = do(t, "PUT", srv.URL+"/planta/tulipan", strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200 (body: %s)", resp.StatusCode, bodyString(t, resp))
	}
	detail := bodyString(t, resp)
	if !strings.Contains(detail, "Tulipán de Otoño") || !strings.Contains(detail, "Nueva descripción con acentos: ñ, á") {
		t.Errorf("updated detail missing new values: %s", detail)
	}

	metaRaw, err := os.ReadFile(filepath.Join(dir, "plants", "tulipan", "meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(metaRaw), "Tulipán de Otoño") {
		t.Errorf("meta.json not updated: %s", metaRaw)
	}
}

func TestIntegrationAddAndDeleteImages(t *testing.T) {
	srv, dir := newTestServer(t)

	body, ct := multipartBody(t, map[string]string{"name": "Hortensia"}, []uploadFile{{field: "images", name: "flor1.png", content: testImage(t, "png", 200, 200)}})
	resp := do(t, "POST", srv.URL+"/plantas", body, ct)
	closeBody(resp)

	// Add a second image.
	body, ct = multipartBody(t, nil, []uploadFile{{field: "images", name: "flor2.png", content: testImage(t, "png", 300, 200)}})
	resp = do(t, "POST", srv.URL+"/planta/hortensia/imagenes", body, ct)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("add image status = %d, want 200", resp.StatusCode)
	}
	detail := bodyString(t, resp)
	if !strings.Contains(detail, `src="/img/hortensia/flor2.png"`) {
		t.Errorf("detail missing newly added image: %s", detail)
	}

	// Delete one image; its original and derived files must vanish.
	resp = do(t, "DELETE", srv.URL+"/planta/hortensia/imagenes/flor1.png", nil, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete image status = %d, want 200", resp.StatusCode)
	}
	closeBody(resp)
	imgDir := filepath.Join(dir, "plants", "hortensia", "images")
	if _, err := os.Stat(filepath.Join(imgDir, "flor1.png")); !os.IsNotExist(err) {
		t.Errorf("flor1.png still present after delete")
	}
	if _, err := os.Stat(filepath.Join(imgDir, ".thumb", "flor1.png.jpg")); !os.IsNotExist(err) {
		t.Errorf("flor1 thumbnail still present after delete")
	}

	detail = bodyString(t, get(t, srv.URL+"/planta/hortensia"))
	if strings.Contains(detail, "flor1") {
		t.Error("detail still references deleted image")
	}
	if !strings.Contains(detail, "flor2") {
		t.Error("detail missing remaining image")
	}
}

func TestIntegrationDeletePlant(t *testing.T) {
	srv, dir := newTestServer(t)

	body, ct := multipartBody(t, map[string]string{"name": "Cactus"}, nil)
	resp := do(t, "POST", srv.URL+"/plantas", body, ct)
	closeBody(resp)

	resp = do(t, "DELETE", srv.URL+"/planta/cactus", nil, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete plant status = %d, want 200", resp.StatusCode)
	}
	grid := bodyString(t, resp)
	if !strings.Contains(grid, "empty-state") {
		t.Errorf("grid should be empty after deleting the only plant: %s", grid)
	}

	if _, err := os.Stat(filepath.Join(dir, "plants", "cactus")); !os.IsNotExist(err) {
		t.Error("plant directory still present after delete")
	}

	// Detail of the deleted plant is 404.
	resp = get(t, srv.URL+"/planta/cactus")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET deleted plant status = %d, want 404", resp.StatusCode)
	}
	closeBody(resp)
}

func TestIntegrationNotFound(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := get(t, srv.URL+"/planta/no-existe")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET missing plant status = %d, want 404", resp.StatusCode)
	}
	closeBody(resp)

	resp = do(t, "DELETE", srv.URL+"/planta/no-existe", nil, "")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("DELETE missing plant status = %d, want 404", resp.StatusCode)
	}
	closeBody(resp)

	resp = do(t, "DELETE", srv.URL+"/planta/no-existe/imagenes/foto.png", nil, "")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("DELETE image on missing plant status = %d, want 404", resp.StatusCode)
	}
	closeBody(resp)

	resp = get(t, srv.URL+"/img/no-existe/foto.png")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET missing image status = %d, want 404", resp.StatusCode)
	}
	closeBody(resp)
}

func TestIntegrationValidation(t *testing.T) {
	srv, _ := newTestServer(t)

	// Empty name is rejected.
	body, ct := multipartBody(t, map[string]string{"name": "   "}, nil)
	resp := do(t, "POST", srv.URL+"/plantas", body, ct)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("POST empty name status = %d, want 400", resp.StatusCode)
	}
	closeBody(resp)

	// Non-image uploads are skipped but the plant is still created.
	body, ct = multipartBody(t, map[string]string{"name": "Solo Notas"},
		[]uploadFile{{field: "images", name: "notas.txt", content: []byte("apuntes de riego")}})
	resp = do(t, "POST", srv.URL+"/plantas", body, ct)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST with non-image status = %d, want 200", resp.StatusCode)
	}
	grid := bodyString(t, resp)
	if !strings.Contains(grid, `hx-get="/planta/solo-notas"`) {
		t.Errorf("plant should be created despite invalid image: %s", grid)
	}

	detail := bodyString(t, get(t, srv.URL+"/planta/solo-notas"))
	if !strings.Contains(detail, "empty-gallery") {
		t.Errorf("detail should show empty gallery: %s", detail)
	}
}
