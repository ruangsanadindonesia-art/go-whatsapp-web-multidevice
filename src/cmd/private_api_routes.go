package cmd

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	domainSend "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/send"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
	"github.com/gofiber/fiber/v3"
)

// Keep the transport thin and testable. The production implementation reuses
// GoWA's existing device manager and send usecase (not a second WhatsApp client).
type privateAPIBackend struct {
	Resolve func(context.Context, string) (context.Context, string, error)
	Send    func(context.Context, domainSend.MessageRequest) (domainSend.GenericResponse, error)
	Status  func(context.Context, string) (bool, bool, error)
	// Groups lists the groups the device has joined (for the MCP group checklist).
	Groups  func(context.Context, string) ([]privateAPIGroupEntry, error)
}

func productionPrivateAPIBackend(dm *whatsapp.DeviceManager) privateAPIBackend {
	return privateAPIBackend{
		Resolve: func(ctx context.Context, deviceID string) (context.Context, string, error) {
			instance, canonical, err := dm.ResolveDevice(deviceID)
			if err != nil { return ctx, "", err }
			return whatsapp.ContextWithDevice(ctx, instance), canonical, nil
		},
		Send: sendUsecase.SendText,
		Status: deviceUsecase.GetStatus,
		Groups: func(ctx context.Context, deviceID string) ([]privateAPIGroupEntry, error) {
			instance, _, err := dm.ResolveDevice(deviceID)
			if err != nil { return nil, err }
			client := whatsapp.ClientFromContext(whatsapp.ContextWithDevice(ctx, instance))
			if client == nil || !client.IsLoggedIn() { return nil, errors.New("device belum login") }
			groups, err := client.GetJoinedGroups(ctx)
			if err != nil { return nil, err }
			out := make([]privateAPIGroupEntry, 0, len(groups))
			for _, g := range groups {
				if g == nil { continue }
				out = append(out, privateAPIGroupEntry{JID: g.JID.String(), Name: g.Name})
			}
			return out, nil
		},
	}
}

var privateAPIPersonalPhone = regexp.MustCompile("^[1-9][0-9]{7,17}$")
var privateAPIGroupJID = regexp.MustCompile("^[0-9-]{9,35}@g[.]us$")

func privateAPINormalizeRecipient(value string) (string, bool, bool) {
	if privateAPIGroupJID.MatchString(value) {
		return value, true, true
	}
	if strings.HasSuffix(value, "@s.whatsapp.net") {
		base := strings.TrimSuffix(value, "@s.whatsapp.net")
		if privateAPIPersonalPhone.MatchString(base) { return value, false, true }
		return "", false, false
	}
	if privateAPIPersonalPhone.MatchString(value) {
		return value+"@s.whatsapp.net", false, true
	}
	return "", false, false
}

func privateAPIError(c fiber.Ctx, status int, code string) error {
	c.Set("Cache-Control","no-store")
	return c.Status(status).JSON(fiber.Map{"error": code})
}

