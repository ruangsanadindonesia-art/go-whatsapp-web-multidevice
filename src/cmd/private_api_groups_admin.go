package cmd

import (
	"bytes"
	"html/template"
	"sort"
	"strings"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	"github.com/gofiber/fiber/v3"
)

// MCP group checklist per API key (/integrations/keys/:id/groups).
//
// API-key MCP clients may send to a group only when the key has the Group
// grant AND the group is checked here. The list comes from the groups the
// key's device has joined, so administrators pick from real groups instead
// of typing JIDs into the server .env.

type groupChoice struct {
	JID     string
	Name    string
	Checked bool
	Missing bool // allowed earlier, but the device is no longer a member
}

type groupPageData struct {
	Base     string
	Key      privateAPIKey
	Usable   bool
	Reason   string
	Choices  []groupChoice
	LoadErr  string
	Saved    bool
}

func registerPrivateAPIGroupAdmin(router fiber.Router, store *privateAPIStore, backend privateAPIBackend) {
	allowed := func(c fiber.Ctx) bool {
		user := authUsername(c)
		return len(config.AppBasicAuthCredential) > 0 && integrationsAdminAllowed(user) && !isRestrictedAccount(user)
	}
	headers := func(c fiber.Ctx) {
		c.Set("Cache-Control", "no-store")
		c.Set("Referrer-Policy", webReferrerPolicy)
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
	}
	// loadKey returns the key and whether its group list can be edited.
	loadKey := func(c fiber.Ctx) (privateAPIKey, bool, string, int) {
		id := c.Params("id")
		if id == "" || len(id) > 80 { return privateAPIKey{}, false, "", fiber.StatusBadRequest }
		key, found, err := store.Get(c.Context(), id)
		if err != nil { return key, false, "", fiber.StatusServiceUnavailable }
		if !found { return key, false, "", fiber.StatusNotFound }
		switch {
		case key.RevokedAt.Valid || time.Now().UTC().Unix() >= key.ExpiresAt:
			return key, false, "Key ini sudah dicabut atau kedaluwarsa.", 0
		case !key.CanMCPTools || !key.CanGroup:
			return key, false, "Key ini tidak punya izin MCP Tools dan Group. Buat key baru dengan kedua centang itu.", 0
		}
		return key, true, "", 0
	}
	deviceGroups := func(c fiber.Ctx, key privateAPIKey) ([]privateAPIGroupEntry, error) {
		if backend.Groups == nil { return nil, fiber.ErrServiceUnavailable }
		return backend.Groups(c.Context(), key.DeviceID)
	}

	router.Get("/integrations/keys/:id/groups", func(c fiber.Ctx) error {
		headers(c)
		if !allowed(c) { return c.SendStatus(fiber.StatusForbidden) }
		key, usable, reason, status := loadKey(c)
		if status != 0 { return c.SendStatus(status) }
		data := groupPageData{Base: strings.TrimRight(config.AppBasePath, "/"), Key: key, Usable: usable, Reason: reason, Saved: c.Query("saved") == "1"}
		if usable {
			current, err := store.KeyGroups(c.Context(), key.ID)
			if err != nil { return c.SendStatus(fiber.StatusServiceUnavailable) }
			checked := map[string]bool{}
			for _, g := range current { checked[g.JID] = true }
			joined, err := deviceGroups(c, key)
			if err != nil {
				data.LoadErr = "Daftar grup device tidak bisa dimuat (device belum login atau terputus). Grup yang sudah diizinkan tetap tampil."
			}
			seen := map[string]bool{}
			for _, g := range joined {
				if seen[g.JID] || !privateAPIGroupJID.MatchString(g.JID) { continue }
				seen[g.JID] = true
				data.Choices = append(data.Choices, groupChoice{JID: g.JID, Name: g.Name, Checked: checked[g.JID]})
			}
			for _, g := range current {
				if !seen[g.JID] {
					data.Choices = append(data.Choices, groupChoice{JID: g.JID, Name: g.Name, Checked: true, Missing: err == nil})
				}
			}
			sort.SliceStable(data.Choices, func(i, j int) bool {
				if data.Choices[i].Checked != data.Choices[j].Checked { return data.Choices[i].Checked }
				return strings.ToLower(data.Choices[i].Name) < strings.ToLower(data.Choices[j].Name)
			})
		}
		var out bytes.Buffer
		if err := groupPageTemplate.Execute(&out, data); err != nil { return c.SendStatus(fiber.StatusInternalServerError) }
		c.Type("html")
		return c.Send(out.Bytes())
	})

	router.Post("/integrations/keys/:id/groups", func(c fiber.Ctx) error {
		headers(c)
		if !allowed(c) { return c.SendStatus(fiber.StatusForbidden) }
		if !safeSameOrigin(c) { return c.SendStatus(fiber.StatusForbidden) }
		if len(c.Body()) > 64*1024 { return c.SendStatus(fiber.StatusRequestEntityTooLarge) }
		key, usable, reason, status := loadKey(c)
		if status != 0 { return c.SendStatus(status) }
		if !usable { return c.Status(fiber.StatusBadRequest).SendString(reason) }

		// Names come from the device (or the stored list); a posted JID must
		// be a group the device joined or one already allowed.
		names := map[string]string{}
		current, err := store.KeyGroups(c.Context(), key.ID)
		if err != nil { return c.SendStatus(fiber.StatusServiceUnavailable) }
		for _, g := range current { names[g.JID] = g.Name }
		if joined, err := deviceGroups(c, key); err == nil {
			for _, g := range joined { names[g.JID] = g.Name }
		}
		var selected []privateAPIGroupEntry
		seen := map[string]bool{}
		for _, raw := range c.Request().PostArgs().PeekMulti("jid") {
			jid := strings.TrimSpace(string(raw))
			name, known := names[jid]
			if !known || seen[jid] { continue }
			seen[jid] = true
			selected = append(selected, privateAPIGroupEntry{JID: jid, Name: name})
		}
		if err := store.SetKeyGroups(c.Context(), key.ID, selected); err != nil {
			return c.Status(fiber.StatusBadRequest).SendString(err.Error())
		}
		return c.Redirect().To(strings.TrimRight(config.AppBasePath, "/") + "/integrations/keys/" + key.ID + "/groups?saved=1")
	})
}

