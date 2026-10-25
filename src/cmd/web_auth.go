package cmd

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"html"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/sqlite"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/basicauth"
)

const (
	webSessionCookie   = "__Host-gowa_session"
	webCSRFCookie      = "__Host-gowa_login_csrf"
	webSessionLifetime = 14 * 24 * time.Hour
	webLoginLimit      = 10
	webLoginWindow     = 15 * time.Minute
	webAuthUserKey     = "gowa.auth.username"
)

const webSessionSchema = `
CREATE TABLE IF NOT EXISTS web_sessions (
	token_hash TEXT PRIMARY KEY,
	username TEXT NOT NULL,
	credential_proof TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	expires_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_web_sessions_expires ON web_sessions(expires_at);
`

// Only random token hashes and salted-by-token credential proofs are stored.
// Passwords and raw session tokens are never persisted in the browser store or DB.
type webSessionStore struct{ db *sql.DB }

func openWebSessionStore(path string) (*webSessionStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open(sqlite.DriverName, sqlite.FormatChatStorageURI("file:"+filepath.ToSlash(path), true, true))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(webSessionSchema); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &webSessionStore{db: db}, nil
}

func (s *webSessionStore) Close() error {
	if s == nil {
		return nil
	}
	return s.db.Close()
}

func secureRandomToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func webHash(v string) string {
	d := sha256.Sum256([]byte(v))
	return hex.EncodeToString(d[:])
}

func credentialProof(rawToken, password string) string {
	return webHash(rawToken + "\x00" + password)
}

func (s *webSessionStore) Create(ctx context.Context, username, password string, now time.Time) (string, error) {
	token, err := secureRandomToken()
	if err != nil {
		return "", err
	}
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO web_sessions(token_hash, username, credential_proof, created_at, expires_at) VALUES (?, ?, ?, ?, ?)`,
		webHash(token), username, credentialProof(token, password), now.Unix(), now.Add(webSessionLifetime).Unix()); err != nil {
		return "", err
	}
	_, _ = s.db.ExecContext(ctx, `DELETE FROM web_sessions WHERE expires_at <= ?`, now.Unix())
	return token, nil
}

func (s *webSessionStore) Identity(ctx context.Context, token string, passwords map[string]string, now time.Time) (string, bool) {
	if len(token) != 43 {
		return "", false
	}
	var username, proof string
	var expiry int64
	err := s.db.QueryRowContext(ctx, `SELECT username, credential_proof, expires_at FROM web_sessions WHERE token_hash = ?`,
		webHash(token)).Scan(&username, &proof, &expiry)
	if err != nil || now.Unix() >= expiry {
		return "", false
	}
	password, stillExists := passwords[username]
	if !stillExists || subtle.ConstantTimeCompare([]byte(credentialProof(token, password)), []byte(proof)) != 1 {
		// Password rotation or account removal immediately invalidates the old session.
		return "", false
	}
	return username, true
}

func (s *webSessionStore) Revoke(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM web_sessions WHERE token_hash = ?`, webHash(token))
	return err
}

type loginAttempt struct {
	count int
	since time.Time
}
type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string]loginAttempt
}

func newLoginLimiter() *loginLimiter { return &loginLimiter{attempts: map[string]loginAttempt{}} }
func (l *loginLimiter) allowed(ip string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.attempts) > 1000 {
		for key, v := range l.attempts {
			if now.Sub(v.since) >= webLoginWindow {
				delete(l.attempts, key)
			}
		}
	}
	v := l.attempts[webHash(ip)]
	if now.Sub(v.since) >= webLoginWindow {
		return true
	}
	return v.count < webLoginLimit
}
func (l *loginLimiter) failed(ip string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	k := webHash(ip)
	v := l.attempts[k]
	if now.Sub(v.since) >= webLoginWindow {
		v = loginAttempt{since: now}
	}
	v.count++
	l.attempts[k] = v
}
func (l *loginLimiter) reset(ip string) {
	l.mu.Lock()
	delete(l.attempts, webHash(ip))
	l.mu.Unlock()
}

func authUsername(c fiber.Ctx) string {
	if v, ok := c.Locals(webAuthUserKey).(string); ok && v != "" {
		return v
	}
	return basicauth.UsernameFromContext(c)
}

func secureWebCookie(name, value string, age time.Duration) *fiber.Cookie {
	return &fiber.Cookie{
		Name: name, Value: value, Path: "/", Secure: true, HTTPOnly: true,
		SameSite: "Lax", Expires: time.Now().Add(age), MaxAge: int(age.Seconds()),
	}
}

func clearWebCookie(c fiber.Ctx, name string) {
	c.Cookie(&fiber.Cookie{Name: name, Value: "", Path: "/", Secure: true, HTTPOnly: true,
		SameSite: "Lax", Expires: time.Unix(0, 0), MaxAge: -1})
}

