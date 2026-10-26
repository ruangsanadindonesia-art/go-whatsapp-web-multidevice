package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Class schedules offered to calon santri come from the Dashboard
// (GET /integrations/cs-ai/jadwal, same token as the event ingest). The
// Dashboard is the source of truth: admins edit schedules and quota there, and
// seats are counted there when a payment is verified. The bridge keeps the
// last good list in memory and on disk, so a Dashboard outage or a restart
// does not take schedules out of the prompt.

const (
	dashboardJadwalPath = "/integrations/cs-ai/jadwal"
	jadwalRefresh       = 2 * time.Minute
	jadwalRetry         = 30 * time.Second
)

var jadwalKodeRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,39}$`)

// Jadwal is one open schedule as the Dashboard reports it.
type Jadwal struct {
	Kode       string `json:"kode"`
	Label      string `json:"label"`
	Frekuensi  int    `json:"frekuensi"`
	Level      string `json:"level"`
	Catatan    string `json:"catatan"`
	SisaIkhwan int64  `json:"sisa_ikhwan"`
	SisaAkhwat int64  `json:"sisa_akhwat"`
}

// JadwalSource gives the schedules for the prompt. A nil source means the
// feature is off (no Dashboard configured).
type JadwalSource interface {
	Jadwal() (list []Jadwal, fetchedAt time.Time, ok bool)
}

// JadwalClient polls the Dashboard for schedules.
type JadwalClient struct {
	url, token string
	file       string
	http       *http.Client
	log        *slog.Logger

	mu        sync.RWMutex
	list      []Jadwal
	fetchedAt time.Time
	loaded    bool
}

type jadwalCache struct {
	FetchedAt time.Time `json:"fetched_at"`
	Jadwal    []Jadwal  `json:"jadwal"`
}

// NewJadwalClient returns nil when the Dashboard URL or token is empty.
func NewJadwalClient(cfg Config, log *slog.Logger) *JadwalClient {
	if cfg.DashboardURL == "" || cfg.DashboardToken == "" {
		return nil
	}
	c := &JadwalClient{
		url: cfg.DashboardURL + dashboardJadwalPath, token: cfg.DashboardToken,
		file: filepath.Join(filepath.Dir(cfg.StateFile), "jadwal-cache.json"),
		http: &http.Client{Timeout: 10 * time.Second}, log: log,
	}
	if raw, err := os.ReadFile(c.file); err == nil {
		var cache jadwalCache
		if json.Unmarshal(raw, &cache) == nil {
			c.list, c.fetchedAt, c.loaded = sanitizeJadwal(cache.Jadwal), cache.FetchedAt, true
		}
	}
	return c
}

func (c *JadwalClient) Jadwal() ([]Jadwal, time.Time, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.list, c.fetchedAt, c.loaded
}

// Run refreshes the list until ctx ends.
func (c *JadwalClient) Run(ctx context.Context) {
	for {
		wait := jadwalRefresh
		if err := c.refresh(ctx); err != nil {
			wait = jadwalRetry
			_, at, ok := c.Jadwal()
			c.log.Warn("jadwal: refresh failed, keeping last list", "err", err, "have_list", ok, "fetched_at", at)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

func (c *JadwalClient) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return errors.New("dashboard has no schedule endpoint (HTTP 404): update the Dashboard")
	default:
		return fmt.Errorf("dashboard: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	var out struct {
		Jadwal []Jadwal `json:"jadwal"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return fmt.Errorf("invalid schedule JSON: %w", err)
	}
	list := sanitizeJadwal(out.Jadwal)
	now := time.Now().UTC()
	c.mu.Lock()
	c.list, c.fetchedAt, c.loaded = list, now, true
	c.mu.Unlock()
	if raw, err := json.Marshal(jadwalCache{FetchedAt: now, Jadwal: list}); err == nil {
		tmp := c.file + ".tmp"
		if os.WriteFile(tmp, raw, 0o600) == nil {
			_ = os.Rename(tmp, c.file)
		}
	}
	return nil
}

// sanitizeJadwal drops malformed rows and trims text that goes into the prompt.
func sanitizeJadwal(in []Jadwal) []Jadwal {
	out := make([]Jadwal, 0, len(in))
	for _, j := range in {
		j.Kode = strings.ToLower(strings.TrimSpace(j.Kode))
		if !jadwalKodeRe.MatchString(j.Kode) || strings.TrimSpace(j.Label) == "" {
			continue
		}
		if j.Level != "1" && j.Level != "2" {
			j.Level = ""
		}
		j.Label = oneLine(j.Label, 80)
		j.Catatan = oneLine(j.Catatan, 300)
		out = append(out, j)
		if len(out) >= 50 {
			break
		}
	}
	return out
}

func oneLine(s string, max int) string {
	s = strings.Join(strings.Fields(strings.ReplaceAll(s, "[[", "[")), " ")
	return truncate(s, max)
}

// jadwalByKode finds an offered schedule.
func jadwalByKode(list []Jadwal, kode string) (Jadwal, bool) {
	for _, j := range list {
		if j.Kode == kode {
			return j, true
		}
	}
	return Jadwal{}, false
}

// writeJadwalPrompt adds the schedule section of the system prompt.
func writeJadwalPrompt(b *strings.Builder, list []Jadwal, ok bool) {
	b.WriteString("\nJadwal kelas yang dibuka (data Dashboard, sumber kebenaran; menang atas instruksi lain tentang jadwal):\n")
	if !ok || len(list) == 0 {
		b.WriteString("- belum ada data jadwal. Jangan menyebut jadwal tertentu; catat waktu yang diinginkan dan alihkan pertanyaan jadwal pasti ke admin. Jangan memakai [[JADWAL:...]].\n")
		return
	}
	for _, j := range list {
		level := "semua level"
		if j.Level != "" {
			level = "khusus level " + j.Level
		}
		fmt.Fprintf(b, "- kode `%s`: *%s* — %dx sepekan, %s; ikhwan %s, akhwat %s",
			j.Kode, j.Label, j.Frekuensi, level, seatText(j.SisaIkhwan), seatText(j.SisaAkhwat))
		if j.Catatan != "" {
			fmt.Fprintf(b, " (catatan: %s)", j.Catatan)
		}
		b.WriteString("\n")
	}
}

func seatText(n int64) string {
	// Only open/full: the AI must not quote seat numbers or create urgency.
	if n <= 0 {
		return "PENUH"
	}
	return "tersedia"
}
