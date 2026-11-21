package usecase

import (
	"math"
	"sort"
	"time"

	domainAnalytics "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/analytics"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/servicehours"
)

// computeTurns splits rows (sorted by ChatJID, Timestamp) into customer
// turns. A turn opens on an inbound message after an outbound one (or at the
// start of a chat) and closes on the next outbound message. Only turns that
// open on an in-range (non-context) message are returned.
func computeTurns(rows []domainAnalytics.MessageRow, s servicehours.Schedule) []domainAnalytics.Turn {
	var turns []domainAnalytics.Turn
	var (
		chat    string
		open    bool
		inRange bool
		asked   time.Time
	)
	flush := func() {
		if open && inRange {
			turns = append(turns, domainAnalytics.Turn{ChatJID: chat, AskedAt: asked})
		}
		open = false
	}
	for _, r := range rows {
		if r.ChatJID != chat {
			flush()
			chat = r.ChatJID
		}
		if !r.IsFromMe {
			if !open {
				open, inRange, asked = true, !r.Context, r.Timestamp
			}
			continue
		}
		if open && inRange {
			replied := r.Timestamp
			turns = append(turns, domainAnalytics.Turn{
				ChatJID: chat, AskedAt: asked, RepliedAt: &replied, Response: s.Elapsed(asked, replied),
			})
		}
		open = false
	}
	flush()
	return turns
}

// summarize aggregates in-range rows and turns as of now.
func summarize(rows []domainAnalytics.MessageRow, turns []domainAnalytics.Turn, now time.Time, s servicehours.Schedule) domainAnalytics.Summary {
	sum := domainAnalytics.Summary{Unanswered: []domainAnalytics.UnansweredChat{}}
	for _, r := range rows {
		switch {
		case r.Context:
		case r.IsFromMe:
			sum.Outbound++
		default:
			sum.Inbound++
		}
	}
	var responses []time.Duration
	for _, t := range turns {
		sum.Turns++
		if t.RepliedAt != nil {
			sum.Answered++
			responses = append(responses, t.Response)
			continue
		}
		sum.Unanswered = append(sum.Unanswered, domainAnalytics.UnansweredChat{
			ChatJID: t.ChatJID, WaitingSince: t.AskedAt, WaitingService: s.Elapsed(t.AskedAt, now),
		})
	}
	sort.Slice(responses, func(i, j int) bool { return responses[i] < responses[j] })
	sum.MedianResponse = nearestRank(responses, 0.5)
	sum.P90Response = nearestRank(responses, 0.9)
	sort.SliceStable(sum.Unanswered, func(i, j int) bool {
		return sum.Unanswered[i].WaitingSince.Before(sum.Unanswered[j].WaitingSince)
	})
	return sum
}

// nearestRank returns the p-th percentile of sorted values (0 when empty).
func nearestRank(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	return sorted[idx]
}
