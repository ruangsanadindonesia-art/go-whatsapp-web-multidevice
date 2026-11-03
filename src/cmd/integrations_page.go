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

// The managed GoWA browser UI is published from a separate upstream repository.
// Keep this administrator-only page in the backend so its access control and
// release do not depend on changing the upstream gowa-ui bundle.
//
// It is intentionally read-only. Never expose a "create API key" control before
// the key's corresponding authentication, device scope and action enforcement
// are implemented. A cosmetic UI would be a dangerous false assurance.
type integrationAccountView struct {
	Username string
	Scope    string
	Devices  string
}

type integrationsPageData struct {
	Base           string
	Tab            string
	Username       string
	Accounts       []integrationAccountView
	Clients        []privateAPIKey
	APIKeysEnabled bool
	NowUnix        int64
	MCPEnabled     bool
	OAuthEnabled   bool
	MCPPath        string
	APIClients     string
	MCPAuth        string
	LegacyCount    int
	ScopedCount    int
}

func integrationsPageDataFor(username, tab string) integrationsPageData {
	data := integrationsPageData{
		Base:         strings.TrimRight(config.AppBasePath, "/"),
		Tab:          tab,
		Username:     username,
		MCPEnabled:   config.McpEnabled,
		OAuthEnabled: config.McpEnabled && config.McpOAuthEnabled,
		MCPPath:      strings.TrimRight(config.AppBasePath, "/") + "/mcp",
		APIClients:   "Private API v1 menggunakan Bearer Key; REST existing tetap Basic Auth.",
		NowUnix:      time.Now().UTC().Unix(),
		MCPAuth:      "Basic Auth (legacy)",
		LegacyCount:  len(config.AppBasicAuthCredential),
	}
	if data.OAuthEnabled {
		data.MCPAuth = "OAuth 2.1 + Basic Auth (legacy)"
	}
	for _, credential := range config.AppBasicAuthCredential {
		account, _, ok := strings.Cut(credential, ":")
		if !ok || account == "" {
			continue
		}
		entry := integrationAccountView{Username: account, Scope: "Full access (legacy)", Devices: "Semua device"}
		if devices, restricted := config.AppBasicAuthDevices[account]; restricted {
			entry.Scope = "Device-scoped REST"
			entry.Devices = strings.Join(devices, ", ")
			data.ScopedCount++
		}
		data.Accounts = append(data.Accounts, entry)
	}
	sort.Slice(data.Accounts, func(i, j int) bool {
		return data.Accounts[i].Username < data.Accounts[j].Username
	})
	return data
}

func integrationsAdminAllowed(username string) bool {
	if username == "" { return false }
	for _, user := range strings.Split(config.AppIntegrationsAdminUsersRaw, ",") {
		if strings.TrimSpace(user) == username { return true }
	}
	return false
}

func registerIntegrationsPage(router fiber.Router, stores ...*privateAPIStore) {
	var keyStore *privateAPIStore
	if len(stores) > 0 { keyStore = stores[0] }
	router.Get("/integrations", func(c fiber.Ctx) error {
		c.Set("Cache-Control", "no-store")
		c.Set("Referrer-Policy", "no-referrer")
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("Content-Security-Policy",
			"default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")

		// An installation without Basic Auth must never expose admin inventory.
		// The ordinary GoWA web/Basic middleware runs before this route.
		if len(config.AppBasicAuthCredential) == 0 {
			return c.Status(fiber.StatusServiceUnavailable).SendString("API/MCP Management requires APP_BASIC_AUTH")
		}
		username := authUsername(c)
		if !integrationsAdminAllowed(username) || isRestrictedAccount(username) {
			return c.SendStatus(fiber.StatusForbidden)
		}

		tab := "api"
		if c.Query("tab") == "mcp" {
			tab = "mcp"
		}
		data := integrationsPageDataFor(username, tab)
		if keyStore != nil {
			clients, err := keyStore.List(c.Context())
			if err != nil { return c.SendStatus(fiber.StatusServiceUnavailable) }
			data.Clients, data.APIKeysEnabled = clients, true
		}
		var out bytes.Buffer
		if err := integrationsPageTemplate.Execute(&out, data); err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		c.Type("html")
		return c.Send(out.Bytes())
	})
}

