package rest

import (
	"time"

	domainAnalytics "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/analytics"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/gofiber/fiber/v3"
)

// Analytics serves /api/app/analytics/* for the Ruang Sanad panel. Mounted on
// the device-scoped group, so the device comes from X-Device-Id.
type Analytics struct {
	Service domainAnalytics.IAnalyticsUsecase
	now     func() time.Time
}

func InitRestAnalytics(app fiber.Router, service domainAnalytics.IAnalyticsUsecase) *Analytics {
	h := &Analytics{Service: service, now: time.Now}
	app.Get("/api/app/analytics/summary", h.Summary)
	app.Get("/api/app/analytics/volume", h.Volume)
	return h
}

type analyticsRange struct {
	req      domainAnalytics.RangeRequest
	from, to string // inclusive dates, YYYY-MM-DD
}

// parseRange reads ?from=&to= as inclusive dates in the service time zone;
// the default is the last 7 days including today.
func (h *Analytics) parseRange(c fiber.Ctx) analyticsRange {
	loc := h.Service.Location()
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
			panic(pkgError.ValidationError("analytics: '" + name + "' must be YYYY-MM-DD"))
		}
		return d
	}
	fromDay, toDay = parse("from", fromDay), parse("to", toDay)
	return analyticsRange{
		req:  domainAnalytics.RangeRequest{From: fromDay, To: toDay.AddDate(0, 0, 1)},
		from: fromDay.Format("2006-01-02"),
		to:   toDay.Format("2006-01-02"),
	}
}

func (h *Analytics) Summary(c fiber.Ctx) error {
	r := h.parseRange(c)
	sum, err := h.Service.Summary(whatsapp.ContextWithDevice(c.Context(), getDeviceFromCtx(c)), r.req)
	utils.PanicIfNeeded(err)
	return c.JSON(utils.ResponseData{Status: 200, Code: "SUCCESS", Message: "analytics summary", Results: fiber.Map{
		"summary": sum, "service_hours": h.Service.ServiceHours(), "from": r.from, "to": r.to,
	}})
}

func (h *Analytics) Volume(c fiber.Ctx) error {
	r := h.parseRange(c)
	buckets, err := h.Service.Volume(whatsapp.ContextWithDevice(c.Context(), getDeviceFromCtx(c)), r.req)
	utils.PanicIfNeeded(err)
	return c.JSON(utils.ResponseData{Status: 200, Code: "SUCCESS", Message: "analytics volume", Results: fiber.Map{
		"buckets": buckets, "service_hours": h.Service.ServiceHours(), "from": r.from, "to": r.to,
	}})
}
