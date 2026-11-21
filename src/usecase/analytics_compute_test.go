package usecase

import (
	"testing"
	"time"

	domainAnalytics "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/analytics"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/servicehours"
)

var testWIB, _ = time.LoadLocation("Asia/Jakarta")

func wibAt(day, h, m int) time.Time { return time.Date(2026, 11, day, h, m, 0, 0, testWIB) }

func testSchedule(t *testing.T) servicehours.Schedule {
	t.Helper()
	s, err := servicehours.Parse("08:00-15:00,19:00-21:00", "Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func in(chat string, ts time.Time) domainAnalytics.MessageRow {
	return domainAnalytics.MessageRow{ChatJID: chat, Timestamp: ts}
}
func out(chat string, ts time.Time) domainAnalytics.MessageRow {
	return domainAnalytics.MessageRow{ChatJID: chat, Timestamp: ts, IsFromMe: true}
}
func ctxRow(r domainAnalytics.MessageRow) domainAnalytics.MessageRow { r.Context = true; return r }

func TestTurnsGroupConsecutiveInbound(t *testing.T) {
	turns := computeTurns([]domainAnalytics.MessageRow{
		in("a", wibAt(23, 9, 0)), in("a", wibAt(23, 9, 2)), out("a", wibAt(23, 9, 10)),
	}, testSchedule(t))
	if len(turns) != 1 || turns[0].Response != 10*time.Minute || turns[0].RepliedAt == nil {
		t.Fatalf("got %+v", turns)
	}
}

func TestTurnsIgnoreLeadingOutbound(t *testing.T) {
	turns := computeTurns([]domainAnalytics.MessageRow{
		out("a", wibAt(23, 9, 0)), in("a", wibAt(23, 9, 5)), out("a", wibAt(23, 9, 6)),
	}, testSchedule(t))
	if len(turns) != 1 || turns[0].Response != time.Minute {
		t.Fatalf("got %+v", turns)
	}
}

func TestTurnsOvernightUsesServiceTime(t *testing.T) {
	turns := computeTurns([]domainAnalytics.MessageRow{
		in("a", wibAt(23, 22, 0)), out("a", wibAt(24, 8, 5)),
	}, testSchedule(t))
	if len(turns) != 1 || turns[0].Response != 5*time.Minute {
		t.Fatalf("got %+v", turns)
	}
}

func TestTurnsSeparateChats(t *testing.T) {
	turns := computeTurns([]domainAnalytics.MessageRow{
		in("a", wibAt(23, 9, 0)),
		in("b", wibAt(23, 9, 1)), out("b", wibAt(23, 9, 3)),
	}, testSchedule(t))
	if len(turns) != 2 || turns[0].RepliedAt != nil || turns[1].Response != 2*time.Minute {
		t.Fatalf("got %+v", turns)
	}
}

func TestTurnsContextBeforeRangeIsNotCounted(t *testing.T) {
	// Turn started before the range (context row) and continued inside it:
	// it belongs to the earlier range and is not counted here.
	turns := computeTurns([]domainAnalytics.MessageRow{
		ctxRow(in("a", wibAt(22, 14, 0))), in("a", wibAt(23, 9, 0)), out("a", wibAt(23, 9, 5)),
	}, testSchedule(t))
	if len(turns) != 0 {
		t.Fatalf("got %+v", turns)
	}
}

func TestTurnsReplyAfterRangeClosesTurn(t *testing.T) {
	// Asked inside the range, answered after it (context row): answered.
	turns := computeTurns([]domainAnalytics.MessageRow{
		in("a", wibAt(23, 20, 50)), ctxRow(out("a", wibAt(24, 8, 10))),
	}, testSchedule(t))
	if len(turns) != 1 || turns[0].RepliedAt == nil || turns[0].Response != 20*time.Minute {
		t.Fatalf("got %+v", turns)
	}
}

func TestUnansweredLastInbound(t *testing.T) {
	rows := []domainAnalytics.MessageRow{in("a", wibAt(23, 10, 0))}
	s := testSchedule(t)
	sum := summarize(rows, computeTurns(rows, s), wibAt(23, 10, 30), s)
	if len(sum.Unanswered) != 1 || sum.Unanswered[0].WaitingService != 30*time.Minute || sum.Unanswered[0].ChatJID != "a" {
		t.Fatalf("got %+v", sum.Unanswered)
	}
	if sum.Turns != 1 || sum.Answered != 0 || sum.Inbound != 1 {
		t.Fatalf("got %+v", sum)
	}
}

func TestUnansweredSortedLongestFirst(t *testing.T) {
	rows := []domainAnalytics.MessageRow{in("a", wibAt(23, 11, 0)), in("b", wibAt(23, 9, 0))}
	s := testSchedule(t)
	sum := summarize(rows, computeTurns(rows, s), wibAt(23, 12, 0), s)
	if len(sum.Unanswered) != 2 || sum.Unanswered[0].ChatJID != "b" {
		t.Fatalf("got %+v", sum.Unanswered)
	}
}

func TestSummaryPercentiles(t *testing.T) {
	var rows []domainAnalytics.MessageRow
	for i := 1; i <= 10; i++ {
		chat := string(rune('a' + i))
		rows = append(rows, in(chat, wibAt(23, 9, 0)), out(chat, wibAt(23, 9, i)))
	}
	s := testSchedule(t)
	sum := summarize(rows, computeTurns(rows, s), wibAt(23, 12, 0), s)
	if sum.MedianResponse != 5*time.Minute || sum.P90Response != 9*time.Minute {
		t.Fatalf("median %v p90 %v", sum.MedianResponse, sum.P90Response)
	}
	if sum.Inbound != 10 || sum.Outbound != 10 || sum.Turns != 10 || sum.Answered != 10 {
		t.Fatalf("got %+v", sum)
	}
}

func TestSummaryContextRowsNotCounted(t *testing.T) {
	rows := []domainAnalytics.MessageRow{ctxRow(out("a", wibAt(22, 9, 0))), in("a", wibAt(23, 9, 0)), ctxRow(out("a", wibAt(25, 9, 0)))}
	s := testSchedule(t)
	sum := summarize(rows, computeTurns(rows, s), wibAt(26, 9, 0), s)
	if sum.Inbound != 1 || sum.Outbound != 0 {
		t.Fatalf("got %+v", sum)
	}
}

func TestSummaryEmpty(t *testing.T) {
	s := testSchedule(t)
	sum := summarize(nil, nil, wibAt(23, 9, 0), s)
	if sum.Turns != 0 || sum.MedianResponse != 0 || sum.Unanswered == nil {
		t.Fatalf("got %+v (Unanswered must be an empty slice, not nil)", sum)
	}
}
