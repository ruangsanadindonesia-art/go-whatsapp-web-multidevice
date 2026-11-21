package cmd

import (
	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/gofiber/fiber/v3"
)

// webAppIdentity is what the /app SPA needs to know about the signed-in account.
type webAppIdentity struct {
	Username string   `json:"username"`
	Role     string   `json:"role"`    // "admin" or "cs"
	Devices  []string `json:"devices"` // allowed devices for "cs"; empty = all (admin)
}

// registerWebAppAPI mounts the device-free /api/app endpoints. Device-scoped
// ones (analytics) live in ui/rest and are mounted on the device group.
func registerWebAppAPI(router fiber.Router) {
	router.Get("/api/app/me", func(c fiber.Ctx) error {
		user := authUsername(c)
		id := webAppIdentity{Username: user, Role: "admin", Devices: []string{}}
		if isRestrictedAccount(user) {
			id.Role = "cs"
			id.Devices = append([]string{}, config.AppBasicAuthDevices[user]...)
		}
		c.Set(fiber.HeaderCacheControl, "no-store")
		return c.JSON(utils.ResponseData{Status: 200, Code: "SUCCESS", Message: "identity", Results: id})
	})
}
