package webapp

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gofiber/fiber/v3"
)

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":         {Data: []byte("<!doctype html><html><head><title>t</title></head><body>APP</body></html>")},
		"assets/index-a1.js": {Data: []byte("console.log(1)")},
	}
}

func get(t *testing.T, app *fiber.App, target string) (int, string, map[string]string) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest("GET", target, nil))
	if err != nil {
		t.Fatalf("GET %s: %v", target, err)
	}
	body, _ := io.ReadAll(resp.Body)
	h := map[string]string{}
	for k, v := range resp.Header {
		h[k] = strings.Join(v, ",")
	}
	return resp.StatusCode, string(body), h
}

func TestWebAppServesIndexForDeepLink(t *testing.T) {
	app := fiber.New()
	register(app, testFS(), "")
	for _, target := range []string{"/app", "/app/", "/app/analytics", "/app/analytics/x"} {
		status, body, h := get(t, app, target)
		if status != 200 || !strings.Contains(body, "APP") {
			t.Fatalf("%s: status %d body %q", target, status, body)
		}
		if !strings.HasPrefix(h["Content-Type"], "text/html") {
			t.Fatalf("%s: content-type %q", target, h["Content-Type"])
		}
		if h["Cache-Control"] != "no-cache" {
			t.Fatalf("%s: cache-control %q", target, h["Cache-Control"])
		}
		if !strings.Contains(body, `<base href="/app/">`) {
			t.Fatalf("%s: missing injected <base>: %q", target, body)
		}
		if h["X-Frame-Options"] != "DENY" || h["X-Content-Type-Options"] != "nosniff" {
			t.Fatalf("%s: missing security headers %v", target, h)
		}
	}
}

func TestWebAppInjectsBasePath(t *testing.T) {
	app := fiber.New()
	register(app.Group("/wa"), testFS(), "/wa")
	_, body, _ := get(t, app, "/wa/app/analytics")
	if !strings.Contains(body, `<base href="/wa/app/">`) {
		t.Fatalf("missing base path in <base>: %q", body)
	}
}

func TestWebAppServesAssetWithLongCache(t *testing.T) {
	app := fiber.New()
	register(app, testFS(), "")
	status, body, h := get(t, app, "/app/assets/index-a1.js")
	if status != 200 || body != "console.log(1)" {
		t.Fatalf("status %d body %q", status, body)
	}
	if h["Cache-Control"] != "public, max-age=31536000, immutable" {
		t.Fatalf("cache-control %q", h["Cache-Control"])
	}
	if !strings.Contains(h["Content-Type"], "javascript") {
		t.Fatalf("content-type %q", h["Content-Type"])
	}
}

func TestWebAppMissingAssetIs404(t *testing.T) {
	app := fiber.New()
	register(app, testFS(), "")
	if status, _, _ := get(t, app, "/app/assets/missing.js"); status != 404 {
		t.Fatalf("missing asset: status %d, want 404", status)
	}
}

func TestWebAppRejectsPathTraversal(t *testing.T) {
	app := fiber.New()
	fsys := testFS()
	fsys["../secret"] = &fstest.MapFile{Data: []byte("SECRET")}
	register(app, fsys, "")
	for _, target := range []string{"/app/../go.mod", "/app/%2e%2e/secret", "/app/..%2fsecret"} {
		_, body, _ := get(t, app, target)
		if strings.Contains(body, "SECRET") || strings.Contains(body, "module ") {
			t.Fatalf("%s leaked file contents: %q", target, body)
		}
	}
}

func TestWebAppPlaceholderWhenNotBuilt(t *testing.T) {
	app := fiber.New()
	register(app, fstest.MapFS{".gitkeep": {}}, "")
	status, body, _ := get(t, app, "/app/")
	if status != 200 || !strings.Contains(body, "npm run build") {
		t.Fatalf("status %d body %q", status, body)
	}
}
