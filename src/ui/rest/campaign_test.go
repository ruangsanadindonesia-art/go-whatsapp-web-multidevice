package rest

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	domainCampaign "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/campaign"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/rest/middleware"
	"github.com/gofiber/fiber/v3"
)

type campaignStub struct {
	from, to  time.Time
	action    string
	assetPath string
}

func (s *campaignStub) List(_ context.Context, f domainCampaign.ListFilter) ([]domainCampaign.Campaign, error) {
	s.from, s.to = f.From, f.To
	return []domainCampaign.Campaign{{Key: "k1"}}, nil
}
func (s *campaignStub) Get(_ context.Context, key string) (*domainCampaign.Detail, error) {
	if key != "k1" {
		return nil, domainCampaign.ErrNotFound
	}
	return &domainCampaign.Detail{Campaign: domainCampaign.Campaign{Key: "k1"}}, nil
}
func (s *campaignStub) Dashboard(_ context.Context, from, to time.Time) (domainCampaign.Dashboard, error) {
	s.from, s.to = from, to
	return domainCampaign.Dashboard{From: "x"}, nil
}
func (s *campaignStub) Calendar(_ context.Context, from, to time.Time) ([]domainCampaign.CalendarEvent, error) {
	s.from, s.to = from, to
	return []domainCampaign.CalendarEvent{}, nil
}
func (s *campaignStub) Action(_ context.Context, key, action string) (domainCampaign.ActionResult, error) {
	s.action = action
	if key != "k1" {
		return domainCampaign.ActionResult{}, domainCampaign.ErrNotFound
	}
	return domainCampaign.ActionResult{Changed: 2}, nil
}
func (s *campaignStub) Asset(_ context.Context, id, field string) (*domainCampaign.Asset, error) {
	if id != "s1" || field != "image" {
		return nil, domainCampaign.ErrNotFound
	}
	return &domainCampaign.Asset{Path: s.assetPath, Filename: "a.svg", ContentType: "image/svg+xml"}, nil
}
func (s *campaignStub) Location() *time.Location { return stubWIB }

func campaignApp(stub *campaignStub, now time.Time) *fiber.App {
	app := fiber.New()
	app.Use(middleware.Recovery())
	h := InitRestCampaign(app, stub)
	h.now = func() time.Time { return now }
	return app
}

func TestCampaignDashboardInclusiveRange(t *testing.T) {
	stub := &campaignStub{}
	app := campaignApp(stub, time.Now())
	status, body := getJSON(t, app, "/api/app/campaigns/dashboard?from=2026-11-20&to=2026-11-22")
	if status != 200 {
		t.Fatalf("status %d %v", status, body)
	}
	if !stub.from.Equal(time.Date(2026, 11, 20, 0, 0, 0, 0, stubWIB)) || !stub.to.Equal(time.Date(2026, 11, 23, 0, 0, 0, 0, stubWIB)) {
		t.Fatalf("range %v .. %v", stub.from, stub.to)
	}
}

func TestCampaignSingleDateAndDefaults(t *testing.T) {
	stub := &campaignStub{}
	now := time.Date(2026, 11, 22, 3, 0, 0, 0, time.UTC) // 10.00 WIB
	app := campaignApp(stub, now)
	getJSON(t, app, "/api/app/campaigns/calendar?from=2026-11-21&to=2026-11-21")
	if stub.to.Sub(stub.from) != 24*time.Hour {
		t.Fatalf("single date: %v .. %v", stub.from, stub.to)
	}
	getJSON(t, app, "/api/app/campaigns/dashboard")
	if !stub.from.Equal(time.Date(2026, 11, 16, 0, 0, 0, 0, stubWIB)) || !stub.to.Equal(time.Date(2026, 11, 23, 0, 0, 0, 0, stubWIB)) {
		t.Fatalf("default 7 days: %v .. %v", stub.from, stub.to)
	}
}

func TestCampaignRangeValidation(t *testing.T) {
	app := campaignApp(&campaignStub{}, time.Now())
	for _, q := range []string{"from=2026-11-22&to=2026-11-20", "from=2025-01-01&to=2026-11-20", "from=bad&to=2026-11-20"} {
		if status, body := getJSON(t, app, "/api/app/campaigns/dashboard?"+q); status != 400 {
			t.Fatalf("%s: status %d %v", q, status, body)
		}
	}
}

func TestCampaignGetAndActions(t *testing.T) {
	stub := &campaignStub{}
	app := campaignApp(stub, time.Now())
	if status, _ := getJSON(t, app, "/api/app/campaigns/k1"); status != 200 {
		t.Fatalf("get status %d", status)
	}
	if status, _ := getJSON(t, app, "/api/app/campaigns/zzz"); status != 404 {
		t.Fatalf("unknown status %d", status)
	}
	resp, _ := app.Test(httptest.NewRequest("POST", "/api/app/campaigns/k1/pause", nil))
	if resp.StatusCode != 200 || stub.action != "pause" {
		t.Fatalf("pause %d %q", resp.StatusCode, stub.action)
	}
	resp, _ = app.Test(httptest.NewRequest("POST", "/api/app/campaigns/k1/explode", nil))
	if resp.StatusCode != 404 {
		t.Fatalf("unknown action %d", resp.StatusCode)
	}
	if status, _ := getJSON(t, app, "/api/app/campaigns?from=2026-11-01&to=2026-11-02"); status != 200 || stub.from.IsZero() {
		t.Fatalf("list status %d from %v", status, stub.from)
	}
}

func TestCampaignAssetIsSandboxed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.svg")
	if err := os.WriteFile(path, []byte("<svg/>"), 0o600); err != nil {
		t.Fatal(err)
	}
	app := campaignApp(&campaignStub{assetPath: path}, time.Now())
	resp, err := app.Test(httptest.NewRequest("GET", "/api/app/campaigns/assets/s1/image", nil))
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("asset %v %v", resp.StatusCode, err)
	}
	if resp.Header.Get("Content-Security-Policy") != "default-src 'none'; sandbox" || resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("headers %v", resp.Header)
	}
	resp, _ = app.Test(httptest.NewRequest("GET", "/api/app/campaigns/assets/s2/image", nil))
	if resp.StatusCode != 404 {
		t.Fatalf("missing asset %d", resp.StatusCode)
	}
}
