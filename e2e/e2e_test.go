//go:build e2e

// Package e2e drives the real application in a headless Chromium via
// chromedp, against an instance started by the isolated compose stack
// (see docker-compose.e2e.yml and `make test-e2e`).
package e2e

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/cdproto/runtime"
)

func appURL() string {
	if u := os.Getenv("E2E_APP_URL"); u != "" {
		return u
	}
	return "http://localhost:8080"
}

// newBrowser creates a headless Chromium context for chromedp.
func newBrowser(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	execPath := os.Getenv("CHROME_BIN")
	if execPath == "" {
		execPath = "/usr/bin/chromium-browser"
	}
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(execPath),
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-gpu", true),
	)
	actx, cancelAllocator := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancelContext := chromedp.NewContext(actx)

	// Surface browser console errors so JS failures are visible in the test log.
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if ev, ok := ev.(*runtime.EventConsoleAPICalled); ok {
			t.Logf("console.%s: %v", ev.Type, ev.Args)
		}
		if ev, ok := ev.(*runtime.EventExceptionThrown); ok {
			t.Logf("js exception: %v", ev.ExceptionDetails)
		}
	})

	return ctx, func() {
		cancelContext()
		cancelAllocator()
	}
}

// waitFor polls a JS expression until it is truthy or the timeout elapses.
func waitFor(t *testing.T, ctx context.Context, timeout time.Duration, desc, expr string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		var ok bool
		lastErr = chromedp.Run(ctx, chromedp.Evaluate(expr, &ok))
		if lastErr == nil && ok {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s (last err: %v)", desc, lastErr)
}

func exists(sel string) string {
	return fmt.Sprintf(`!!document.querySelector(%s)`, strconv.Quote(sel))
}

func textIn(sel, text string) string {
	return fmt.Sprintf(
		`(() => { const el = document.querySelector(%s); return el ? el.textContent.includes(%s) : false; })()`,
		strconv.Quote(sel), strconv.Quote(text))
}

func visible(sel string) string {
	return fmt.Sprintf(
		`(() => { const el = document.querySelector(%s); return el ? getComputedStyle(el).display !== "none" : false; })()`,
		strconv.Quote(sel))
}

func imageLoaded(sel string) string {
	return fmt.Sprintf(
		`(() => { const el = document.querySelector(%s); return el ? (el.complete && el.naturalWidth > 0) : false; })()`,
		strconv.Quote(sel))
}

// clickButton clicks the first button whose trimmed text matches the given
// text (exact when exact=true, substring otherwise).
func clickButton(text string, exact bool) chromedp.ActionFunc {
	matcher := fmt.Sprintf(`b.textContent.trim().includes(%s)`, strconv.Quote(text))
	if exact {
		matcher = fmt.Sprintf(`b.textContent.trim() === %s`, strconv.Quote(text))
	}
	expr := fmt.Sprintf(
		`Array.from(document.querySelectorAll('button')).find(b => %s).click()`, matcher)
	return chromedp.ActionFunc(func(ctx context.Context) error {
		var res interface{}
		return chromedp.Run(ctx, chromedp.Evaluate(expr, &res))
	})
}

// writeSampleImage writes a real PNG to a temp file for the upload input.
func writeSampleImage(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 320, 240))
	for y := 0; y < 240; y++ {
		for x := 0; x < 320; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 96, A: 255})
		}
	}
	path := filepath.Join(t.TempDir(), "muestra.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEndToEndPlantLifecycle(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()
	base := appURL()

	const plantName = "Rosa E2E"
	const plantDesc = "Descripción E2E\nSegunda línea"

	// The app starts with a fresh volume -> empty state.
	if err := chromedp.Run(ctx, chromedp.Navigate(base+"/")); err != nil {
		t.Fatalf("navigate: %v", err)
	}
	waitFor(t, ctx, 30*time.Second, "app to load",
		`typeof htmx !== 'undefined' && typeof Alpine !== 'undefined'`)
	waitFor(t, ctx, 15*time.Second, "empty state", exists("#plant-grid .empty-state"))
	if err := chromedp.Run(ctx, chromedp.Evaluate(textIn("#plant-grid", "Empieza tu bitácora"), nil)); err != nil {
		t.Fatalf("empty state text: %v", err)
	}

	// Open the "Nueva Planta" modal and fill it in with a real image.
	sample := writeSampleImage(t)
	if err := chromedp.Run(ctx,
		clickButton("+ Nueva Planta", true),
		chromedp.SetValue(`#name`, plantName, chromedp.ByID),
		chromedp.SetValue(`#description`, plantDesc, chromedp.ByID),
		chromedp.SetUploadFiles(`#images`, []string{sample}, chromedp.ByID),
	); err != nil {
		t.Fatalf("filling create form: %v", err)
	}
	waitFor(t, ctx, 5*time.Second, "create modal visible", visible(".modal-overlay"))
	if err := chromedp.Run(ctx, clickButton("Crear Planta", true)); err != nil {
		t.Fatalf("submit create: %v", err)
	}

	// HTMX swaps the grid: the new card (with its collage thumb) appears.
	waitFor(t, ctx, 15*time.Second, "plant card in grid", textIn("#plant-grid", plantName))
	waitFor(t, ctx, 15*time.Second, "mosaic thumbnail rendered", imageLoaded(".plant-card .mosaic-photo"))

	// Open the detail view from the card.
	if err := chromedp.Run(ctx, chromedp.Evaluate(`document.querySelector('.plant-card').click()`, &[]bool{})); err != nil {
		t.Fatalf("open detail: %v", err)
	}
	waitFor(t, ctx, 15*time.Second, "detail view", exists("#detail-container .plant-detail"))
	waitFor(t, ctx, 15*time.Second, "description in detail", textIn("#detail-container", "Descripción E2E"))
	waitFor(t, ctx, 15*time.Second, "gallery image loaded", imageLoaded("#detail-container .gallery-image"))

	// Delete the plant through the confirm modal.
	if err := chromedp.Run(ctx, clickButton("Eliminar Planta", false)); err != nil {
		t.Fatalf("open delete modal: %v", err)
	}
	waitFor(t, ctx, 5*time.Second, "delete modal visible", visible("#detail-container .modal-overlay"))
	if err := chromedp.Run(ctx, clickButton("Eliminar", true)); err != nil {
		t.Fatalf("confirm delete: %v", err)
	}

	// Back to the empty state.
	waitFor(t, ctx, 15*time.Second, "empty state after delete", exists("#plant-grid .empty-state"))
	var grid string
	if err := chromedp.Run(ctx, chromedp.OuterHTML("#plant-grid", &grid, chromedp.ByID)); err != nil {
		t.Fatalf("reading final grid: %v", err)
	}
	if strings.Contains(grid, plantName) {
		t.Fatalf("plant should be gone after delete: %s", grid)
	}
}