func registerPrivateIntegrationAPI(app fiber.Router, base string, store *privateAPIStore, backend privateAPIBackend) {
	prefix := strings.TrimRight(base,"/")+"/api/v1"
	auth := func(c fiber.Ctx) error {
		if len(config.AppBasicAuthCredential) == 0 {
			return privateAPIError(c, fiber.StatusServiceUnavailable, "INTEGRATION_DISABLED")
		}
		raw := strings.TrimSpace(c.Get(fiber.HeaderAuthorization))
		if !strings.HasPrefix(raw,"Bearer ") {
			return privateAPIError(c, fiber.StatusUnauthorized, "API_KEY_REQUIRED")
		}
		token := strings.TrimPrefix(raw,"Bearer ")
		key, valid, err := store.Authenticate(c.Context(), token)
		if err != nil { return privateAPIError(c, fiber.StatusServiceUnavailable, "KEY_STORE_UNAVAILABLE") }
		if !valid { return privateAPIError(c, fiber.StatusUnauthorized, "INVALID_API_KEY") }
		c.Locals("private_api_key", key)
		c.Set("Cache-Control","no-store")
		return c.Next()
	}
	readKey := func(c fiber.Ctx) privateAPIKey {
		key, _ := c.Locals("private_api_key").(privateAPIKey)
		return key
	}
	device := func(c fiber.Ctx, id string, key privateAPIKey) (context.Context, string, bool) {
		if id == "" || backend.Resolve == nil {
			return c.Context(), "", false
		}
		ctx, canonical, err := backend.Resolve(c.Context(),id)
		if err != nil || canonical == "" || canonical != key.DeviceID {
			return c.Context(),"",false
		}
		return ctx, canonical, true
	}
	app.Get(prefix+"/devices/:device_id/status",auth,func(c fiber.Ctx) error {
		key := readKey(c)
		if backend.Status == nil { return privateAPIError(c,503,"SERVICE_UNAVAILABLE") }
		_, canonical, ok := device(c,c.Params("device_id"),key)
		if !ok { return privateAPIError(c,403,"DEVICE_NOT_ALLOWED") }
		connected, loggedIn, err := backend.Status(c.Context(), canonical)
		if err != nil { return privateAPIError(c,503,"DEVICE_STATUS_UNAVAILABLE") }
		return c.JSON(fiber.Map{"device_id":canonical,"is_connected":connected,"is_logged_in":loggedIn})
	})
	app.Get(prefix+"/messages/:request_id",auth,func(c fiber.Ctx) error {
		key := readKey(c)
		record, found, err := store.GetSend(c.Context(), key.ID,c.Params("request_id"))
		if err != nil { return privateAPIError(c,503,"KEY_STORE_UNAVAILABLE") }
		if !found { return privateAPIError(c,404,"REQUEST_NOT_FOUND") }
		return c.JSON(fiber.Map{"request_id":record.ID,"state":record.State,"message_id":record.MessageID})
	})
	app.Post(prefix+"/messages",auth,func(c fiber.Ctx) error {
		key := readKey(c)
		if backend.Send == nil { return privateAPIError(c,503,"SERVICE_UNAVAILABLE") }
		if !strings.HasPrefix(strings.ToLower(c.Get("Content-Type")),"application/json") {
			return privateAPIError(c,415,"JSON_REQUIRED")
		}
		if len(c.Body()) == 0 || len(c.Body()) > 8192 {
			return privateAPIError(c,400,"INVALID_BODY")
		}
		var input struct {
			Phone string `json:"phone"`
			Message string `json:"message"`
		}
		decoder := json.NewDecoder(bytes.NewReader(c.Body()))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			return privateAPIError(c,400,"INVALID_BODY")
		}
		var extra any
		if err := decoder.Decode(&extra); !errors.Is(err,io.EOF) {
			return privateAPIError(c,400,"INVALID_BODY")
		}
		if strings.TrimSpace(input.Message)=="" || len(input.Message)>4096 {
			return privateAPIError(c,400,"INVALID_MESSAGE")
		}
		phone, group, valid := privateAPINormalizeRecipient(input.Phone)
		if !valid { return privateAPIError(c,400,"INVALID_RECIPIENT") }
		if group && !key.CanGroup {
			return privateAPIError(c,403,"GROUP_SEND_NOT_ALLOWED")
		}
		_, canonical, ok := device(c,c.Get("X-Device-Id"),key)
		if !ok { return privateAPIError(c,403,"DEVICE_NOT_ALLOWED") }
		idem := strings.TrimSpace(c.Get("Idempotency-Key"))
		if !privateAPIIdempotencyPattern.MatchString(idem) {
			return privateAPIError(c,400,"IDEMPOTENCY_KEY_REQUIRED")
		}
		fingerBytes := sha256.Sum256([]byte(canonical+"\x00"+phone+"\x00"+input.Message))
		fingerprint := hex.EncodeToString(fingerBytes[:])
		if !store.AllowSend(key.ID,time.Now()) {
			return privateAPIError(c,429,"RATE_LIMITED")
		}
		record, isNew, err := store.ReserveSend(c.Context(),key.ID,idem,fingerprint)
		if err != nil { return privateAPIError(c,503,"KEY_STORE_UNAVAILABLE") }
		if record.Fingerprint != fingerprint {
			return privateAPIError(c,409,"IDEMPOTENCY_CONFLICT")
		}
		if !isNew {
			return c.JSON(fiber.Map{"request_id":record.ID,"state":record.State,"message_id":record.MessageID,"replayed":true})
		}
		// Use the same canonical device when delegating. If the device has
		// disappeared after validation, mark UNCERTAIN and never retry blindly.
		sendCtx, resolved, err := backend.Resolve(c.Context(),canonical)
		var response domainSend.GenericResponse
		if err == nil && resolved == canonical {
			response,err = backend.Send(sendCtx,domainSend.MessageRequest{
				BaseRequest: domainSend.BaseRequest{Phone:phone},
				Message:input.Message,
			})
		} else if err == nil {
			err = errors.New("device changed during send")
		}
		state := "SENT"
		if err != nil { state = "UNCERTAIN" }
		// Even when the HTTP request times out, try to persist the terminal
		// state using a bounded independent context.
		finishCtx, cancel := context.WithTimeout(context.Background(),3*time.Second)
		finishErr := store.FinishSend(finishCtx,record.ID,state,response.MessageID)
		cancel()
		if finishErr != nil { return privateAPIError(c,503,"RECONCILIATION_REQUIRED") }
		if err != nil {
			return c.Status(202).JSON(fiber.Map{"request_id":record.ID,"state":state,"note":"Check status and reconcile before any new send"})
		}
		return c.JSON(fiber.Map{"request_id":record.ID,"state":state,"message_id":response.MessageID})
	})
}
