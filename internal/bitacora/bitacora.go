// Package bitacora implements the Bitácora de Plantas application: storage,
// image handling and the HTTP handler. main.go is a thin entrypoint that
// delegates here; the package exposes a small public API so tests under test/
// can exercise the app from an external package.
package bitacora

import (
	"encoding/json"
	"fmt"
	"html/template"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"io/fs"
	"log"
	"math"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

const (
	maxUploadSize = 50 << 20 // 50MB
	// ThumbnailSize is the max long side for mosaic thumbnails.
	ThumbnailSize = 400
	displaySize   = 1200 // max long side for display/detail images
	jpegQuality   = 82   // quality for derived JPEG versions
)

// DataDir and PlantsDir hold the on-disk storage locations. They are
// resolved from the environment once at startup so the app can point them at
// a Docker volume and tests at an ephemeral directory. BITACORA_DATA_DIR
// wins, then DATA_DIR (kept for backwards compatibility with the existing
// compose setup), then the historical "data" default.
var (
	DataDir   string
	PlantsDir string
)

// TemplatesDir holds the directory containing the HTML templates. It defaults
// to "templates" relative to the process working directory; tests point it at
// an absolute path because they run from their own package directory.
var TemplatesDir = "templates"

// InitDataDirs resolves DataDir/PlantsDir from the environment.
func InitDataDirs() {
	root := os.Getenv("BITACORA_DATA_DIR")
	if root == "" {
		root = os.Getenv("DATA_DIR")
	}
	if root == "" {
		root = "data"
	}
	DataDir = root
	PlantsDir = filepath.Join(root, "plants")
}

// imgMu serializes lazy generation of derived images so concurrent requests
// for the same version can't write the same file at the same time.
var imgMu sync.Mutex

// Image represents a single image of a plant with its metadata
type Image struct {
	Name    string    `json:"-"`
	URL     string    `json:"-"`
	ModTime time.Time `json:"-"`
}

// Plant represents a plant with its metadata
type Plant struct {
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	CreatedAt    time.Time     `json:"created_at"`
	Slug         string        `json:"-"`
	Images       []Image       `json:"-"`
	MosaicLayout []MosaicPhoto `json:"-"` // 0-5 picked images + circular collage layout data
}

// MosaicPhoto carries everything needed to place one photo inside the
// circular overlapping collage: its position on an imaginary circle
// (Angle in degrees + RadiusPct as % of the container), its size (% of the
// container), a random scrapbook tilt and its stacking order.
type MosaicPhoto struct {
	Image
	Angle     float64 // degrees around the circle (0 = right, CCW in CSS coords)
	RadiusPct float64 // distance of the photo center from the container center (%)
	SizePct   float64 // photo diameter as % of the container
	Rotate    float64 // scrapbook tilt in degrees (-14..14)
	Z         int
}

// PopulateMosaics fills each plant's MosaicLayout: a random subset of up to
// 5 images placed in a circular overlapping collage. Every render produces a
// different arrangement (new subset, new start angle, new dominant photo,
// new rotations and sizes).
func PopulateMosaics(plants []Plant, rng *rand.Rand) {
	for i := range plants {
		p := &plants[i]
		n := len(p.Images)
		if n == 0 {
			p.MosaicLayout = nil
			continue
		}
		count := n
		if count > 5 {
			count = 5
		}
		imgs := make([]Image, len(p.Images))
		copy(imgs, p.Images)
		rng.Shuffle(len(imgs), func(i, j int) { imgs[i], imgs[j] = imgs[j], imgs[i] })
		p.MosaicLayout = circularLayout(imgs[:count], count, rng)
	}
}

// circularLayout distributes photos around an imaginary circle inside the
// square .plant-mosaic container:
//
//	left = 50% + R*cos(angle)   top = 50% + R*sin(angle)
//
// with angle_i = startAngle + i*(360/n). One random photo is the dominant:
// bigger, higher z-index and pulled toward the center (R*0.6) so the
// composition stays balanced; the rest form the outer ring. Sizes vary per
// photo and every photo gets a slight random rotation (scrapbook feel).
func circularLayout(imgs []Image, count int, rng *rand.Rand) []MosaicPhoto {
	layout := make([]MosaicPhoto, count)

	if count == 1 {
		// Single photo: one centered circle.
		layout[0] = MosaicPhoto{
			Image:     imgs[0],
			Angle:     0,
			RadiusPct: 0,
			SizePct:   68,
			Rotate:    float64(rng.Intn(9) - 4), // subtle tilt
			Z:         1,
		}
		return layout
	}

	// Ring radius (30-33% of the container). startAngle random per render.
	R := float64(30 + rng.Intn(4))
	start := float64(rng.Intn(360))

	dominant := rng.Intn(count)
	dominantSize := float64(55 + rng.Intn(6)) // 55..60 % of the container

	others := make([]int, 0, count-1)
	sizes := make([]float64, count)

	for j := 0; j < count; j++ {
		angle := start + float64(j)*(360.0/float64(count))
		var size, radius float64
		switch {
		case count == 2:
			// Two overlapping circles 180° apart: sizes 55% and 48%. A smaller
			// ring radius (22-24%) guarantees the two circles always overlap
			// (2R < r1+r2) while staying inside the square container.
			size = 55
			if j != dominant {
				size = 48
			}
			radius = float64(22 + rng.Intn(3))
		case j == dominant:
			size = dominantSize
			radius = R * 0.6 // dominant sits near the center
		default:
			size = float64(40 + rng.Intn(7)) // 40..46 % of the container
			radius = R
		}
		sizes[j] = size
		if j != dominant {
			others = append(others, j)
		}
		layout[j] = MosaicPhoto{
			Image:     imgs[j],
			Angle:     angle,
			RadiusPct: radius,
			SizePct:   size,
			Rotate:    float64(rng.Intn(29) - 14), // -14..14 degrees
			Z:         0,
		}
	}

	// z-index: bigger photos on top (natural overlap), dominant above all.
	sort.Slice(others, func(a, b int) bool { return sizes[others[a]] > sizes[others[b]] })
	for rank, idx := range others {
		layout[idx].Z = rank + 1
	}
	layout[dominant].Z = count + 1

	return layout
}

// loadGridPlants loads all plants and computes a fresh random circular
// collage for each one (new subset + new arrangement on every grid render).
func loadGridPlants() ([]Plant, error) {
	plants, err := LoadAllPlants()
	if err != nil {
		return nil, err
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	PopulateMosaics(plants, rng)
	return plants, nil
}

var templates *template.Template

// Run starts the HTTP server. Extracted so main.go stays a thin entrypoint and
// the whole app remains exercisable from test/ via the exported API.
func Run() {
	InitDataDirs()

	// Ensure data directories exist
	if err := os.MkdirAll(DataDir, 0755); err != nil {
		log.Fatal("Error creating data directory:", err)
	}
	if err := os.MkdirAll(PlantsDir, 0755); err != nil {
		log.Fatal("Error creating data directory:", err)
	}

	// Load templates
	if _, err := LoadTemplates(); err != nil {
		log.Fatal("Error loading templates:", err)
	}

	log.Println("Servidor iniciando en puerto 8080...")
	log.Fatal(http.ListenAndServe(":8080", NewMux()))
}

// LoadTemplates parses every template in TemplatesDir with the custom function
// map. Extracted from main so tests can load the exact same set.
func LoadTemplates() (*template.Template, error) {
	funcMap := template.FuncMap{
		"split": strings.Split,
		// plantJSON renders the plant data as a JSON-safe object usable inside
		// a <script type="application/json"> block (json.Marshal escapes <, >, &).
		"plantJSON": func(p Plant) template.JS {
			data := struct {
				Slug        string `json:"slug"`
				Name        string `json:"name"`
				Description string `json:"description"`
			}{p.Slug, p.Name, p.Description}
			b, _ := json.Marshal(data)
			return template.JS(b)
		},
		// math helpers used by plant-grid.html to position each circular
		// collage photo: left = 50 + R*cos(angle), top = 50 + R*sin(angle).
		"add":             func(a, b float64) float64 { return a + b },
		"mul":             func(a, b float64) float64 { return a * b },
		"sinDeg":          func(d float64) float64 { return math.Sin(d * math.Pi / 180.0) },
		"cosDeg":          func(d float64) float64 { return math.Cos(d * math.Pi / 180.0) },
		"formatDate":      FormatDate,
		"formatDateShort": FormatDateShort,
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseGlob(filepath.Join(TemplatesDir, "*.html"))
	if err != nil {
		return nil, err
	}
	templates = tmpl
	return tmpl, nil
}

// FormatDate renders a timestamp in the Spanish journal style used by the
// plant detail header, e.g. "19 de August de 2026".
func FormatDate(t time.Time) string {
	return t.Format("2 de January de 2006")
}

// FormatDateShort renders a timestamp in the compact DD/MM/YYYY style used by
// the gallery captions.
func FormatDateShort(t time.Time) string {
	return t.Format("02/01/2006")
}

// NewMux returns the fully wired HTTP handler for the application. Extracted
// from main so tests can exercise it with httptest.
func NewMux() http.Handler {
	mux := http.NewServeMux()

	// Static files first
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// API routes
	mux.HandleFunc("GET /{$}", handleHome) // {$} ensures exact match
	mux.HandleFunc("GET /planta/{slug}", handlePlantDetail)
	mux.HandleFunc("POST /plantas", handleCreatePlant)
	mux.HandleFunc("PUT /planta/{slug}", handleUpdatePlant)
	mux.HandleFunc("POST /planta/{slug}/imagenes", handleAddImages)
	mux.HandleFunc("DELETE /planta/{slug}/imagenes/{filename}", handleDeleteImage)
	mux.HandleFunc("DELETE /planta/{slug}", handleDeletePlant)
	mux.HandleFunc("GET /img/{slug}/{filename}", handleServeImage)

	return mux
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	plants, err := loadGridPlants()
	if err != nil {
		http.Error(w, "Error cargando plantas", http.StatusInternalServerError)
		return
	}

	data := struct {
		Plants    []Plant
		TotalSize string
	}{
		Plants:    plants,
		TotalSize: SizeLabel(),
	}

	if err := templates.ExecuteTemplate(w, "index.html", data); err != nil {
		http.Error(w, "Error renderizando template", http.StatusInternalServerError)
	}
}

func handlePlantDetail(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	plant, err := LoadPlant(slug)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Planta no encontrada", http.StatusNotFound)
		} else {
			http.Error(w, "Error cargando planta", http.StatusInternalServerError)
		}
		return
	}

	renderPlantDetail(w, plant)
}

func handleCreatePlant(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		http.Error(w, "Archivo demasiado grande", http.StatusRequestEntityTooLarge)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))

	if name == "" {
		http.Error(w, "El nombre es requerido", http.StatusBadRequest)
		return
	}

	// Create slug
	slug := CreateSlug(name)
	slug = EnsureUniqueSlug(slug)

	// Create plant directory
	plantDir := filepath.Join(PlantsDir, slug)
	if err := os.MkdirAll(filepath.Join(plantDir, "images", ".thumb"), 0755); err != nil {
		http.Error(w, "Error creando directorio", http.StatusInternalServerError)
		return
	}
	if err := os.MkdirAll(filepath.Join(plantDir, "images", ".display"), 0755); err != nil {
		http.Error(w, "Error creando directorio", http.StatusInternalServerError)
		return
	}

	// Save metadata
	plant := Plant{
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
		Slug:        slug,
	}

	if err := SavePlantMeta(slug, plant); err != nil {
		http.Error(w, "Error guardando metadatos", http.StatusInternalServerError)
		return
	}

	// Handle images if provided
	if files := r.MultipartForm.File["images"]; len(files) > 0 {
		if err := SaveImages(slug, files); err != nil {
			log.Printf("Error saving images: %v", err)
		}
	}

	// Return updated plant list
	renderPlantGrid(w)
}

