// Package servicehours models the CS service windows (e.g. 08:00-15:00 and
// 19:00-21:00 WIB, every day) and measures elapsed time inside them.
package servicehours

import (
	"fmt"
	"sort"
	"strings"
	"time"
	_ "time/tzdata" // zone database fallback for minimal images
)

// Window is one daily service window, as offsets from local midnight.
type Window struct{ Start, End time.Duration }

// Schedule is a set of non-overlapping daily windows in one time zone.
type Schedule struct {
	Windows []Window
	Loc     *time.Location
}

// Parse reads "HH:MM-HH:MM[,HH:MM-HH:MM...]" in time zone tz.
func Parse(raw, tz string) (Schedule, error) {
	loc, err := time.LoadLocation(strings.TrimSpace(tz))
	if err != nil {
		return Schedule{}, fmt.Errorf("service hours: unknown time zone %q", tz)
	}
	var ws []Window
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		a, b, ok := strings.Cut(part, "-")
		if !ok {
			return Schedule{}, fmt.Errorf("service hours: %q is not HH:MM-HH:MM", part)
		}
		start, err1 := clock(a)
		end, err2 := clock(b)
		if err1 != nil || err2 != nil {
			return Schedule{}, fmt.Errorf("service hours: %q is not HH:MM-HH:MM", part)
		}
		if start >= end {
			return Schedule{}, fmt.Errorf("service hours: %q ends before it starts", part)
		}
		ws = append(ws, Window{start, end})
	}
	sort.Slice(ws, func(i, j int) bool { return ws[i].Start < ws[j].Start })
	for i := 1; i < len(ws); i++ {
		if ws[i].Start < ws[i-1].End {
			return Schedule{}, fmt.Errorf("service hours: windows overlap")
		}
	}
	return Schedule{Windows: ws, Loc: loc}, nil
}

func clock(s string) (time.Duration, error) {
	t, err := time.Parse("15:04", strings.TrimSpace(s))
	if err != nil {
		if strings.TrimSpace(s) == "24:00" {
			return 24 * time.Hour, nil
		}
		return 0, err
	}
	return time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute, nil
}

// Elapsed returns the time between from and to that falls inside the
// service windows; 0 when to <= from.
func (s Schedule) Elapsed(from, to time.Time) time.Duration {
	if !to.After(from) {
		return 0
	}
	from, to = from.In(s.Loc), to.In(s.Loc)
	var total time.Duration
	day := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, s.Loc)
	for !day.After(to) {
		for _, w := range s.Windows {
			ws := addClock(day, w.Start)
			we := addClock(day, w.End)
			lo, hi := maxTime(from, ws), minTime(to, we)
			if hi.After(lo) {
				total += hi.Sub(lo)
			}
		}
		day = time.Date(day.Year(), day.Month(), day.Day()+1, 0, 0, 0, 0, s.Loc)
	}
	return total
}

// InService reports whether t falls inside a window.
func (s Schedule) InService(t time.Time) bool {
	t = t.In(s.Loc)
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, s.Loc)
	for _, w := range s.Windows {
		if !t.Before(addClock(day, w.Start)) && t.Before(addClock(day, w.End)) {
			return true
		}
	}
	return false
}

// String formats the windows back as "HH:MM-HH:MM,...".
func (s Schedule) String() string {
	parts := make([]string, len(s.Windows))
	for i, w := range s.Windows {
		parts[i] = fmtClock(w.Start) + "-" + fmtClock(w.End)
	}
	return strings.Join(parts, ",")
}

// addClock places a wall-clock offset on day (DST-safe via time.Date).
func addClock(day time.Time, d time.Duration) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location()).Add(d)
}

func fmtClock(d time.Duration) string {
	return fmt.Sprintf("%02d:%02d", int(d/time.Hour), int(d%time.Hour/time.Minute))
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
