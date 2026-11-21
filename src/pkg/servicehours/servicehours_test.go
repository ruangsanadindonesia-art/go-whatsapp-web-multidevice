package servicehours

import (
	"testing"
	"time"
)

var wib = mustLoc("Asia/Jakarta")

func mustLoc(name string) *time.Location {
	l, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return l
}

func at(day, h, m int) time.Time { return time.Date(2026, 11, day, h, m, 0, 0, wib) }

func sched(t *testing.T) Schedule {
	t.Helper()
	s, err := Parse("08:00-15:00,19:00-21:00", "Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestElapsedInsideWindow(t *testing.T) {
	if got := sched(t).Elapsed(at(23, 9, 0), at(23, 9, 10)); got != 10*time.Minute {
		t.Fatalf("got %v", got)
	}
}

func TestElapsedAcrossGap(t *testing.T) {
	if got := sched(t).Elapsed(at(23, 14, 50), at(23, 19, 10)); got != 20*time.Minute {
		t.Fatalf("got %v", got)
	}
}

func TestElapsedOvernight(t *testing.T) {
	if got := sched(t).Elapsed(at(23, 22, 0), at(24, 8, 5)); got != 5*time.Minute {
		t.Fatalf("got %v", got)
	}
}

func TestElapsedBeforeOpen(t *testing.T) {
	if got := sched(t).Elapsed(at(23, 7, 0), at(23, 8, 30)); got != 30*time.Minute {
		t.Fatalf("got %v", got)
	}
}

func TestElapsedMultiDay(t *testing.T) {
	// Mon 23 Nov 20:00 -> Wed 25 Nov 08:00 = 1h (Mon) + 9h (Tue)
	if got := sched(t).Elapsed(at(23, 20, 0), at(25, 8, 0)); got != 10*time.Hour {
		t.Fatalf("got %v", got)
	}
}

func TestElapsedReversedIsZero(t *testing.T) {
	if got := sched(t).Elapsed(at(23, 10, 0), at(23, 9, 0)); got != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestElapsedUsesScheduleZone(t *testing.T) {
	// 01:00 UTC = 08:00 WIB; 01:30 UTC = 08:30 WIB
	from := time.Date(2026, 11, 23, 1, 0, 0, 0, time.UTC)
	if got := sched(t).Elapsed(from, from.Add(30*time.Minute)); got != 30*time.Minute {
		t.Fatalf("got %v", got)
	}
}

func TestInService(t *testing.T) {
	s := sched(t)
	for tm, want := range map[time.Time]bool{at(23, 8, 0): true, at(23, 14, 59): true, at(23, 15, 0): false, at(23, 20, 0): true, at(23, 22, 0): false} {
		if s.InService(tm) != want {
			t.Fatalf("InService(%v) != %v", tm, want)
		}
	}
}

func TestParseRejectsOverlap(t *testing.T) {
	if _, err := Parse("08:00-15:00,14:00-16:00", "Asia/Jakarta"); err == nil {
		t.Fatal("want error")
	}
}

func TestParseRejectsBadFormat(t *testing.T) {
	for _, raw := range []string{"8-15", "", "15:00-08:00", "08:00-24:30", "08:00-15:00,"} {
		if _, err := Parse(raw, "Asia/Jakarta"); err == nil {
			t.Fatalf("Parse(%q): want error", raw)
		}
	}
	if _, err := Parse("08:00-15:00", "Mars/Base"); err == nil {
		t.Fatal("bad zone: want error")
	}
}

func TestString(t *testing.T) {
	if got := sched(t).String(); got != "08:00-15:00,19:00-21:00" {
		t.Fatalf("got %q", got)
	}
}
