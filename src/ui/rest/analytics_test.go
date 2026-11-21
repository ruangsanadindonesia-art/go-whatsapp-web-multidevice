package rest

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	domainAnalytics "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/analytics"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/rest/middleware"
	"github.com/gofiber/fiber/v3"
)

type analyticsStub struct {
	req domainAnalytics.RangeRequest
}

var stubWIB, _ = time.LoadLocation("Asia/Jakarta")

func (s *analyticsStub) Summary(_ context.Context, req domainAnalytics.RangeRequest) (domainAnalytics.Summary, error) {
	s.req = req
	return domainAnalytics.Summary{Inbound: 3, MedianResponse: 90 * time.Second, Unanswered: []domainAnalytics.UnansweredChat{}}, nil
}
func (s *analyticsStub) Volume(_ context.Context, req domainAnalytics.RangeRequest) ([]domainAnalytics.HourBucket, error) {
	s.req = req
	return []domainAnalytics.HourBucket{{Day: "2026-11-23", Hour: 9, Inbound: 1}}, nil
}
func (s *analyticsStub) ServiceHours() string     { return "08:00-15:00,19:00-21:00" }
func (s *analyticsStub) Location() *time.Location { return stubWIB }

func analyticsApp(stub *analyticsStub, now time.Time) *fiber.App {
	app := fiber.New()
	app.Use(middleware.Recovery())
	h := InitRestAnalytics(app, stub)
	h.now = func() time.Time { return now }
	return app
}

func getJSON(t *testing.T, app *fiber.App, target string) (int, map[string]any) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest("GET", target, nil))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return resp.StatusCode, out
}

func TestAnalyticsSummaryParsesDates(t *testing.T) {
	stub := &analyticsStub{}
	app := analyticsApp(stub, time.Now())
	status, body := getJSON(t, app, "/api/app/analytics/summary?from=2026-11-20&to=2026-11-23")
	if status != 200 {
		t.Fatalf("status %d body %v", status, body)
	}
	wantFrom := time.Date(2026, 11, 20, 0, 0, 0, 0, stubWIB)
	wantTo := time.Date(2026, 11, 24, 0, 0, 0, 0, stubWIB) // 'to' is inclusive
	if !stub.req.From.Equal(wantFrom) || !stub.req.To.Equal(wantTo) {
		t.Fatalf("got %v .. %v", stub.req.From, stub.req.To)
	}
	res := body["results"].(map[string]any)
	if res["service_hours"] != "08:00-15:00,19:00-21:00" || res["from"] != "2026-11-20" || res["to"] != "2026-11-23" {
		t.Fatalf("results %v", res)
	}
	sum := res["summary"].(map[string]any)
	if sum["median_response_s"] != float64(90) || sum["inbound"] != float64(3) {
		t.Fatalf("summary %v", sum)
	}
}

func TestAnalyticsSummaryBadDate(t *testing.T) {
	app := analyticsApp(&analyticsStub{}, time.Now())
	for _, q := range []string{"from=20-11-2026&to=2026-11-23", "from=2026-11-20&to=nope"} {
		if status, body := getJSON(t, app, "/api/app/analytics/summary?"+q); status != 400 {
			t.Fatalf("%s: status %d body %v", q, status, body)
		}
	}
}

func TestAnalyticsDefaultsToLast7Days(t *testing.T) {
	stub := &analyticsStub{}
	now := time.Date(2026, 11, 23, 18, 0, 0, 0, time.UTC) // 24 Nov 01:00 WIB
	app := analyticsApp(stub, now)
	status, body := getJSON(t, app, "/api/app/analytics/volume")
	if status != 200 {
		t.Fatalf("status %d %v", status, body)
	}
	if !stub.req.From.Equal(time.Date(2026, 11, 18, 0, 0, 0, 0, stubWIB)) || !stub.req.To.Equal(time.Date(2026, 11, 25, 0, 0, 0, 0, stubWIB)) {
		t.Fatalf("got %v .. %v", stub.req.From, stub.req.To)
	}
	res := body["results"].(map[string]any)
	if len(res["buckets"].([]any)) != 1 {
		t.Fatalf("results %v", res)
	}
}