func handleDeletePlant(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	slug = filepath.Base(slug)
	if slug == "." || slug == ".." {
		http.Error(w, "Planta no encontrada", http.StatusNotFound)
		return
	}

	plantDir := filepath.Join(PlantsDir, slug)
	if _, err := os.Stat(filepath.Join(plantDir, "meta.json")); os.IsNotExist(err) {
		http.Error(w, "Planta no encontrada", http.StatusNotFound)
		return
	}

	if err := os.RemoveAll(plantDir); err != nil {
		http.Error(w, "Error eliminando la planta", http.StatusInternalServerError)
		return
	}

	// Return updated plant list
	renderPlantGrid(w)
}

func renderPlantGrid(w http.ResponseWriter) {
	plants, err := loadGridPlants()
	if err != nil {
		http.Error(w, "Error cargando plantas", http.StatusInternalServerError)
		return
	}

	data := struct {
		Plants []Plant
	}{
		Plants: plants,
	}

	if err := templates.ExecuteTemplate(w, "plant-grid.html", data); err != nil {
		http.Error(w, "Error renderizando template", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write([]byte(OobSizeSpan()))
}

func renderPlantDetail(w http.ResponseWriter, plant Plant) {
	if err := templates.ExecuteTemplate(w, "plant-detail.html", plant); err != nil {
		http.Error(w, "Error renderizando template", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write([]byte(OobSizeSpan()))
}

func handleUpdatePlant(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Error procesando formulario", http.StatusBadRequest)
		return
	}

	plant, err := LoadPlant(slug)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Planta no encontrada", http.StatusNotFound)
		} else {
			http.Error(w, "Error cargando planta", http.StatusInternalServerError)
		}
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))

	if name == "" {
		http.Error(w, "El nombre es requerido", http.StatusBadRequest)
		return
	}

	plant.Name = name
	plant.Description = description

	if err := SavePlantMeta(slug, plant); err != nil {
		http.Error(w, "Error actualizando planta", http.StatusInternalServerError)
		return
	}

	// Return updated detail view
	updatedPlant, err := LoadPlant(slug)
	if err != nil {
		http.Error(w, "Error cargando planta actualizada", http.StatusInternalServerError)
		return
	}

	renderPlantDetail(w, updatedPlant)
}

