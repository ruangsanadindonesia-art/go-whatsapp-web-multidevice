package rest

import (
	"errors"
	"mime"
	"path/filepath"
	"time"

	domainCampaign "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/campaign"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/gofiber/fiber/v3"
)

// maxCampaignRangeDays caps dashboard/calendar/list date ranges.
const maxCampaignRangeDays = 366

// Campaign serves /api/app/campaigns/* for the panel's schedule menu. Mounted
// on the device-scoped group.
type Campaign struct {
	Service domainCampaign.ICampaignUsecase
	now     func() time.Time
}

func InitRestCampaign(app fiber.Router, service domainCampaign.ICampaignUsecase) *Campaign {
	h := &Campaign{Service: service, now: time.Now}
	// Static paths before /:key.
	app.Get("/api/app/campaigns", h.List)
	app.Get("/api/app/campaigns/dashboard", h.Dashboard)
	app.Get("/api/app/campaigns/calendar", h.Calendar)
	app.Get("/api/app/campaigns/assets/:schedule_id/:field", h.Asset)
	app.Get("/api/app/campaigns/:key", h.Get)
	app.Post("/api/app/campaigns/:key/:action<regex(^(pause|resume|cancel)$)>", h.Action)
	return h
}

// dateRange reads ?from=&to= as inclusive YYYY-MM-DD dates in the service
// zone. Missing values default to the last 7 days; when required is false
// and both are missing, it returns zero times (no range).
func (h *Campaign) dateRange(c fiber.Ctx, required bool) (from, to time.Time) {
	loc := h.Service.Location()
	if !required && c.Query("from") == "" && c.Query("to") == "" {
		return time.Time{}, time.Time{}
	}
	today := h.now().In(loc)
	toDay := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, loc)
	fromDay := toDay.AddDate(0, 0, -6)
	parse := func(name string, def time.Time) time.Time {
		v := c.Query(name)
		if v == "" {
			return def
		}
		d, err := time.ParseInLocation("2006-01-02", v, loc)
		if err != nil {
			panic(pkgError.ValidationError("'" + name + "' must be YYYY-MM-DD"))
		}
		return d
	}
	fromDay, toDay = parse("from", fromDay), parse("to", toDay)
	if toDay.Before(fromDay) {
		panic(pkgError.ValidationError("'to' must not be before 'from'"))
	}
	end := toDay.AddDate(0, 0, 1)
	if end.Sub(fromDay) > maxCampaignRangeDays*24*time.Hour+time.Hour { // +1h tolerates DST
		panic(pkgError.ValidationError("range is limited to 366 days"))
	}
	return fromDay, end
}

func notFound(c fiber.Ctx, err error) (bool, error) {
	if errors.Is(err, domainCampaign.ErrNotFound) {
		return true, c.Status(fiber.StatusNotFound).JSON(utils.ResponseData{Status: 404, Code: "NOT_FOUND", Message: "campaign not found"})
	}
	return false, nil
}


func ok(c fiber.Ctx, msg string, results any) error {
	return c.JSON(utils.ResponseData{Status: 200, Code: "SUCCESS", Message: msg, Results: results})
}

func (h *Campaign) List(c fiber.Ctx) error {
	from, to := h.dateRange(c, false)
	list, err := h.Service.List(whatsapp.ContextWithDevice(c.Context(), getDeviceFromCtx(c)), domainCampaign.ListFilter{
		From: from, To: to, Status: c.Query("status"), Query: c.Query("q"),
	})
	utils.PanicIfNeeded(err)
	return ok(c, "campaigns", list)
}

func (h *Campaign) Get(c fiber.Ctx) error {
	d, err := h.Service.Get(whatsapp.ContextWithDevice(c.Context(), getDeviceFromCtx(c)), c.Params("key"))
	if handled, rerr := notFound(c, err); handled {
		return rerr
	}
	utils.PanicIfNeeded(err)
	return ok(c, "campaign", d)
}

func (h *Campaign) Dashboard(c fiber.Ctx) error {
	from, to := h.dateRange(c, true)
	d, err := h.Service.Dashboard(whatsapp.ContextWithDevice(c.Context(), getDeviceFromCtx(c)), from, to)
	utils.PanicIfNeeded(err)
	return ok(c, "campaign dashboard", d)
}

func (h *Campaign) Calendar(c fiber.Ctx) error {
	from, to := h.dateRange(c, true)
	events, err := h.Service.Calendar(whatsapp.ContextWithDevice(c.Context(), getDeviceFromCtx(c)), from, to)
	utils.PanicIfNeeded(err)
	return ok(c, "campaign calendar", events)
}

func (h *Campaign) Action(c fiber.Ctx) error {
	res, err := h.Service.Action(whatsapp.ContextWithDevice(c.Context(), getDeviceFromCtx(c)), c.Params("key"), c.Params("action"))
	if handled, rerr := notFound(c, err); handled {
		return rerr
	}
	utils.PanicIfNeeded(err)
	return ok(c, "campaign "+c.Params("action"), res)
}

// Asset streams a stored upload of a schedule on the caller's device. The
// response is sandboxed so an uploaded SVG/HTML cannot run as the panel.
func (h *Campaign) Asset(c fiber.Ctx) error {
	a, err := h.Service.Asset(whatsapp.ContextWithDevice(c.Context(), getDeviceFromCtx(c)), c.Params("schedule_id"), c.Params("field"))
	if handled, rerr := notFound(c, err); handled {
		return rerr
	}
	utils.PanicIfNeeded(err)
	c.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	c.Set(fiber.HeaderXContentTypeOptions, "nosniff")
	c.Set(fiber.HeaderCacheControl, "private, max-age=300")
	c.Set(fiber.HeaderContentDisposition, mime.FormatMediaType("inline", map[string]string{"filename": filepath.Base(a.Filename)}))
	if err := c.SendFile(a.Path); err != nil {
		return err
	}
	if a.ContentType != "" {
		c.Set(fiber.HeaderContentType, a.ContentType)
	}
	return nil
}
