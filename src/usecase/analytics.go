package usecase

import (
	"context"
	"sort"
	"time"

	domainAnalytics "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/analytics"
	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/servicehours"
)

// MaxAnalyticsRange caps one analytics query.
const MaxAnalyticsRange = 90 * 24 * time.Hour

type serviceAnalytics struct {
	reader   domainAnalytics.Reader
	schedule servicehours.Schedule
	now      func() time.Time
	deviceID func(context.Context) string
}

func NewAnalyticsService(reader domainAnalytics.Reader, schedule servicehours.Schedule) domainAnalytics.IAnalyticsUsecase {
	return &serviceAnalytics{reader: reader, schedule: schedule, now: time.Now, deviceID: deviceIDFromContext}
}

func (s *serviceAnalytics) ServiceHours() string { return s.schedule.String() }

func (s *serviceAnalytics) Location() *time.Location { return s.schedule.Loc }

func (s *serviceAnalytics) rows(ctx context.Context, req domainAnalytics.RangeRequest) ([]domainAnalytics.MessageRow, error) {
	if !req.To.After(req.From) {
		return nil, pkgError.ValidationError("analytics: 'to' must be after 'from'")
	}
	if req.To.Sub(req.From) > MaxAnalyticsRange {
		return nil, pkgError.ValidationError("analytics: range is limited to 90 days")
	}
	device := s.deviceID(ctx)
	if device == "" {
		return nil, pkgError.ValidationError("analytics: device is not logged in")
	}
	return s.reader.MessagesInRange(ctx, device, req.From, req.To)
}

func (s *serviceAnalytics) Summary(ctx context.Context, req domainAnalytics.RangeRequest) (domainAnalytics.Summary, error) {
	rows, err := s.rows(ctx, req)
	if err != nil {
		return domainAnalytics.Summary{}, err
	}
	return summarize(rows, computeTurns(rows, s.schedule), s.now(), s.schedule), nil
}

// Volume counts in-range messages per day and hour in the service time zone.
func (s *serviceAnalytics) Volume(ctx context.Context, req domainAnalytics.RangeRequest) ([]domainAnalytics.HourBucket, error) {
	rows, err := s.rows(ctx, req)
	if err != nil {
		return nil, err
	}
	type key struct {
		day  string
		hour int
	}
	buckets := map[key]*domainAnalytics.HourBucket{}
	for _, r := range rows {
		if r.Context {
			continue
		}
		local := r.Timestamp.In(s.schedule.Loc)
		k := key{local.Format("2006-01-02"), local.Hour()}
		b := buckets[k]
		if b == nil {
			b = &domainAnalytics.HourBucket{Day: k.day, Hour: k.hour}
			buckets[k] = b
		}
		if r.IsFromMe {
			b.Outbound++
		} else {
			b.Inbound++
		}
	}
	out := make([]domainAnalytics.HourBucket, 0, len(buckets))
	for _, b := range buckets {
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Day != out[j].Day {
			return out[i].Day < out[j].Day
		}
		return out[i].Hour < out[j].Hour
	})
	return out, nil
}