func handleAddImages(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		http.Error(w, "Archivo demasiado grande", http.StatusRequestEntityTooLarge)
		return
	}

	files := r.MultipartForm.File["images"]
	if len(files) == 0 {
		http.Error(w, "No se seleccionaron imágenes", http.StatusBadRequest)
		return
	}

	if err := SaveImages(slug, files); err != nil {
		http.Error(w, "Error guardando imágenes", http.StatusInternalServerError)
		return
	}

	// Return updated detail view
	plant, err := LoadPlant(slug)
	if err != nil {
		http.Error(w, "Error cargando planta", http.StatusInternalServerError)
		return
	}

	renderPlantDetail(w, plant)
}

func handleDeleteImage(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	filename := r.PathValue("filename")

	// Sanitize filename
	filename = filepath.Base(filename)
	if filename == "." || filename == ".." {
		http.Error(w, "Nombre de archivo inválido", http.StatusBadRequest)
		return
	}

	plantDir := filepath.Join(PlantsDir, slug)
	imagesDir := filepath.Join(plantDir, "images")

	imagePath := filepath.Join(imagesDir, filename)

	// Check if plant exists
	if _, err := os.Stat(filepath.Join(plantDir, "meta.json")); os.IsNotExist(err) {
		http.Error(w, "Planta no encontrada", http.StatusNotFound)
		return
	}

	// Delete image and derived versions
	_ = os.Remove(imagePath)
	_ = os.Remove(filepath.Join(imagesDir, ".thumb", DerivedName(filename)))
	_ = os.Remove(filepath.Join(imagesDir, ".display", DerivedName(filename)))

	// Return updated detail view
	plant, err := LoadPlant(slug)
	if err != nil {
		http.Error(w, "Error cargando planta", http.StatusInternalServerError)
		return
	}

	renderPlantDetail(w, plant)
}

