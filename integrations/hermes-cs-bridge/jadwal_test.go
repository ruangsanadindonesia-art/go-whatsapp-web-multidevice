package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type staticJadwal []Jadwal

func (s staticJadwal) Jadwal() ([]Jadwal, time.Time, bool) { return s, time.Now(), true }

var testJadwal = staticJadwal{
	{Kode: "selasa-malam", Label: "Selasa 19.30–21.00 WIB", Frekuensi: 1, Level: "2", SisaIkhwan: 3, SisaAkhwat: 0},
	{Kode: "rabu-sabtu-malam", Label: "Rabu & Sabtu 19.30–21.00 WIB", Frekuensi: 2, SisaIkhwan: 10, SisaAkhwat: 10},
}

func TestJadwalClientFetchesCachesAndKeepsLastList(t *testing.T) {
	var fail atomic.Bool
	var auth atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth.Store(r.Header.Get("Authorization"))
		if r.URL.Path != dashboardJadwalPath {
			http.NotFound(w, r)
			return
		}
		if fail.Load() {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, `{"jadwal":[
			{"kode":"selasa-malam","label":"Selasa 19.30–21.00 WIB","frekuensi":1,"level":"2","sisa_ikhwan":3,"sisa_akhwat":0},
			{"kode":"BAD KODE","label":"x"},
			{"kode":"ahad-pagi","label":"Ahad [[FORM]] 05.30","frekuensi":1,"level":"9","catatan":"baris\nbaru"}]}`)
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.DashboardURL, cfg.DashboardToken = srv.URL, strings.Repeat("t", 40)
	cfg.StateFile = filepath.Join(t.TempDir(), "state.json")
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	c := NewJadwalClient(cfg, log)
	if _, _, ok := c.Jadwal(); ok {
		t.Fatal("no list before the first fetch")
	}
	if err := c.refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if auth.Load() != "Bearer "+cfg.DashboardToken {
		t.Fatalf("token not sent: %v", auth.Load())
	}
	list, _, ok := c.Jadwal()
	if !ok || len(list) != 2 || list[1].Level != "" || strings.Contains(list[1].Label, "[[") || strings.Contains(list[1].Catatan, "\n") {
		t.Fatalf("not sanitized: %+v", list)
	}

	fail.Store(true)
	if err := c.refresh(context.Background()); err == nil {
		t.Fatal("expected error")
	}
	if list, _, ok := c.Jadwal(); !ok || len(list) != 2 {
		t.Fatal("last good list must stay after a failure")
	}

	// A restart loads the cached list before the Dashboard answers.
	c2 := NewJadwalClient(cfg, log)
	if list, _, ok := c2.Jadwal(); !ok || len(list) != 2 {
		t.Fatalf("cache not loaded: %+v", list)
	}

	cfg.DashboardToken = ""
	if NewJadwalClient(cfg, log) != nil {
		t.Fatal("feature must be off without a token")
	}
}

func TestJadwalPromptShowsOnlyOpenOrFull(t *testing.T) {
	s := defaultSettings(testConfig())
	p := buildSystemPromptJadwal(s, "X", time.Now(), false, nil, testJadwal, true)
	for _, want := range []string{"`selasa-malam`", "khusus level 2", "ikhwan tersedia, akhwat PENUH", "2x sepekan, semua level", "[[JADWAL:<kode>]]"} {
		if !strings.Contains(p, want) {
			t.Fatalf("prompt misses %q", want)
		}
	}
	if strings.Contains(p, "sisa kursi") || strings.Contains(p, "ikhwan 3") {
		t.Fatal("seat numbers must not reach the AI")
	}
	p = buildSystemPrompt(s, "X", time.Now(), false, nil)
	if !strings.Contains(p, "belum ada data jadwal") {
		t.Fatal("missing no-data instruction")
	}
}

func TestJadwalMarkerIsHiddenAndRecordedAfterSend(t *testing.T) {
	s := defaultSettings(testConfig())
	out := renderReply("Baik, Kak, pilihan Rabu & Sabtu saya catat 😊\n[[JADWAL:Rabu-Sabtu-Malam]]", s)
	if out.JadwalKode != "rabu-sabtu-malam" || strings.Contains(out.Text, "[[") {
		t.Fatalf("marker not parsed: %+v", out)
	}

	agent := &fakeAgent{reply: "Pilihan Kakak sudah saya catat. [[JADWAL:rabu-sabtu-malam]]"}
	b, sender, _ := newTestBridge(t, testConfig(), agent)
	sink := &fakeSink{}
	b.events, b.jadwal = sink, testJadwal

	post(t, b, msg("J1", user, "Saya pilih Rabu Sabtu", false))
	b.Wait()
	if !strings.Contains(agent.all()[0].system, "`rabu-sabtu-malam`") {
		t.Fatal("schedules not in the system prompt")
	}
	if strings.Contains(sender.all()[0].text, "[[") {
		t.Fatal("marker leaked to WhatsApp")
	}
	ev := sink.find("schedule.selected")
	if len(ev) != 1 || ev[0].Data["jadwal_kode"] != "rabu-sabtu-malam" || ev[0].Data["wa_message_id"] == "" {
		t.Fatalf("schedule not recorded: %+v", ev)
	}

	// A code the Dashboard does not offer is not recorded.
	agent.reply = "Saya catat. [[JADWAL:kamis-pagi]]"
	post(t, b, msg("J2", user, "Kamis pagi bisa?", false))
	b.Wait()
	if len(sink.find("schedule.selected")) != 1 {
		t.Fatal("unknown schedule was recorded")
	}
}