func webLoginHTML(action, csrf, errorCode string, logout bool) string {
	title := "Masuk ke GoWA"
	button := "Masuk"
	form := `<label>Username<input name="username" autocomplete="username" required maxlength="100"></label>
<label>Password<input name="password" type="password" autocomplete="current-password" required></label>`
	if logout {
		title = "Keluar dari GoWA"
		button = "Keluar dari perangkat ini"
		form = "<p>Sesi login di browser ini akan diakhiri. Sesi pengguna lain tidak terpengaruh.</p>"
	}
	var notice string
	switch errorCode {
	case "invalid":
		notice = "Username atau password tidak sesuai."
	case "limited":
		notice = "Terlalu banyak percobaan. Coba kembali setelah 15 menit."
	case "expired":
		notice = "Sesi berakhir. Silakan masuk kembali."
	case "error":
		notice = "Tidak dapat memproses permintaan saat ini."
	}
	if notice != "" {
		notice = `<div class="notice" role="alert">` + html.EscapeString(notice) + `</div>`
	}
	return `<!doctype html><html lang="id"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="referrer" content="no-referrer"><title>` + html.EscapeString(title) + ` | Ruang Sanad</title>
<style>
:root{font-family:system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;color:#18342f;background:#f7f1e6}
*{box-sizing:border-box}body{margin:0;min-height:100svh;display:grid;place-items:center;padding:24px;background:radial-gradient(ellipse at top,#e9d9b9,#f7f1e6 65%)}
main{width:min(100%,410px);padding:36px 30px;border:1px solid #d4c6aa;background:#fffdf8;box-shadow:0 20px 65px #213a3017;border-radius:18px}
.brand{text-align:center;color:#28564c;font-weight:700;letter-spacing:.18em;font-size:.83rem}
h1{text-align:center;font-family:Georgia,serif;color:#28564c;font-size:1.85rem;margin:16px 0 8px}p.note{text-align:center;color:#64716b;font-size:.89rem;margin:0 0 25px}
label{display:block;margin:16px 0;color:#31584f;font-size:.87rem;font-weight:600}
input{display:block;margin:8px 0 0;width:100%;border:1px solid #cbd4ce;border-radius:10px;padding:12px 14px;font:inherit;background:white;color:#18342f;outline-offset:3px}
button{width:100%;margin-top:18px;border:0;border-radius:10px;background:#28564c;color:white;font:600 1rem system-ui;padding:13px;cursor:pointer}
button:hover{background:#1e473d}.notice{color:#9b3524;background:#fff0ed;border-radius:8px;padding:12px;font-size:.86rem;margin:16px 0}
footer{text-align:center;margin-top:22px;color:#8b8272;font-size:.74rem}
</style></head><body><main><div class="brand">RUANG SANAD</div>
<h1>` + html.EscapeString(title) + `</h1><p class="note">Akses aman dashboard WhatsApp</p>
` + notice + `<form method="post" action="` + html.EscapeString(action) + `">
<input type="hidden" name="csrf" value="` + html.EscapeString(csrf) + `">
` + form + `<button type="submit">` + html.EscapeString(button) + `</button></form>
<footer>Sesi berlaku 14 hari. Jangan gunakan perangkat bersama.</footer></main>
<script>try{localStorage.removeItem("gowa-ui.connection.v1")}catch(e){}</script>
</body></html>`
}