var groupPageTemplate = template.Must(template.New("groups").Parse(`<!doctype html>
<html lang="id"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Grup MCP | GoWA</title>
<style>
body{font-family:system-ui,-apple-system,"Segoe UI",sans-serif;background:#f6f5f0;color:#203c36;margin:0;padding:32px 16px}
main{max-width:720px;margin:auto}
h1{font-size:26px;margin:0 0 6px;letter-spacing:-.02em}
p{line-height:1.6;color:#586c61;font-size:14px}
.card{background:#fff;border:1px solid #dce2dc;border-radius:13px;padding:20px;margin-top:16px}
.note{border-radius:11px;border:1px solid #e6d9b0;background:#fffbef;color:#61522b;padding:14px 16px;font-size:13px;line-height:1.6}
.ok{border-color:#cfe3d5;background:#f0f7f2;color:#2f5d46}
label.g{display:flex;gap:12px;align-items:flex-start;padding:12px 4px;border-bottom:1px solid #edf0eb;font-size:14px;cursor:pointer}
label.g:last-of-type{border-bottom:0}
label.g input{margin-top:3px;width:18px;height:18px}
.mono{font-family:ui-monospace,Menlo,Consolas,monospace;font-size:12px;color:#6b8075;word-break:break-all}
.miss{color:#a44235;font-size:12px}
button{border:0;background:#28564c;color:#fff;border-radius:8px;padding:11px 18px;font-weight:700;cursor:pointer;margin-top:14px}
a{color:#28564c;font-weight:650}
</style></head><body><main>
<p><a href="{{.Base}}/integrations?tab=api">← Kembali ke API Management</a></p>
<h1>Grup yang boleh dikirim lewat MCP</h1>
<p>Key <strong>{{.Key.Name}}</strong> · device <span class="mono">{{.Key.DeviceID}}</span></p>
{{if .Saved}}<div class="note ok">Tersimpan. Berlaku untuk panggilan MCP berikutnya, tanpa restart.</div>{{end}}
{{if not .Usable}}<div class="note">{{.Reason}}</div>{{else}}
<div class="note">Hanya grup yang dicentang yang bisa dikirimi pesan oleh klien MCP (mis. Hermes) dengan key ini. Mengelola grup (keluar, ubah nama, anggota) dan mengubah jadwal kirim tetap diblokir. Centang seperlunya.</div>
{{if .LoadErr}}<div class="note" style="margin-top:10px">{{.LoadErr}}</div>{{end}}
<form class="card" method="post" action="{{.Base}}/integrations/keys/{{.Key.ID}}/groups">
{{range .Choices}}
<label class="g"><input type="checkbox" name="jid" value="{{.JID}}" {{if .Checked}}checked{{end}}>
<span><strong>{{if .Name}}{{.Name}}{{else}}(tanpa nama){{end}}</strong><br><span class="mono">{{.JID}}</span>{{if .Missing}}<br><span class="miss">Device tidak lagi menjadi anggota grup ini</span>{{end}}</span></label>
{{else}}<p>Device ini belum tergabung di grup mana pun.</p>{{end}}
<button type="submit">Simpan</button>
</form>
{{end}}
</main></body></html>`))
