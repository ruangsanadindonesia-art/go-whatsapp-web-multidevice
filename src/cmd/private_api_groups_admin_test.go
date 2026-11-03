package cmd

import (
	"context"
	"io"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	"github.com/gofiber/fiber/v3"
)

func TestPrivateAPIKeyGroupsStore(t *testing.T) {
	store, err := openPrivateAPIStore(filepath.Join(t.TempDir(), "private.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()
	key, _, err := store.Create(context.Background(), "hermes", "cs-ai", true, privateAPIToolGrants{MCP: true})
	if err != nil { t.Fatal(err) }
	if err := store.SetKeyGroups(context.Background(), key.ID, []privateAPIGroupEntry{{JID: "not-a-group"}}); err == nil {
		t.Fatal("invalid JID accepted")
	}
	if err := store.SetKeyGroups(context.Background(), key.ID, []privateAPIGroupEntry{{JID: "120363239125791211@g.us", Name: "Tim"}}); err != nil {
		t.Fatal(err)
	}
	groups, _ := store.KeyGroups(context.Background(), key.ID)
	counts, _ := store.GroupCounts(context.Background())
	if len(groups) != 1 || groups[0].Name != "Tim" || counts[key.ID] != 1 {
		t.Fatalf("groups=%+v counts=%v", groups, counts)
	}
	if err := store.SetKeyGroups(context.Background(), key.ID, nil); err != nil { t.Fatal(err) }
	if groups, _ := store.KeyGroups(context.Background(), key.ID); len(groups) != 0 {
		t.Fatal("clearing the list failed")
	}
}

func TestPrivateAPIGroupAdminPage(t *testing.T) {
	prevCredentials, prevAdmins, prevScopes := config.AppBasicAuthCredential, config.AppIntegrationsAdminUsersRaw, config.AppBasicAuthDevices
	t.Cleanup(func() {
		config.AppBasicAuthCredential, config.AppIntegrationsAdminUsersRaw, config.AppBasicAuthDevices = prevCredentials, prevAdmins, prevScopes
	})
	config.AppBasicAuthCredential = []string{"admin:example", "other:example"}
	config.AppIntegrationsAdminUsersRaw = "admin"
	config.AppBasicAuthDevices = map[string][]string{}
	store, err := openPrivateAPIStore(filepath.Join(t.TempDir(), "private.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()
	ctx := context.Background()
	key, _, _ := store.Create(ctx, "hermes", "cs-ai", true, privateAPIToolGrants{MCP: true})
	plain, _, _ := store.Create(ctx, "n8n", "cs-ai", false, privateAPIToolGrants{REST: true})

	user := "admin"
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error { c.Locals(webAuthUserKey, user); return c.Next() })
	registerPrivateAPIGroupAdmin(app, store, privateAPIBackend{
		Groups: func(ctx context.Context, device string) ([]privateAPIGroupEntry, error) {
			return []privateAPIGroupEntry{{JID: "120363239125791211@g.us", Name: "Tim Inti"}, {JID: "120363000000000001@g.us", Name: "Alumni"}}, nil
		},
	})
	do := func(method, path, origin string, form url.Values) (int, string) {
		t.Helper()
		var body io.Reader
		if form != nil { body = strings.NewReader(form.Encode()) }
		req := httptest.NewRequest(method, path, body)
		req.Host = "wa.example.com"
		if origin != "" { req.Header.Set("Origin", origin) }
		if form != nil { req.Header.Set("Content-Type", "application/x-www-form-urlencoded") }
		resp, err := app.Test(req)
		if err != nil { t.Fatal(err) }
		data, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(data)
	}
	path := "/integrations/keys/" + key.ID + "/groups"

	code, body := do("GET", path, "", nil)
	if code != 200 || !strings.Contains(body, "Tim Inti") || !strings.Contains(body, "Alumni") || strings.Contains(body, "checked") {
		t.Fatalf("page: %d %s", code, body)
	}
	form := url.Values{"jid": {"120363239125791211@g.us", "120363999999999999@g.us"}}
	if code, _ := do("POST", path, "https://evil.example", form); code != 403 {
		t.Fatalf("cross-origin save must be refused, got %d", code)
	}
	if code, _ := do("POST", path, "https://wa.example.com", form); code != 303 && code != 302 {
		t.Fatalf("save: %d", code)
	}
	groups, _ := store.KeyGroups(ctx, key.ID)
	if len(groups) != 1 || groups[0].JID != "120363239125791211@g.us" || groups[0].Name != "Tim Inti" {
		t.Fatalf("only joined groups may be saved: %+v", groups)
	}

	// A key without MCP + Group grants cannot get a group list.
	if code, body := do("GET", "/integrations/keys/"+plain.ID+"/groups", "", nil); code != 200 || !strings.Contains(body, "tidak punya izin") {
		t.Fatalf("plain key page: %d", code)
	}
	if code, _ := do("POST", "/integrations/keys/"+plain.ID+"/groups", "https://wa.example.com", form); code != 400 {
		t.Fatalf("plain key save: %d", code)
	}
	user = "other"
	if code, _ := do("GET", path, "", nil); code != 403 {
		t.Fatalf("non-admin must be refused, got %d", code)
	}
}
