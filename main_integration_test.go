package main

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestServer starts the real application handler against an ephemeral data
// dir (t.Setenv + t.TempDir) and returns the server plus the data dir path.
func newTestServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	dir := setupDataDir(t)

	var err error
	templates, err = loadTemplates()
	if err != nil {
		t.Fatalf("loading templates: %v", err)
	}

	srv := httptest.NewServer(newMux())
	t.Cleanup(srv.Close)
	return srv, dir
}

func closeBody(resp *http.Response) {
	_ = resp.Body.Close()
}

func bodyString(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer closeBody(resp)
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	return string(b)
}

func get(t *testing.T, url string) *http.Response {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	return resp
}

func do(t *testing.T, method, url string, body io.Reader, contentType string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("building %s %s: %v", method, url, err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return resp
}

type uploadFile struct {
	field   string
	name    string
	content []byte
}

func multipartBody(t *testing.T, fields map[string]string, files []uploadFile) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range files {
		fw, err := mw.CreateFormFile(f.field, f.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(f.content); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, mw.FormDataContentType()
}

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