func handleServeImage(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	filename := r.PathValue("filename")

	// Sanitize paths
	slug = filepath.Base(slug)
	filename = filepath.Base(filename)
	if slug == "." || slug == ".." || filename == "." || filename == ".." {
		http.Error(w, "Ruta inválida", http.StatusBadRequest)
		return
	}

	originalPath := filepath.Join(PlantsDir, slug, "images", filename)

	// Security check - ensure path is within plants directory
	absPath, err := filepath.Abs(originalPath)
	if err != nil {
		http.Error(w, "Error de ruta", http.StatusInternalServerError)
		return
	}

	absDataDir, err := filepath.Abs(PlantsDir)
	if err != nil {
		http.Error(w, "Error de ruta", http.StatusInternalServerError)
		return
	}

	if !strings.HasPrefix(absPath, absDataDir) {
		http.Error(w, "Acceso denegado", http.StatusForbidden)
		return
	}

	// Pick which version to serve. Derived versions are always real JPEGs
	// named <originalname>.jpg; if they don't exist yet (legacy uploads) they
	// are generated lazily from the original.
	var servePath string
	isDerived := false
	maxSize := displaySize
	switch {
	case r.URL.Query().Get("thumb") == "1":
		servePath = filepath.Join(PlantsDir, slug, "images", ".thumb", DerivedName(filename))
		isDerived = true
		maxSize = ThumbnailSize
	case r.URL.Query().Get("original") == "1":
		servePath = originalPath
	default:
		servePath = filepath.Join(PlantsDir, slug, "images", ".display", DerivedName(filename))
		isDerived = true
	}

	if isDerived {
		if _, err := os.Stat(servePath); os.IsNotExist(err) {
			if _, oerr := os.Stat(originalPath); os.IsNotExist(oerr) {
				http.Error(w, "Imagen no encontrada", http.StatusNotFound)
				return
			}
			if err := EnsureDerived(originalPath, servePath, maxSize); err != nil {
				log.Printf("Error generando imagen derivada %s: %v", servePath, err)
				http.Error(w, "No se pudo generar la imagen", http.StatusInternalServerError)
				return
			}
		}
	}

	f, err := os.Open(servePath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Imagen no encontrada", http.StatusNotFound)
			return
		}
		http.Error(w, "Error abriendo imagen", http.StatusInternalServerError)
		return
	}
	defer closeQuiet(f)

	// Content-Type must reflect the actual bytes, not the file extension:
	// legacy files can be JPEG data stored with a .png name.
	buf := make([]byte, 512)
	n, _ := io.ReadFull(f, buf)
	ct := http.DetectContentType(buf[:n])
	if !strings.HasPrefix(ct, "image/") {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)

	info, err := f.Stat()
	if err != nil {
		http.Error(w, "Error leyendo imagen", http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, r, filename, info.ModTime(), f)
}

