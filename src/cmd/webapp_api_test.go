package cmd

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

func webAppMe(t *testing.T, user string) webAppIdentity {
	t.Helper()
	app := fiber.New()
	app.Use(newBasicAuthMiddleware(map[string]string{"admin": "pw", "cs": "pw"}))
	registerWebAppAPI(app)
	req := httptest.NewRequest("GET", "/api/app/me", nil)
	req.SetBasicAuth(user, "pw")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	b, _ := io.ReadAll(resp.Body)
	var out struct {
		Results webAppIdentity `json:"results"`
	}
	require.NoError(t, json.Unmarshal(b, &out), string(b))
	return out.Results
}

func TestWebAppMeReturnsRoleForRestrictedAccount(t *testing.T) {
	old := config.AppBasicAuthDevices
	t.Cleanup(func() { config.AppBasicAuthDevices = old })
	config.AppBasicAuthDevices = map[string][]string{"cs": {"dev1"}}

	me := webAppMe(t, "cs")
	require.Equal(t, "cs", me.Username)
	require.Equal(t, "cs", me.Role)
	require.Equal(t, []string{"dev1"}, me.Devices)
}

func TestWebAppMeAdmin(t *testing.T) {
	old := config.AppBasicAuthDevices
	t.Cleanup(func() { config.AppBasicAuthDevices = old })
	config.AppBasicAuthDevices = map[string][]string{"cs": {"dev1"}}

	me := webAppMe(t, "admin")
	require.Equal(t, "admin", me.Username)
	require.Equal(t, "admin", me.Role)
	require.Equal(t, []string{}, me.Devices)
}
