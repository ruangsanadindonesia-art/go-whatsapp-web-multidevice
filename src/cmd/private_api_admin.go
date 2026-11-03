package cmd

import (
	"context"
	"html"
	"strings"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	"github.com/gofiber/fiber/v3"
)

// Browser session and Basic auth are supplied by GoWA's existing middleware.
// All mutations enforce same-origin at the handler too (including Basic).
func registerPrivateAPIKeyAdmin(router fiber.Router, store *privateAPIStore, backend privateAPIBackend) {
	allowed := func(c fiber.Ctx) bool {
		user := authUsername(c)
		return len(config.AppBasicAuthCredential) > 0 &&
			integrationsAdminAllowed(user) && !isRestrictedAccount(user)
	}
	secure := func(c fiber.Ctx) {
		c.Set("Cache-Control","no-store")
		c.Set("Referrer-Policy","no-referrer")
		c.Set("X-Content-Type-Options","nosniff")
		c.Set("X-Frame-Options","DENY")
		c.Set("Content-Security-Policy","default-src 'none'; style-src 'unsafe-inline'; form-action 'none'; base-uri 'none'; frame-ancestors 'none'")
	}
	router.Post("/integrations/keys",func(c fiber.Ctx) error {
		secure(c)
		if !allowed(c) { return c.SendStatus(fiber.StatusForbidden) }
		if !safeSameOrigin(c) { return c.SendStatus(fiber.StatusForbidden) }
		if len(c.Body()) > 4096 { return c.SendStatus(fiber.StatusRequestEntityTooLarge) }
		if backend.Resolve == nil { return c.SendStatus(fiber.StatusServiceUnavailable) }
		name := strings.TrimSpace(c.FormValue("name"))
		requested := strings.TrimSpace(c.FormValue("device_id"))
		if requested == "" { return c.Status(400).SendString("device_id wajib diisi") }
		_, canonical, err := backend.Resolve(c.Context(),requested)
		if err != nil || canonical == "" { return c.Status(400).SendString("device_id tidak ditemukan") }
		allowGroup := c.FormValue("allow_group") == "yes"
		key, raw, err := store.Create(c.Context(),name,canonical,allowGroup)
		if err != nil { return c.Status(400).SendString("Nama integrasi atau perangkat tidak valid") }
		// Never redirect with the secret in a URL or persist the raw key.
		// Escaping and no-store protect the one-time view.
		page := "<!doctype html><html lang=\"id\"><head><meta charset=\"utf-8\">" +
			"<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">" +
			"<title>API Key dibuat | GoWA</title><style>" +
			"body{font-family:system-ui;background:#f6f5f0;color:#26483e;margin:0;padding:40px 16px}" +
			"main{max-width:680px;margin:auto;padding:32px;background:#fff;border-radius:16px;border:1px solid #dce2dc}" +
			"pre{white-space:pre-wrap;word-break:break-all;padding:16px;border-radius:10px;background:#f4f7f4;font-size:13px}" +
			"a{color:#28564c;font-weight:700}p{line-height:1.7;color:#586c61}</style></head><body><main>" +
			"<h1>API Key berhasil dibuat</h1><p>Integrasi: "+html.EscapeString(key.Name)+
			" | Device: "+html.EscapeString(key.DeviceID)+
			"</p><p><strong>Simpan sekarang. Key ini hanya ditampilkan satu kali.</strong>" +
			" Jangan bagikan atau tempelkan ke chat, log, dan frontend browser.</p>" +
			"<pre aria-label=\"One-time API Key\">"+html.EscapeString(raw)+"</pre>" +
			"<p>Authorization: Bearer (key di atas). Masa berlaku 90 hari.</p>" +
			"<a href=\""+html.EscapeString(strings.TrimRight(config.AppBasePath,"/"))+"/integrations?tab=api\">Kembali ke API Management</a>"+
			"</main></body></html>"
		c.Type("html")
		return c.SendString(page)
	})
	router.Post("/integrations/keys/:id/revoke",func(c fiber.Ctx) error {
		secure(c)
		if !allowed(c) { return c.SendStatus(fiber.StatusForbidden) }
		if !safeSameOrigin(c) { return c.SendStatus(fiber.StatusForbidden) }
		id := c.Params("id")
		if id == "" || len(id) > 80 { return c.SendStatus(fiber.StatusBadRequest) }
		revoked, err := store.Revoke(c.Context(),id)
		if err != nil { return c.SendStatus(fiber.StatusServiceUnavailable) }
		if !revoked { return c.SendStatus(fiber.StatusNotFound) }
		return c.Redirect().To(strings.TrimRight(config.AppBasePath,"/")+"/integrations?tab=api")
	})
}

// Verify production routes can use a context injected by DeviceManager.
var _ = context.Background
