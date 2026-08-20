package unit

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"bitacora-plantas/internal/bitacora"
)

// setTemplatesDir points bitacora at the repo's templates/ directory. Tests
// run from their own package directory, so the relative "templates" default
// would not resolve from here.
func setTemplatesDir(t *testing.T) {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile))) // test/unit -> repo root
	bitacora.TemplatesDir = filepath.Join(repoRoot, "templates")
}

// setupDataDir points the storage globals at an ephemeral temp directory and
// restores the previous environment afterwards (t.Setenv).
func setupDataDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("BITACORA_DATA_DIR", dir)
	bitacora.InitDataDirs()
	setTemplatesDir(t)
	if err := os.MkdirAll(bitacora.PlantsDir, 0755); err != nil {
		t.Fatalf("creating plants dir: %v", err)
	}
	return dir
}

// testImage returns a small but real encoded image of the given format.
func testImage(t *testing.T, format string, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	var err error
	switch format {
	case "png":
		err = png.Encode(&buf, img)
	case "jpeg":
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	default:
		t.Fatalf("unknown test image format %q", format)
	}
	if err != nil {
		t.Fatalf("encoding test image: %v", err)
	}
	return buf.Bytes()
}

// mkdirPlantDirs creates the on-disk layout handleCreatePlant would for a
// plant: <plants>/<slug>/images/{,.thumb,.display}.
func mkdirPlantDirs(t *testing.T, slug string) {
	t.Helper()
	imgDir := filepath.Join(bitacora.PlantsDir, slug, "images")
	for _, d := range []string{".thumb", ".display"} {
		if err := os.MkdirAll(filepath.Join(imgDir, d), 0755); err != nil {
			t.Fatal(err)
		}
	}
}

// multipartHeader builds a parsed multipart.FileHeader for the given content.
func multipartHeader(t *testing.T, filename string, content []byte) *multipart.FileHeader {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("images", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("POST", "/", &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if err := req.ParseMultipartForm(1 << 20); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = req.MultipartForm.RemoveAll()
	})
	return req.MultipartForm.File["images"][0]
}

// newTestServer starts the real application handler against an ephemeral data
// dir (t.Setenv + t.TempDir) and returns the server plus the data dir path.
func newTestServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	dir := setupDataDir(t)

	if _, err := bitacora.LoadTemplates(); err != nil {
		t.Fatalf("loading templates: %v", err)
	}

	srv := httptest.NewServer(bitacora.NewMux())
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
