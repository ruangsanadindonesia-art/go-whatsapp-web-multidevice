package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	domainAnalytics "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/analytics"
	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
)

type stubReader struct {
	rows   []domainAnalytics.MessageRow
	device string
	calls  int
}

func (s *stubReader) MessagesInRange(_ context.Context, deviceID string, _, _ time.Time) ([]domainAnalytics.MessageRow, error) {
	s.calls++
	s.device = deviceID
	return s.rows, nil
}

func (s *stubReader) ChatNames(_ context.Context, _ string, jids []string) (map[string]string, error) {
	out := map[string]string{}
	for _, j := range jids {
		if j == "a" {
			out[j] = "Fulan"
		}
	}
	return out, nil
}

func newTestAnalytics(t *testing.T, r *stubReader, now time.Time) *serviceAnalytics {
	svc := NewAnalyticsService(r, testSchedule(t)).(*serviceAnalytics)
	svc.now = func() time.Time { return now }
	svc.deviceID = func(context.Context) string { return "dev1@s.whatsapp.net" }
	return svc
}

func isValidation(err error) bool {
	var v pkgError.ValidationError
	return errors.As(err, &v)
}

func TestAnalyticsRejectsInvertedRange(t *testing.T) {
	r := &stubReader{}
	svc := newTestAnalytics(t, r, wibAt(23, 12, 0))
	_, err := svc.Summary(context.Background(), domainAnalytics.RangeRequest{From: wibAt(23, 0, 0), To: wibAt(22, 0, 0)})
	if !isValidation(err) || r.calls != 0 {
		t.Fatalf("err %v calls %d", err, r.calls)
	}
}

func TestAnalyticsRejectsOver90Days(t *testing.T) {
	r := &stubReader{}
	svc := newTestAnalytics(t, r, wibAt(23, 12, 0))
	from := wibAt(1, 0, 0)
	_, err := svc.Volume(context.Background(), domainAnalytics.RangeRequest{From: from, To: from.Add(91 * 24 * time.Hour)})
	if !isValidation(err) || r.calls != 0 {
		t.Fatalf("err %v calls %d", err, r.calls)
	}
}

func TestAnalyticsRequiresDevice(t *testing.T) {
	svc := newTestAnalytics(t, &stubReader{}, wibAt(23, 12, 0))
	svc.deviceID = func(context.Context) string { return "" }
	_, err := svc.Summary(context.Background(), domainAnalytics.RangeRequest{From: wibAt(22, 0, 0), To: wibAt(23, 0, 0)})
	if !isValidation(err) {
		t.Fatalf("err %v", err)
	}
}

func TestAnalyticsSummaryUsesDeviceAndNow(t *testing.T) {
	r := &stubReader{rows: []domainAnalytics.MessageRow{in("a", wibAt(23, 10, 0))}}
	svc := newTestAnalytics(t, r, wibAt(23, 10, 30))
	sum, err := svc.Summary(context.Background(), domainAnalytics.RangeRequest{From: wibAt(23, 0, 0), To: wibAt(24, 0, 0)})
	if err != nil {
		t.Fatal(err)
	}
	if r.device != "dev1@s.whatsapp.net" || len(sum.Unanswered) != 1 || sum.Unanswered[0].WaitingService != 30*time.Minute || sum.Unanswered[0].Name != "Fulan" {
		t.Fatalf("device %q sum %+v", r.device, sum)
	}
}

func TestAnalyticsVolumeBucketsInServiceZone(t *testing.T) {
	r := &stubReader{rows: []domainAnalytics.MessageRow{
		ctxRow(in("a", wibAt(22, 9, 0))),
		in("a", time.Date(2026, 11, 23, 2, 15, 0, 0, time.UTC)), // 09:15 WIB
		out("a", wibAt(23, 9, 40)),
		in("b", wibAt(23, 20, 1)),
	}}
	svc := newTestAnalytics(t, r, wibAt(24, 0, 0))
	got, err := svc.Volume(context.Background(), domainAnalytics.RangeRequest{From: wibAt(23, 0, 0), To: wibAt(24, 0, 0)})
	if err != nil {
		t.Fatal(err)
	}
	want := []domainAnalytics.HourBucket{
		{Day: "2026-11-23", Hour: 9, Inbound: 1, Outbound: 1},
		{Day: "2026-11-23", Hour: 20, Inbound: 1},
	}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %+v", got)
	}
}
