package cmd

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	"github.com/gofiber/fiber/v3"
)

func TestIntegrationsPageFailClosedAndScoped(t *testing.T) {
	prevCredentials := config.AppBasicAuthCredential
	prevDevices := config.AppBasicAuthDevices
	prevAdmins := config.AppIntegrationsAdminUsersRaw
	t.Cleanup(func() {
		config.AppBasicAuthCredential = prevCredentials
		config.AppBasicAuthDevices = prevDevices
		config.AppIntegrationsAdminUsersRaw = prevAdmins
	})

	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals(webAuthUserKey, "admin")
		return c.Next()
	})
	registerIntegrationsPage(app)

	config.AppBasicAuthCredential = nil
	response, err := app.Test(httptest.NewRequest("GET", "/integrations", nil))
	if err != nil { t.Fatal(err) }
	_ = response.Body.Close()
	if response.StatusCode != fiber.StatusServiceUnavailable {
		t.Fatalf("without Basic Auth got %d, want 503", response.StatusCode)
	}

	config.AppBasicAuthCredential = []string{"admin:secret"}
	config.AppIntegrationsAdminUsersRaw = "admin"
	config.AppBasicAuthDevices = map[string][]string{"admin": {"dev-a"}}
	response, err = app.Test(httptest.NewRequest("GET", "/integrations", nil))
	if err != nil { t.Fatal(err) }
	_ = response.Body.Close()
	if response.StatusCode != fiber.StatusForbidden {
		t.Fatalf("restricted account got %d, want 403", response.StatusCode)
	}
}

func TestIntegrationsPageDisplaysRealStateWithoutSecrets(t *testing.T) {
	prevCredentials := config.AppBasicAuthCredential
	prevDevices := config.AppBasicAuthDevices
	prevAdmins := config.AppIntegrationsAdminUsersRaw
	prevMCP := config.McpEnabled
	prevOAuth := config.McpOAuthEnabled
	t.Cleanup(func() {
		config.AppBasicAuthCredential = prevCredentials
		config.AppBasicAuthDevices = prevDevices
		config.AppIntegrationsAdminUsersRaw = prevAdmins
		config.McpEnabled = prevMCP
		config.McpOAuthEnabled = prevOAuth
	})

	config.AppBasicAuthCredential = []string{"admin:never-display-this", "service:other-secret"}
	config.AppIntegrationsAdminUsersRaw = "admin"
	config.AppBasicAuthDevices = map[string][]string{"service": {"device-finance"}}
	config.McpEnabled = true
	config.McpOAuthEnabled = false

	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals(webAuthUserKey, "admin")
		return c.Next()
	})
	registerIntegrationsPage(app)

	for _, tc := range []struct{ path, needle string }{
		{"/integrations", "device-finance"},
		{"/integrations?tab=mcp", "OAuth MCP"},
	} {
		response, err := app.Test(httptest.NewRequest("GET", tc.path, nil))
		if err != nil { t.Fatal(err) }
		content, err := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if err != nil { t.Fatal(err) }
		body := string(content)
		if response.StatusCode != fiber.StatusOK || !strings.Contains(body, tc.needle) {
			t.Fatalf("%s: status=%d, missing %q", tc.path, response.StatusCode, tc.needle)
		}
		if strings.Contains(body, "never-display-this") || strings.Contains(body, "other-secret") {
			t.Fatalf("%s: credential leaked in HTML", tc.path)
		}
		if response.Header.Get("Content-Security-Policy") == "" ||
			response.Header.Get("Cache-Control") != "no-store" {
			t.Fatalf("%s: missing page security headers", tc.path)
		}
	}
}

func TestIntegrationsPageEscapesConfiguredUsernames(t *testing.T) {
	prevCredentials := config.AppBasicAuthCredential
	prevDevices := config.AppBasicAuthDevices
	prevAdmins := config.AppIntegrationsAdminUsersRaw
	t.Cleanup(func() {
		config.AppBasicAuthCredential = prevCredentials
		config.AppBasicAuthDevices = prevDevices
		config.AppIntegrationsAdminUsersRaw = prevAdmins
	})

	config.AppBasicAuthCredential = []string{"admin:secret", "<script>:secret"}
	config.AppIntegrationsAdminUsersRaw = "admin"
	config.AppBasicAuthDevices = map[string][]string{}
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals(webAuthUserKey, "admin")
		return c.Next()
	})
	registerIntegrationsPage(app)
	response, err := app.Test(httptest.NewRequest("GET", "/integrations", nil))
	if err != nil { t.Fatal(err) }
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil { t.Fatal(err) }
	if response.StatusCode != fiber.StatusOK || strings.Contains(string(body), "<script>") ||
		!strings.Contains(string(body), "&lt;script&gt;") {
		t.Fatalf("HTML template did not escape configured username correctly")
	}
}