// Helper functions

// closeQuiet closes c, deliberately discarding the error. Used for deferred
// closes where the read side already dictates the outcome.
func closeQuiet(c io.Closer) {
	_ = c.Close()
}

// CreateSlug converts a plant name into its URL-safe slug.
func CreateSlug(name string) string {
	// Convert to lowercase and normalize
	slug := strings.ToLower(name)

	// Remove accents (basic implementation)
	slug = RemoveAccents(slug)

	// Replace spaces and invalid characters with hyphens
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	slug = reg.ReplaceAllString(slug, "-")

	// Remove leading/trailing hyphens
	slug = strings.Trim(slug, "-")

	return slug
}

// RemoveAccents strips common diacritics from a string.
func RemoveAccents(s string) string {
	accents := map[rune]rune{
		'á': 'a', 'à': 'a', 'ä': 'a', 'â': 'a', 'ã': 'a',
		'é': 'e', 'è': 'e', 'ë': 'e', 'ê': 'e',
		'í': 'i', 'ì': 'i', 'ï': 'i', 'î': 'i',
		'ó': 'o', 'ò': 'o', 'ö': 'o', 'ô': 'o', 'õ': 'o',
		'ú': 'u', 'ù': 'u', 'ü': 'u', 'û': 'u',
		'ñ': 'n', 'ç': 'c',
	}

	var result []rune
	for _, r := range s {
		if replacement, exists := accents[r]; exists {
			result = append(result, replacement)
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}

// EnsureUniqueSlug returns slug if it is not taken yet, otherwise appends a
// numeric suffix (-1, -2, ...) until a free directory is found.
func EnsureUniqueSlug(slug string) string {
	originalSlug := slug
	counter := 1

	for {
		if _, err := os.Stat(filepath.Join(PlantsDir, slug)); os.IsNotExist(err) {
			return slug
		}
		slug = originalSlug + "-" + strconv.Itoa(counter)
		counter++
	}
}

// LoadAllPlants returns every plant sorted by creation date (newest first).
func LoadAllPlants() ([]Plant, error) {
	entries, err := os.ReadDir(PlantsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Plant{}, nil
		}
		return nil, err
	}

	var plants []Plant
	for _, entry := range entries {
		if entry.IsDir() {
			if plant, err := LoadPlant(entry.Name()); err == nil {
				plants = append(plants, plant)
			}
		}
	}

	// Sort by creation date (newest first)
	sort.Slice(plants, func(i, j int) bool {
		return plants[i].CreatedAt.After(plants[j].CreatedAt)
	})

	return plants, nil
}

// LoadPlant reads a plant's metadata and images from disk.
func LoadPlant(slug string) (Plant, error) {
	var plant Plant

	// Sanitize slug
	slug = filepath.Base(slug)
	if slug == "." || slug == ".." {
		return plant, os.ErrNotExist
	}

	metaPath := filepath.Join(PlantsDir, slug, "meta.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return plant, err
	}

	if err := json.Unmarshal(data, &plant); err != nil {
		return plant, err
	}

	plant.Slug = slug

	// Load images (excluding the .thumb subdirectory)
	imagesDir := filepath.Join(PlantsDir, slug, "images")
	if entries, err := os.ReadDir(imagesDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			if !IsImageFile(entry.Name()) {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			plant.Images = append(plant.Images, Image{
				Name:    entry.Name(),
				URL:     "/img/" + slug + "/" + entry.Name(),
				ModTime: info.ModTime(),
			})
		}
		// Sort oldest first (chronological = growth timeline)
		sort.Slice(plant.Images, func(i, j int) bool {
			return plant.Images[i].ModTime.Before(plant.Images[j].ModTime)
		})
	}

	return plant, nil
}

// SavePlantMeta writes a plant's metadata JSON to disk.
func SavePlantMeta(slug string, plant Plant) error {
	metaPath := filepath.Join(PlantsDir, slug, "meta.json")
	data, err := json.MarshalIndent(plant, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(metaPath, data, 0644)
}

// SaveImages validates and stores uploaded image files for a plant, then
// generates their derived thumbnails and display versions.
func SaveImages(slug string, files []*multipart.FileHeader) error {
	plantDir := filepath.Join(PlantsDir, slug)
	imagesDir := filepath.Join(plantDir, "images")
	thumbsDir := filepath.Join(imagesDir, ".thumb")
	displaysDir := filepath.Join(imagesDir, ".display")

	for _, fileHeader := range files {
		// Validate file type
		file, err := fileHeader.Open()
		if err != nil {
			continue
		}

		// Read first 512 bytes for content type detection
		buffer := make([]byte, 512)
		_, err = file.Read(buffer)
		if err != nil {
			closeQuiet(file)
			continue
		}
		_, _ = file.Seek(0, 0)

		contentType := http.DetectContentType(buffer)
		if !IsValidImageType(contentType) {
			closeQuiet(file)
			continue
		}

		// Sanitize filename
		filename := SanitizeFilename(fileHeader.Filename)
		if filename == "" {
			closeQuiet(file)
			continue
		}

		// Ensure unique filename
		filename = EnsureUniqueFilename(imagesDir, filename)

		// Save original image
		originalPath := filepath.Join(imagesDir, filename)
		dst, err := os.Create(originalPath)
		if err != nil {
			closeQuiet(file)
			continue
		}

		_, err = io.Copy(dst, file)
		closeQuiet(dst)
		closeQuiet(file)

		if err != nil {
			_ = os.Remove(originalPath)
			continue
		}

		// Generate derived versions (always real JPEGs with .jpg extension).
		// The original file stays untouched in images/.
		if err := CreateDerived(originalPath, filepath.Join(displaysDir, DerivedName(filename)), displaySize); err != nil {
			log.Printf("Error generando versión display para %s: %v", filename, err)
		}
		if err := CreateDerived(originalPath, filepath.Join(thumbsDir, DerivedName(filename)), ThumbnailSize); err != nil {
			log.Printf("Error generando miniatura para %s: %v", filename, err)
		}
	}

	return nil
}

// IsValidImageType reports whether a MIME content type is an accepted upload
// format.
func IsValidImageType(contentType string) bool {
	validTypes := []string{
		"image/jpeg",
		"image/png",
		"image/webp",
	}

	for _, validType := range validTypes {
		if contentType == validType {
			return true
		}
	}
	return false
}

// IsImageFile reports whether a filename has an accepted image extension.
func IsImageFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	validExts := []string{".jpg", ".jpeg", ".png", ".webp"}

	for _, validExt := range validExts {
		if ext == validExt {
			return true
		}
	}
	return false
}

// SanitizeFilename reduces a filename to a safe, flat basename.
func SanitizeFilename(filename string) string {
	// Keep only safe characters
	filename = filepath.Base(filename)

	// Replace unsafe characters
	reg := regexp.MustCompile(`[^a-zA-Z0-9._-]`)
	filename = reg.ReplaceAllString(filename, "_")

	// Ensure it's not empty and has an extension
	if filename == "" || filename == "." || filename == ".." {
		return ""
	}

	return filename
}

// EnsureUniqueFilename returns filename if it is not taken in dir, otherwise
// appends a numeric suffix (-1, -2, ...) before the extension until a free
// name is found.
func EnsureUniqueFilename(dir, filename string) string {
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)
	counter := 1

	for {
		if _, err := os.Stat(filepath.Join(dir, filename)); os.IsNotExist(err) {
			return filename
		}
		filename = name + "_" + strconv.Itoa(counter) + ext
		counter++
	}
}

