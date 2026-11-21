// Package webapp serves the Ruang Sanad /app single-page app. The SPA is
// built from web/ into dist/ (see web/vite.config.ts and the Docker web
// stage) and embedded into the binary. A checkout without a build only has
// dist/.gitkeep; the route then serves a placeholder page.
package webapp

import (
	"bytes"
	"embed"
	"html"
	"io/fs"
	"mime"
	"path"
	"strings"

	"github.com/gofiber/fiber/v3"
)

//go:embed all:dist
var distFS embed.FS

const placeholderHTML = `<!doctype html><html lang="id"><head><meta charset="utf-8"><title>Ruang Sanad</title></head>` +
	`<body style="font-family:system-ui;padding:2rem;color:#16484a;background:#fcf6ea">` +
	`<p>UI /app belum di-build. Jalankan <code>npm run build</code> di <code>web/</code>.</p></body></html>`

// Register mounts GET {basePath}/app and {basePath}/app/* on router. basePath
// is APP_BASE_PATH ("" when unset); router must already be that group.
func Register(router fiber.Router, basePath string) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err) // unreachable: dist is embedded at build time
	}
	register(router, sub, basePath)
}

func register(router fiber.Router, fsys fs.FS, basePath string) {
	baseHref := strings.TrimRight(basePath, "/") + "/app/"
	index := loadIndex(fsys, baseHref)

	serveIndex := func(c fiber.Ctx) error {
		setSecurityHeaders(c)
		c.Set(fiber.HeaderCacheControl, "no-cache")
		c.Type("html", "utf-8")
		return c.Send(index)
	}

	router.Get("/app", serveIndex)
	router.Get("/app/*", func(c fiber.Ctx) error {
		rel := strings.TrimPrefix(path.Clean("/"+c.Params("*")), "/")
		if rel == "" || rel == "index.html" {
			return serveIndex(c)
		}
		if !fs.ValidPath(rel) {
			return c.SendStatus(fiber.StatusNotFound)
		}
		data, err := fs.ReadFile(fsys, rel)
		if err != nil {
			// Unknown asset files are real 404s; every other path is a client route.
			if strings.HasPrefix(rel, "assets/") || path.Ext(rel) != "" {
				return c.SendStatus(fiber.StatusNotFound)
			}
			return serveIndex(c)
		}
		setSecurityHeaders(c)
		if strings.HasPrefix(rel, "assets/") {
			c.Set(fiber.HeaderCacheControl, "public, max-age=31536000, immutable")
		} else {
			c.Set(fiber.HeaderCacheControl, "no-cache")
		}
		if ct := mime.TypeByExtension(path.Ext(rel)); ct != "" {
			c.Set(fiber.HeaderContentType, ct)
		}
		return c.Send(data)
	})
}

func loadIndex(fsys fs.FS, baseHref string) []byte {
	data, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		return []byte(placeholderHTML)
	}
	tag := []byte(`<base href="` + html.EscapeString(baseHref) + `">`)
	if i := bytes.Index(bytes.ToLower(data), []byte("<head>")); i >= 0 {
		i += len("<head>")
		out := make([]byte, 0, len(data)+len(tag))
		out = append(out, data[:i]...)
		out = append(out, tag...)
		return append(out, data[i:]...)
	}
	return append(tag, data...)
}

func setSecurityHeaders(c fiber.Ctx) {
	c.Set(fiber.HeaderXContentTypeOptions, "nosniff")
	c.Set(fiber.HeaderXFrameOptions, "DENY")
	c.Set("Referrer-Policy", "same-origin")
}
