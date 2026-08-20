package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// setupDataDir points the storage globals at an ephemeral temp directory and
// restores the previous environment afterwards (t.Setenv).
func setupDataDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("BITACORA_DATA_DIR", dir)
	initDataDirs()
	if err := os.MkdirAll(plantsDir, 0755); err != nil {
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

func TestCreateSlug(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"Mi Planta", "mi-planta"},
		{"Rosal del Jardín", "rosal-del-jardin"},
		{"El Ñandú", "el-nandu"},
		{"  Hola   Mundo  ", "hola-mundo"},
		{"Hola!", "hola"},
		{"CAFÉ con LECHE", "cafe-con-leche"},
		{"a_b-c.d", "a-b-c-d"},
		{"más de un---guión--", "mas-de-un-guion"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := createSlug(tt.name); got != tt.want {
				t.Errorf("createSlug(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestRemoveAccents(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"áàäâãéèëêíìïîóòöôõúùüûñç", "aaaaaeeeeiiiiooooouuuunc"},
		{"no acentos", "no acentos"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := removeAccents(tt.in); got != tt.want {
			t.Errorf("removeAccents(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"foto.png", "foto.png"},
		{"../evil.png", "evil.png"},
		{"a b c.png", "a_b_c.png"},
		{"weird*name?.jpg", "weird_name_.jpg"},
		{"/etc/passwd", "passwd"},
		{"solo.jpg", "solo.jpg"},
		{".", ""},
		{"..", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := sanitizeFilename(tt.in); got != tt.want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestDerivedName(t *testing.T) {
	tests := map[string]string{
		"foto.png": "foto.png.jpg",
		"a.jpg":    "a.jpg.jpg",
		"sin-ext":  "sin-ext.jpg",
	}
	for in, want := range tests {
		if got := derivedName(in); got != want {
			t.Errorf("derivedName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsValidImageType(t *testing.T) {
	valid := []string{"image/jpeg", "image/png", "image/webp"}
	for _, ct := range valid {
		if !isValidImageType(ct) {
			t.Errorf("isValidImageType(%q) = false, want true", ct)
		}
	}
	invalid := []string{"text/plain", "application/pdf", "image/gif", ""}
	for _, ct := range invalid {
		if isValidImageType(ct) {
			t.Errorf("isValidImageType(%q) = true, want false", ct)
		}
	}
}

func TestIsImageFile(t *testing.T) {
	tests := map[string]bool{
		"foto.jpg":  true,
		"foto.jpeg": true,
		"foto.png":  true,
		"foto.webp": true,
		"FOTO.PNG":  true,
		"foto.txt":  false,
		"foto":      false,
		".hidden":   false,
	}
	for name, want := range tests {
		if got := isImageFile(name); got != want {
			t.Errorf("isImageFile(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestFormatMB(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0,00"},
		{1 << 20, "1,00"},
		{3 * 1024 * 1024 / 2, "1,50"},
		{1048576 + 524288, "1,50"},
	}
	for _, tt := range tests {
		if got := formatMB(tt.in); got != tt.want {
			t.Errorf("formatMB(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFormatDates(t *testing.T) {
	ts := time.Date(2026, time.August, 19, 23, 30, 0, 0, time.UTC)
	if got := formatDate(ts); got != "19 de August de 2026" {
		t.Errorf("formatDate = %q", got)
	}
	if got := formatDateShort(ts); got != "19/08/2026" {
		t.Errorf("formatDateShort = %q", got)
	}
}

func TestSizeLabelEmpty(t *testing.T) {
	setupDataDir(t)
	if got, want := sizeLabel(), "📦 Peso total de imágenes: 0,00 MB"; got != want {
		t.Errorf("sizeLabel() = %q, want %q", got, want)
	}
}

func TestEnsureUniqueFilename(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"foto.png", "a.jpg"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		in, want string
	}{
		{"foto.png", "foto_1.png"},
		{"a.jpg", "a_1.jpg"},
		{"nuevo.webp", "nuevo.webp"},
	}
	for _, tt := range tests {
		if got := ensureUniqueFilename(dir, tt.in); got != tt.want {
			t.Errorf("ensureUniqueFilename(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}

	// Now that foto_1.png exists, it must be bumped to foto_1_1.png.
	if err := os.WriteFile(filepath.Join(dir, "foto_1.png"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := ensureUniqueFilename(dir, "foto_1.png"); got != "foto_1_1.png" {
		t.Errorf("ensureUniqueFilename(foto_1.png) = %q, want foto_1_1.png", got)
	}
}

func TestEnsureUniqueSlug(t *testing.T) {
	setupDataDir(t)
	if err := os.MkdirAll(filepath.Join(plantsDir, "rosa"), 0755); err != nil {
		t.Fatal(err)
	}
	if got := ensureUniqueSlug("rosa"); got != "rosa-1" {
		t.Errorf("ensureUniqueSlug(rosa) = %q, want rosa-1", got)
	}
	if got := ensureUniqueSlug("tulipan"); got != "tulipan" {
		t.Errorf("ensureUniqueSlug(tulipan) = %q, want tulipan", got)
	}
}

// mkdirPlantDirs creates the on-disk layout handleCreatePlant would for a
// plant: <plants>/<slug>/images/{,.thumb,.display}.
func mkdirPlantDirs(t *testing.T, slug string) {
	t.Helper()
	imgDir := filepath.Join(plantsDir, slug, "images")
	for _, d := range []string{".thumb", ".display"} {
		if err := os.MkdirAll(filepath.Join(imgDir, d), 0755); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPlantStorage(t *testing.T) {
	setupDataDir(t)

	now := time.Now()
	first := Plant{Name: "Rosa", Description: "primera", CreatedAt: now.Add(-time.Hour), Slug: "rosa"}
	mkdirPlantDirs(t, "rosa")
	if err := savePlantMeta("rosa", first); err != nil {
		t.Fatalf("savePlantMeta: %v", err)
	}
	second := Plant{Name: "Tulipán", Description: "segunda", CreatedAt: now, Slug: "tulipan"}
	mkdirPlantDirs(t, "tulipan")
	if err := savePlantMeta("tulipan", second); err != nil {
		t.Fatalf("savePlantMeta: %v", err)
	}

	// Loaded plant round-trips the metadata.
	got, err := loadPlant("rosa")
	if err != nil {
		t.Fatalf("loadPlant: %v", err)
	}
	if got.Name != "Rosa" || got.Description != "primera" || got.Slug != "rosa" {
		t.Errorf("loadPlant mismatch: %+v", got)
	}
	if !got.CreatedAt.Equal(first.CreatedAt) {
		t.Errorf("CreatedAt round-trip mismatch: %v != %v", got.CreatedAt, first.CreatedAt)
	}

	// Newest first ordering.
	all, err := loadAllPlants()
	if err != nil {
		t.Fatalf("loadAllPlants: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("loadAllPlants returned %d plants, want 2", len(all))
	}
	if all[0].Slug != "tulipan" || all[1].Slug != "rosa" {
		t.Errorf("loadAllPlants ordering wrong: %v", []string{all[0].Slug, all[1].Slug})
	}

	// Missing plant.
	if _, err := loadPlant("no-existe"); !os.IsNotExist(err) {
		t.Errorf("loadPlant(missing) err = %v, want IsNotExist", err)
	}
}

func TestPlantImages(t *testing.T) {
	setupDataDir(t)

	plant := Plant{Name: "Aloe", Slug: "aloe"}
	mkdirPlantDirs(t, "aloe")
	if err := savePlantMeta("aloe", plant); err != nil {
		t.Fatalf("savePlantMeta: %v", err)
	}
	dir := filepath.Join(plantsDir, "aloe", "images")
	if err := os.MkdirAll(filepath.Join(dir, ".thumb"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".display"), 0755); err != nil {
		t.Fatal(err)
	}

	// Older file sorts first (growth timeline).
	older := filepath.Join(dir, "foto.png")
	newer := filepath.Join(dir, "foto2.png")
	if err := os.WriteFile(older, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(newer, []byte("y"), 0644); err != nil {
		t.Fatal(err)
	}
	// A derived folder and a non-image must be ignored.
	if err := os.WriteFile(filepath.Join(dir, ".thumb", "foto.png.jpg"), []byte("z"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("n"), 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := loadPlant("aloe")
	if err != nil {
		t.Fatalf("loadPlant: %v", err)
	}
	if len(loaded.Images) != 2 {
		t.Fatalf("loaded %d images, want 2", len(loaded.Images))
	}
	if loaded.Images[0].Name != "foto.png" || loaded.Images[1].Name != "foto2.png" {
		t.Errorf("image order wrong: %+v", loaded.Images)
	}
	if got := loaded.Images[0].URL; got != "/img/aloe/foto.png" {
		t.Errorf("image URL = %q", got)
	}
}

func TestTotalImageSize(t *testing.T) {
	setupDataDir(t)

	imgDir := filepath.Join(plantsDir, "rosa", "images")
	if err := os.MkdirAll(filepath.Join(imgDir, ".thumb"), 0755); err != nil {
		t.Fatal(err)
	}
	// 500 bytes inside images/...
	if err := os.WriteFile(filepath.Join(imgDir, "a.png"), bytes.Repeat([]byte("a"), 500), 0644); err != nil {
		t.Fatal(err)
	}
	// 200 bytes inside images/.thumb/...
	if err := os.WriteFile(filepath.Join(imgDir, ".thumb", "a.png.jpg"), bytes.Repeat([]byte("t"), 200), 0644); err != nil {
		t.Fatal(err)
	}
	// 300 bytes inside the plant folder but OUTSIDE images/... must be ignored.
	if err := os.WriteFile(filepath.Join(plantsDir, "rosa", "meta.json"), bytes.Repeat([]byte("m"), 300), 0644); err != nil {
		t.Fatal(err)
	}

	if got := totalImageSize(); got != 700 {
		t.Errorf("totalImageSize() = %d, want 700", got)
	}
}

func TestDecodeImageByContent(t *testing.T) {
	tests := []struct {
		format string
	}{
		{"png"},
		{"jpeg"},
	}
	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			raw := testImage(t, tt.format, 40, 30)
			img, err := decodeImage(bytes.NewReader(raw))
			if err != nil {
				t.Fatalf("decodeImage: %v", err)
			}
			if img.Bounds().Dx() != 40 || img.Bounds().Dy() != 30 {
				t.Errorf("decoded size %v, want 40x30", img.Bounds())
			}
		})
	}

	if _, err := decodeImage(bytes.NewReader([]byte("not an image at all"))); err == nil {
		t.Error("decodeImage accepted non-image content")
	}
}

func TestCreateDerived(t *testing.T) {
	setupDataDir(t)

	raw := testImage(t, "png", 1600, 800)
	src := filepath.Join(t.TempDir(), "original.png")
	if err := os.WriteFile(src, raw, 0644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(plantsDir, "derived.jpg")
	if err := createDerived(src, dst, thumbnailSize); err != nil {
		t.Fatalf("createDerived: %v", err)
	}

	f, err := os.Open(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer closeQuiet(f)

	// Derived files are always real JPEGs with the long side <= maxSize.
	img, err := jpeg.Decode(f)
	if err != nil {
		t.Fatalf("decoding derived file: %v", err)
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if w > thumbnailSize || h > thumbnailSize {
		t.Errorf("derived size %dx%d exceeds %d", w, h, thumbnailSize)
	}
	if w != 400 || h != 200 {
		t.Errorf("derived size = %dx%d, want 400x200", w, h)
	}

	// ensureDerived is idempotent and does not regenerate existing files.
	if err := ensureDerived(src, dst, thumbnailSize); err != nil {
		t.Fatalf("ensureDerived: %v", err)
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

func TestSaveImages(t *testing.T) {
	setupDataDir(t)

	plant := Plant{Name: "Hortensia", Slug: "hortensia"}
	mkdirPlantDirs(t, "hortensia")
	if err := savePlantMeta("hortensia", plant); err != nil {
		t.Fatalf("savePlantMeta: %v", err)
	}

	files := []*multipart.FileHeader{
		multipartHeader(t, "foto.png", testImage(t, "png", 640, 480)),
		multipartHeader(t, "no-imagen.txt", []byte("hola, esto no es una imagen")),
	}

	if err := saveImages("hortensia", files); err != nil {
		t.Fatalf("saveImages: %v", err)
	}

	imgDir := filepath.Join(plantsDir, "hortensia", "images")
	if _, err := os.Stat(filepath.Join(imgDir, "foto.png")); err != nil {
		t.Errorf("original image not saved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(imgDir, ".thumb", "foto.png.jpg")); err != nil {
		t.Errorf("thumbnail not generated: %v", err)
	}
	if _, err := os.Stat(filepath.Join(imgDir, ".display", "foto.png.jpg")); err != nil {
		t.Errorf("display image not generated: %v", err)
	}
	// Non-image uploads are silently skipped.
	if _, err := os.Stat(filepath.Join(imgDir, "no-imagen.txt")); !os.IsNotExist(err) {
		t.Errorf("non-image file should not be stored, got err=%v", err)
	}
}

func TestPopulateMosaics(t *testing.T) {
	rng := rand.New(rand.NewSource(42))

	// No images -> no layout.
	plants := []Plant{{Name: "A", Images: nil}}
	populateMosaics(plants, rng)
	if plants[0].MosaicLayout != nil {
		t.Error("empty plant should have nil mosaic")
	}

	// Single image -> one centered photo.
	img := []Image{{Name: "a.png", URL: "/img/x/a.png"}}
	plants = []Plant{{Name: "A", Images: img}}
	populateMosaics(plants, rng)
	if len(plants[0].MosaicLayout) != 1 {
		t.Fatalf("single image layout len = %d, want 1", len(plants[0].MosaicLayout))
	}
	if plants[0].MosaicLayout[0].RadiusPct != 0 || plants[0].MosaicLayout[0].Z != 1 {
		t.Errorf("single image should be centered on top: %+v", plants[0].MosaicLayout[0])
	}

	// More than 5 images -> capped at 5, all positions within the circle.
	imgs := make([]Image, 8)
	for i := range imgs {
		imgs[i] = Image{Name: "i" + strconv.Itoa(i) + ".png", URL: "/img/x/i" + strconv.Itoa(i) + ".png"}
	}
	plants = []Plant{{Name: "B", Images: imgs}}
	populateMosaics(plants, rng)
	if len(plants[0].MosaicLayout) != 5 {
		t.Fatalf("8-image layout len = %d, want 5", len(plants[0].MosaicLayout))
	}
	seen := map[string]bool{}
	for _, p := range plants[0].MosaicLayout {
		if seen[p.Name] {
			t.Error("mosaic reused an image")
		}
		seen[p.Name] = true
		if p.SizePct <= 0 || p.Z < 0 {
			t.Errorf("invalid layout entry: %+v", p)
		}
	}
}

func TestOobSizeSpan(t *testing.T) {
	setupDataDir(t)
	span := oobSizeSpan()
	if span == "" {
		t.Fatal("oobSizeSpan returned empty string")
	}
	if !bytes.Contains([]byte(span), []byte(`id="total-size"`)) {
		t.Errorf("oob span missing id: %s", span)
	}
	if !bytes.Contains([]byte(span), []byte(`hx-swap-oob="true"`)) {
		t.Errorf("oob span missing hx-swap-oob: %s", span)
	}
	if !bytes.Contains([]byte(span), []byte("0,00 MB")) {
		t.Errorf("oob span should reflect empty total: %s", span)
	}
}