var integrationsPageTemplate = template.Must(template.New("integrations").Parse(`<!doctype html>
<html lang="id"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="color-scheme" content="light">
<title>API &amp; MCP Management | GoWA</title>
<style>
:root{font-family:system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;color:#203c36;background:#f6f5f0}
*{box-sizing:border-box}body{margin:0}a{color:inherit}button,input{font:inherit}
.shell{max-width:1160px;margin:0 auto;padding:0 30px 70px}
.top{height:72px;border-bottom:1px solid #dce2dc;background:#fff;display:flex;align-items:center}
.topinner{max-width:1160px;width:100%;margin:auto;padding:0 30px;display:flex;align-items:center;justify-content:space-between;gap:16px}
.brand{display:flex;align-items:center;gap:12px;text-decoration:none;font-weight:800;letter-spacing:.035em}
.logo{width:38px;height:38px;background:#28564c;color:#fff;border-radius:10px;display:grid;place-items:center;font-family:Georgia,serif;font-size:23px}
.topright{font-size:13px;color:#62766d;display:flex;gap:20px;align-items:center}
.topright a{text-decoration:none;color:#28564c;font-weight:600}
.eyebrow{font-weight:700;text-transform:uppercase;letter-spacing:.12em;color:#6e857a;font-size:11px;margin:38px 0 10px}
h1{font-size:clamp(28px,3.4vw,41px);line-height:1.15;letter-spacing:-.035em;margin:0 0 9px}
.lede{font-size:15px;color:#5b7267;margin:0;max-width:720px;line-height:1.7}
.tabs{display:flex;gap:6px;margin:32px 0 22px;border-bottom:1px solid #dce2dc}
.tab{padding:13px 20px;font-size:14px;font-weight:650;text-decoration:none;color:#687a71;border-bottom:3px solid transparent}
.tab.active{color:#24564c;border-bottom-color:#28564c}
.grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:14px}
.card{background:#fff;border:1px solid #dce2dc;border-radius:13px;padding:23px}
.kicker{font-size:12px;color:#697f74;margin-bottom:12px}
.metric{font-size:26px;font-weight:760;letter-spacing:-.035em}
.metric.small{font-size:17px;letter-spacing:0}
.sub{color:#73857b;font-size:12px;margin-top:8px;line-height:1.5}
.section{display:flex;justify-content:space-between;align-items:end;gap:12px;margin:31px 0 13px}
h2{font-size:19px;letter-spacing:-.025em;margin:0}
.section p{margin:4px 0 0;color:#74857b;font-size:13px}
.link{color:#28564c;font-weight:650;text-decoration:none;font-size:13px}
.link:hover{text-decoration:underline}
.note{border-radius:11px;border:1px solid #e6d9b0;background:#fffbef;color:#61522b;padding:16px 19px;font-size:13px;line-height:1.65;margin:16px 0}
.note strong{display:block;margin-bottom:4px}
.table-wrap{overflow:auto;background:#fff;border:1px solid #dce2dc;border-radius:12px}
table{width:100%;border-collapse:collapse;text-align:left;font-size:13px;min-width:600px}
th{font-size:11px;text-transform:uppercase;letter-spacing:.07em;color:#78877e;background:#f8faf8;padding:14px 17px;border-bottom:1px solid #dce2dc}
td{padding:16px 17px;border-bottom:1px solid #edf0eb}
tr:last-child td{border-bottom:0}
.mono{font-family:ui-monospace,Menlo,Consolas,monospace;font-size:12px;word-break:break-all}
.pill{display:inline-block;border:1px solid #d1e2d6;background:#f0f7f2;border-radius:100px;color:#315f48;font-weight:650;font-size:11px;padding:5px 10px}
.pill.warn{border-color:#e8dfbb;background:#fbf6e6;color:#806521}
.row{display:flex;justify-content:space-between;gap:15px;padding:15px 0;border-bottom:1px solid #edf0eb;font-size:13px}
.row:last-child{border-bottom:0}
.row span:first-child{color:#6b8075}
.row strong{font-weight:650;text-align:right}
.two{display:grid;grid-template-columns:1fr 1fr;gap:14px}
ul{margin:12px 0 0;padding-left:19px;color:#50645a;font-size:13px;line-height:1.9}
.tool-list{display:grid;grid-template-columns:1fr 1fr;gap:9px;padding:0;list-style:none}
.tool-list li{border:1px solid #e2eae2;border-radius:9px;padding:9px 12px;background:#fafcf9;font:12px ui-monospace,Menlo,Consolas,monospace}
form.api-form{display:grid;grid-template-columns:1fr 1fr;gap:14px;margin-top:18px}.api-form label{display:flex;flex-direction:column;gap:6px;color:#53685d;font-size:12px;font-weight:650}.api-form input[type=text]{border:1px solid #cddbd2;border-radius:8px;padding:11px 12px;font-size:14px;background:#fff;color:#213e35;max-width:100%}.api-form .full{grid-column:1/-1}.api-form .check{display:flex;flex-direction:row;align-items:center;gap:10px;line-height:1.6}.api-form button,button.revoke{border:0;background:#28564c;color:white;border-radius:8px;padding:10px 16px;cursor:pointer;font-weight:700}.api-form button{width:fit-content}.revoke{background:#a44235!important;font-size:12px}button:hover{filter:brightness(.92)}.inline-form{display:inline-block}footer{font-size:12px;color:#7c897e;border-top:1px solid #e0e5df;margin-top:34px;padding-top:19px}
@media(max-width:730px){.shell{padding:0 17px 48px}.topinner{padding:0 17px}.topright span{display:none}.grid,.two,.api-form{grid-template-columns:1fr}.tabs{overflow-x:auto}.card{padding:18px}}
</style></head><body>
<header class="top"><div class="topinner">
<a class="brand" href="{{.Base}}/"><span class="logo">G</span><span>GoWA <span style="font-weight:450;color:#799086">/ Integrations</span></span></a>
<div class="topright"><span>Login: {{.Username}}</span><a href="{{.Base}}/">Dashboard</a></div>
</div></header>
<main class="shell">
<p class="eyebrow">Platform administration</p>
<h1>API &amp; MCP Management</h1>
<p class="lede">Kelola API Key privat untuk aplikasi eksternal dan pantau konektivitas REST API serta MCP. API Key memiliki izin perangkat khusus, terpisah dari akun login GoWA.</p>
<nav class="tabs" aria-label="Integrasi">
<a class="tab {{if eq .Tab "api"}}active{{end}}" href="{{.Base}}/integrations?tab=api" {{if eq .Tab "api"}}aria-current="page"{{end}}>REST API</a>
<a class="tab {{if eq .Tab "mcp"}}active{{end}}" href="{{.Base}}/integrations?tab=mcp" {{if eq .Tab "mcp"}}aria-current="page"{{end}}>MCP / AI Agents</a>
</nav>
{{if eq .Tab "api"}}
<div class="grid">
<section class="card"><div class="kicker">Authentication API v1</div><div class="metric small">Bearer API Key</div><p class="sub">Scoped dan hashed; REST legacy tetap Basic Auth</p></section>
<section class="card"><div class="kicker">Akun Basic terkonfigurasi</div><div class="metric">{{.LegacyCount}}</div><p class="sub">Termasuk akun administrator dan service</p></section>
<section class="card"><div class="kicker">Akun dengan batas device</div><div class="metric">{{.ScopedCount}}</div><p class="sub">Menggunakan APP_BASIC_AUTH_DEVICES</p></section>
</div>
<div class="note"><strong>Private REST API v1</strong>Setiap key berlaku 90 hari, untuk satu device. Default hanya kirim pesan personal dan membaca status; kirim grup perlu izin terpisah. Idempotency-Key wajib ketika mengirim pesan. Webhook subscription dan izin granular MCP belum tersedia. Jangan gunakan key di frontend browser.</div>
{{if .APIKeysEnabled}}
<div class="section"><div><h2>Buat API Key</h2><p>Secret ditampilkan sekali. Pastikan device ID sesuai dengan perangkat pada dashboard.</p></div></div>
<section class="card">
<form class="api-form" method="post" action="{{.Base}}/integrations/keys">
<label>Nama integrasi <input name="name" type="text" maxlength="80" placeholder="n8n Ruang Sanad" autocomplete="off" required></label>
<label>Device ID <input name="device_id" type="text" maxlength="255" placeholder="ID device GoWA" autocomplete="off" required></label>
<label class="check full"><input name="allow_group" type="checkbox" value="yes"> Izinkan pengiriman grup otomatis (hanya untuk integrasi terpercaya)</label>
<div class="full"><button type="submit">Generate API Key</button></div>
</form>
</section>
<div class="section"><div><h2>API Keys</h2><p>Kelola client, device, masa berlaku, dan pencabutan akses.</p></div></div>
<div class="table-wrap"><table><thead><tr><th>Client</th><th>Device</th><th>Izin</th><th>Status</th><th>Aksi</th></tr></thead><tbody>
{{range .Clients}}
<tr><td><strong>{{.Name}}</strong><div class="sub mono">gwa_{{.Prefix}}_••••</div></td><td class="mono">{{.DeviceID}}</td><td>{{if .CanGroup}}Personal + grup otomatis{{else}}Personal saja{{end}}<div class="sub">Status device/pesan</div></td>
<td>{{if .RevokedAt.Valid}}<span class="pill warn">Revoked</span>{{else if le .ExpiresAt $.NowUnix}}<span class="pill warn">Expired</span>{{else}}<span class="pill">Active</span>{{end}}<div class="sub">Expires: {{.ExpiresAt}} (Unix UTC)</div></td>
<td>{{if not .RevokedAt.Valid}}<form class="inline-form" method="post" action="{{$.Base}}/integrations/keys/{{.ID}}/revoke"><button class="revoke" type="submit">Revoke</button></form>{{else}}-{{end}}</td></tr>
{{else}}<tr><td colspan="5">Belum ada API Key. Buat client di atas.</td></tr>{{end}}
</tbody></table></div>
{{else}}<div class="note">API key storage tidak dikonfigurasi pada proses ini.</div>{{end}}
<div class="section"><div><h2>Existing API identities</h2><p>Hanya metadata identitas, tidak menampilkan password atau token.</p></div></div>
<div class="table-wrap"><table><thead><tr><th>Username</th><th>Akses existing</th><th>Device</th><th>Keterangan</th></tr></thead><tbody>
{{range .Accounts}}<tr><td class="mono">{{.Username}}</td><td>{{.Scope}}</td><td class="mono">{{.Devices}}</td><td><span class="pill warn">Legacy</span></td></tr>{{else}}<tr><td colspan="4">Belum ada akun Basic Auth.</td></tr>{{end}}
</tbody></table></div>
<div class="section"><div><h2>API Documentation</h2><p>OpenAPI existing menggunakan Basic Auth. Endpoint API Key v1 menggunakan kontrak terpisah.</p></div></div>
<div class="two">
<div class="card"><h2>OpenAPI reference</h2><p class="sub">Endpoint send, chat, device, group, media, dan schedule terdokumentasi pada repository.</p><p><a class="link" href="https://github.com/ruangsanadindonesia-art/go-whatsapp-web-multidevice/blob/main/docs/openapi.yaml">Lihat OpenAPI existing</a></p></div>
<div class="card"><h2>API v1 roadmap</h2><p class="sub">API v1 tersedia untuk text send, message status dan device status. Webhook subscriptions belum tersedia.</p><p><a class="link" href="https://github.com/ruangsanadindonesia-art/go-whatsapp-web-multidevice/blob/main/docs/prd-gowa-external-api-platform.md">Lihat PRD API privat</a></p></div>
</div>
{{else}}
<div class="grid">
<section class="card"><div class="kicker">MCP gateway</div><div class="metric small">{{if .MCPEnabled}}Enabled{{else}}Disabled{{end}}</div><p class="sub">MCP_ENABLED pada proses aktif</p></section>
<section class="card"><div class="kicker">OAuth MCP</div><div class="metric small">{{if .OAuthEnabled}}Enabled{{else}}Disabled{{end}}</div><p class="sub">MCP_OAUTH_ENABLED dan MCP_ENABLED</p></section>
<section class="card"><div class="kicker">Tools terkonsolidasi</div><div class="metric">6</div><p class="sub">MCP tool definitions di source saat ini</p></section>
</div>
<div class="note"><strong>Permission per-agent belum diberlakukan.</strong>OAuth existing bukan bukti bahwa sebuah agent hanya bisa mengakses device/tool tertentu. Akun Basic yang restricted saat ini ditolak di MCP. Jangan memberikan akses unrestricted ke agent eksternal sebelum policy granular terverifikasi.</div>
<div class="section"><div><h2>Connection details</h2><p>Informasi non-rahasia dari konfigurasi aktif.</p></div></div>
<div class="two">
<div class="card">
<div class="row"><span>Resource path</span><strong class="mono">{{.MCPPath}}</strong></div>
<div class="row"><span>Authentication</span><strong>{{.MCPAuth}}</strong></div>
<div class="row"><span>Device-scoped OAuth access</span><strong>Not implemented</strong></div>
<div class="row"><span>Granular tool permission</span><strong>Not implemented</strong></div>
</div>
<div class="card"><h2>Registered tool categories</h2><ul class="tool-list"><li>whatsapp_send</li><li>whatsapp_schedule</li><li>whatsapp_message</li><li>whatsapp_chat</li><li>whatsapp_group</li><li>whatsapp_app</li></ul></div>
</div>
<div class="section"><div><h2>MCP integration guide</h2><p>Konfigurasi OAuth dan klien Claude / Hermes.</p></div></div>
<div class="card"><p class="sub">Remote MCP memerlukan HTTPS publik dan OAuth yang dikonfigurasi benar. Izin setiap tool dan device harus diterapkan server-side sebelum diberikan ke pengguna eksternal.</p><a class="link" href="https://github.com/ruangsanadindonesia-art/go-whatsapp-web-multidevice/blob/main/docs/mcp-oauth.md">Lihat panduan MCP OAuth</a></div>
{{end}}
<footer>GoWA Integration Administration · Private API Keys · No plaintext secrets stored</footer>
</main></body></html>`))