func safeSameOrigin(c fiber.Ctx) bool {
	origin := strings.TrimSpace(c.Get(fiber.HeaderOrigin))
	if origin == "" {
		return c.Get("Sec-Fetch-Site") == "same-origin"
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Path != "" {
		return false
	}
	return strings.EqualFold(parsed.Host, c.Get(fiber.HeaderHost))
}

// Register before global authentication, but after existing public OAuth,
// webhook and health endpoints. Only public login resources are exposed.
func registerWebLogin(app *fiber.App, accounts map[string]string, store *webSessionStore, base string) {
	root := strings.TrimRight(base, "/")
	if root == "" {
		root = ""
	}
	loginPath := root + "/login"
	postPath := root + "/auth/login"
	logoutPath := root + "/logout"
	logoutPost := root + "/auth/logout"
	home := root + "/"
	rate := newLoginLimiter()
	show := func(c fiber.Ctx, logout bool) error {
		c.Set("Cache-Control", "no-store")
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
		c.Set("Referrer-Policy", "no-referrer")
		tok, err := secureRandomToken()
		if err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		c.Cookie(secureWebCookie(webCSRFCookie, tok, 15*time.Minute))
		target := postPath
		if logout {
			target = logoutPost
		}
		c.Type("html")
		return c.SendString(webLoginHTML(target, tok, c.Query("error"), logout))
	}
	app.Get(loginPath, func(c fiber.Ctx) error { return show(c, false) })
	app.Post(postPath, func(c fiber.Ctx) error {
		c.Set("Cache-Control", "no-store")
		if !safeSameOrigin(c) || len(c.Body()) > 8192 ||
			subtle.ConstantTimeCompare([]byte(c.FormValue("csrf")), []byte(c.Cookies(webCSRFCookie))) != 1 ||
			c.Cookies(webCSRFCookie) == "" {
			return c.SendStatus(fiber.StatusForbidden)
		}
		clearWebCookie(c, webCSRFCookie)
		now := time.Now()
		if !rate.allowed(c.IP(), now) {
			return c.Redirect().To(loginPath + "?error=limited")
		}
		username, password := c.FormValue("username"), c.FormValue("password")
		expected, exists := accounts[username]
		if !exists || subtle.ConstantTimeCompare([]byte(password), []byte(expected)) != 1 {
			rate.failed(c.IP(), now)
			return c.Redirect().To(loginPath + "?error=invalid")
		}
		rate.reset(c.IP())
		token, err := store.Create(c.Context(), username, password, now)
		if err != nil {
			return c.Redirect().To(loginPath + "?error=error")
		}
		c.Cookie(secureWebCookie(webSessionCookie, token, webSessionLifetime))
		return c.Redirect().To(home)
	})
	app.Get(logoutPath, func(c fiber.Ctx) error {
		if _, ok := store.Identity(c.Context(), c.Cookies(webSessionCookie), accounts, time.Now()); !ok {
			return c.Redirect().To(loginPath)
		}
		return show(c, true)
	})
	app.Post(logoutPost, func(c fiber.Ctx) error {
		if !safeSameOrigin(c) || c.Cookies(webCSRFCookie) == "" ||
			subtle.ConstantTimeCompare([]byte(c.FormValue("csrf")), []byte(c.Cookies(webCSRFCookie))) != 1 {
			return c.SendStatus(fiber.StatusForbidden)
		}
		if err := store.Revoke(c.Context(), c.Cookies(webSessionCookie)); err != nil {
			return c.SendStatus(fiber.StatusServiceUnavailable)
		}
		clearWebCookie(c, webSessionCookie)
		clearWebCookie(c, webCSRFCookie)
		c.Set("Cache-Control", "no-store")
		return c.Redirect().To(loginPath)
	})
}

// Existing machine clients keep Basic credentials. Browser sessions are
// validated server-side, never converted to stored username/password strings.
// There is intentionally no WWW-Authenticate challenge: no native browser popup.
func newWebOrBasicAuthMiddleware(accounts map[string]string, store *webSessionStore, base string) fiber.Handler {
	login := strings.TrimRight(base, "/") + "/login"
	return func(c fiber.Ctx) error {
		header := strings.TrimSpace(c.Get(fiber.HeaderAuthorization))
		if strings.HasPrefix(strings.ToLower(header), "basic ") {
			decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(header[6:]))
			if err == nil {
				user, pass, ok := strings.Cut(string(decoded), ":")
				want, found := accounts[user]
				if ok && found && subtle.ConstantTimeCompare([]byte(want), []byte(pass)) == 1 {
					c.Locals(webAuthUserKey, user)
					return c.Next()
				}
			}
		}
		username, ok := store.Identity(c.Context(), c.Cookies(webSessionCookie), accounts, time.Now())
		if ok {
			// Cookie requests changing state need a same-origin browser signal.
			// Basic and machine integrations are not subject to browser CSRF.
			if c.Method() != fiber.MethodGet && c.Method() != fiber.MethodHead && c.Method() != fiber.MethodOptions && !safeSameOrigin(c) {
				return c.SendStatus(fiber.StatusForbidden)
			}
			if strings.EqualFold(c.Get("Upgrade"), "websocket") && !safeSameOrigin(c) {
				return c.SendStatus(fiber.StatusForbidden)
			}
			c.Locals(webAuthUserKey, username)
			return c.Next()
		}
		p := c.Path()
		htmlNav := c.Method() == fiber.MethodGet && (p == strings.TrimRight(base, "/")+"/" || p == strings.TrimRight(base, "/")+"/cs-ai" || strings.Contains(c.Get(fiber.HeaderAccept), "text/html"))
		c.Set("Cache-Control", "no-store")
		if htmlNav {
			return c.Redirect().To(login + "?error=expired")
		}
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
}

// Database location is under the already-persistent storages mount, not in Git.
func defaultWebSessionDB() string { return filepath.Join(config.PathStorages, "ui-sessions.db") }