// DerivedName returns the JPEG derived filename for an original: the original
// name with .jpg appended. Keeping the original extension in the name makes
// derived files unique even when two originals share the same stem but differ
// in extension (e.g. foto.png and foto.jpg).
func DerivedName(filename string) string {
	return filename + ".jpg"
}

// DecodeImage decodes an image based on its CONTENT, never its extension.
// It reads the first 512 bytes, detects the real type, and uses the matching
// decoder (JPEG data with a .png name is decoded as JPEG).
func DecodeImage(r io.Reader) (image.Image, error) {
	buf := make([]byte, 512)
	n, err := io.ReadFull(r, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	buf = buf[:n]

	ct := http.DetectContentType(buf)

	seeker, ok := r.(io.Seeker)
	rewind := func() bool {
		if !ok {
			return false
		}
		_, err := seeker.Seek(0, io.SeekStart)
		return err == nil
	}

	switch {
	case strings.HasPrefix(ct, "image/jpeg"):
		if !rewind() {
			return nil, fmt.Errorf("cannot rewind source")
		}
		return jpeg.Decode(r)
	case strings.HasPrefix(ct, "image/png"):
		if !rewind() {
			return nil, fmt.Errorf("cannot rewind source")
		}
		return png.Decode(r)
	case strings.HasPrefix(ct, "image/webp"):
		if !rewind() {
			return nil, fmt.Errorf("cannot rewind source")
		}
		return webp.Decode(r)
	}

	// Fallback: sniff magic bytes directly.
	if len(buf) >= 3 && buf[0] == 0xff && buf[1] == 0xd8 && buf[2] == 0xff {
		if !rewind() {
			return nil, fmt.Errorf("cannot rewind source")
		}
		return jpeg.Decode(r)
	}
	if len(buf) >= 8 && buf[1] == 'P' && buf[2] == 'N' && buf[3] == 'G' {
		if !rewind() {
			return nil, fmt.Errorf("cannot rewind source")
		}
		return png.Decode(r)
	}
	if len(buf) >= 12 && string(buf[0:4]) == "RIFF" && string(buf[8:12]) == "WEBP" {
		if !rewind() {
			return nil, fmt.Errorf("cannot rewind source")
		}
		return webp.Decode(r)
	}

	return nil, fmt.Errorf("unsupported image content type: %s", ct)
}

// CreateDerived decodes srcPath by content and writes a JPEG (quality
// jpegQuality) scaled so its long side is at most maxSize to dstPath.
func CreateDerived(srcPath, dstPath string, maxSize int) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer closeQuiet(srcFile)

	img, err := DecodeImage(srcFile)
	if err != nil {
		return err
	}

	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	newW, newH := width, height
	if width > maxSize || height > maxSize {
		if width >= height {
			newW = maxSize
			newH = int(float64(height) * float64(maxSize) / float64(width))
		} else {
			newH = maxSize
			newW = int(float64(width) * float64(maxSize) / float64(height))
		}
		if newH < 1 {
			newH = 1
		}
		if newW < 1 {
			newW = 1
		}
	}

	var out image.Image
	if newW != width || newH != height {
		dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
		draw.CatmullRom.Scale(dst, dst.Rect, img, bounds, draw.Over, nil)
		out = dst
	} else {
		out = img
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return err
	}

	// Write to a temp file then rename so readers never see a partial file.
	tmpPath := dstPath + ".tmp"
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	if err := jpeg.Encode(tmpFile, out, &jpeg.Options{Quality: jpegQuality}); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, dstPath)
}

// EnsureDerived generates derivedPath from originalPath if it doesn't exist.
// A package-level mutex serializes generation to avoid races.
func EnsureDerived(originalPath, derivedPath string, maxSize int) error {
	imgMu.Lock()
	defer imgMu.Unlock()

	if _, err := os.Stat(derivedPath); err == nil {
		return nil
	}
	return CreateDerived(originalPath, derivedPath, maxSize)
}

// TotalImageSize returns the sum of the sizes of every file inside any
// images/ directory under data/plants (originals + derived versions).
func TotalImageSize() int64 {
	var total int64
	_ = filepath.WalkDir(PlantsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(PlantsDir, path)
		if err != nil {
			return nil
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) < 2 || parts[1] != "images" {
			return nil
		}
		if info, err := d.Info(); err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

// FormatMB renders a byte count as a MB value with a comma decimal separator
// (Spanish locale), e.g. "1,50".
func FormatMB(bytes int64) string {
	mb := float64(bytes) / (1024 * 1024)
	return strings.Replace(fmt.Sprintf("%.2f", mb), ".", ",", 1)
}

// SizeLabel returns the full footer text for the current total.
func SizeLabel() string {
	return "📦 Peso total de imágenes: " + FormatMB(TotalImageSize()) + " MB"
}

// OobSizeSpan renders the footer span for an HTMX out-of-band swap so
// mutating responses can update the footer total without a page reload.
func OobSizeSpan() string {
	return `<span id="total-size" hx-swap-oob="true">` + SizeLabel() + `</span>`
}
