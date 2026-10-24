package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// Markers are model output controls, not customer-facing WhatsApp content.
func TestCSFunnelMarkersAndContextualCaptionParsing(t *testing.T) {
	s := defaultSettings(testConfig())
	s.Media = []MediaItem{{Code: "brosur", Caption: "Caption galeri", File: "brosur.png"}}

	reply := "Saya rangkum kebutuhan Kakak. [[TAHAP:discovery]] [[GAMBAR:brosur|Ini brosur untuk belajar pagi secara bertahap, Kak.]]"
	out := renderReply(reply, s)
	if out.FunnelStage != "discovery" || len(out.Images) != 1 ||
		out.Images[0].Caption != "Ini brosur untuk belajar pagi secara bertahap, Kak." ||
		strings.Contains(out.Text, "[[") {
		t.Fatalf("invalid discovery marker / caption rendering: %+v", out)
	}

	out = renderReply("Saya rekomendasikan *1× sepekan* — *Rp149.000*. [[TAHAP:rekomendasi:1x]]", s)
	if out.FunnelStage != "rekomendasi" || out.FunnelPackage != "1x" || strings.Contains(out.Text, "[[") {
		t.Fatalf("invalid recommendation marker: %+v", out)
	}
	out = renderReply("Informasi program [[TAHAP:rekomendasi:tidak-ada]]", s)
	if out.FunnelStage != "" || len(out.Warnings) == 0 {
		t.Fatalf("unknown package was accepted: %+v", out)
	}

	out = renderReply("[[GAMBAR:brosur]]", s)
	if len(out.Images) != 1 || out.Images[0].Caption != "Caption galeri" {
		t.Fatalf("backward-compatible default caption broken: %+v", out)
	}
	out = renderReply("[[GAMBAR:brosur|"+strings.Repeat("x", 1001)+"]]", s)
	if len(out.Images) != 1 || out.Images[0].Caption != "Caption galeri" {
		t.Fatalf("oversized caption not replaced by safe fallback: %+v", out)
	}
}

func TestCSFunnelEmitsOnlyAfterVerifiedWhatsAppSend(t *testing.T) {
	agent := &fakeAgent{reply: "Dari cerita Kakak, ingin memperbaiki bacaan dan nyaman belajar pagi. [[TAHAP:discovery]]"}
	b, sender, _ := newTestBridge(t, testConfig(), agent)
	sink := &fakeSink{}
	b.events = sink

	post(t, b, msg("FIN1", user, "Saya ingin belajar lagi, waktunya pagi", false))
	b.Wait()
	events := sink.find("conversation.stage")
	if len(events) != 1 || events[0].Data["stage"] != "discovery" || events[0].EventID == "" {
		t.Fatalf("missing discovery event: %+v", events)
	}
	if strings.Contains(sender.all()[0].text, "[[") {
		t.Fatalf("internal marker leaked to WhatsApp")
	}
	agent.reply = "Saya rekomendasikan Kelas Tahsin Reguler *1× sepekan* — *Rp149.000* untuk belajar sesuai level. [[TAHAP:rekomendasi:1x]]"
	post(t, b, msg("FIN2", user, "Boleh rekomendasikan yang 1x?", false))
	b.Wait()
	events = sink.find("conversation.stage")
	if len(events) != 2 || events[1].Data["stage"] != "rekomendasi" || events[1].Data["paket_kode"] != "1x" {
		t.Fatalf("missing recommendation event: %+v", events)
	}

	// An AI marker alone is not evidence of a true offer.
	agent.reply = "Ini harga paket Rp999.999. [[TAHAP:rekomendasi:1x]]"
	post(t, b, msg("FIN3", user, "Bisa jelaskan?", false))
	b.Wait()
	if got := len(sink.find("conversation.stage")); got != 2 {
		t.Fatalf("invalid offer advanced funnel: %d events", got)
	}
}

type failedCSFunnelTextSender struct{}

func (failedCSFunnelTextSender) SendText(context.Context, string, string) (string, error) {
	return "", errors.New("gowa down")
}
func (failedCSFunnelTextSender) SendImage(context.Context, string, string, string, string) (string, error) {
	return "", errors.New("gowa down")
}
func (failedCSFunnelTextSender) Typing(context.Context, string, bool) {}

func TestCSFunnelNeverAdvancesAfterFailedSend(t *testing.T) {
	b, _, _ := newTestBridge(t, testConfig(), &fakeAgent{
		reply: "Kelas Tahsin Reguler 1× sepekan — Rp149.000. [[TAHAP:rekomendasi:1x]]",
	})
	b.gowa = failedCSFunnelTextSender{}
	sink := &fakeSink{}
	b.events = sink
	post(t, b, msg("NO-SEND", user, "Pagi 1x sepekan", false))
	b.Wait()
	if len(sink.find("conversation.stage")) != 0 {
		t.Fatal("funnel changed despite a failed GOWA send")
	}
}

func TestCSAIImageUsesGeneratedCaptionInWhatsAppAndCRM(t *testing.T) {
	agent := &fakeAgent{reply: "Ini brosurnya, Kak. [[GAMBAR:brosur|Berikut brosur Tahsin untuk Kakak yang ingin belajar bertahap pada pagi hari.]]"}
	b, snd, _ := newTestBridge(t, adminConfig(), agent)
	sink := &fakeSink{}
	b.events = sink
	if resp := uploadImage(t, b, "brosur", "Info kelas saat lead meminta brosur", "Caption statis galeri", tinyPNG(t)); resp.Code != 200 {
		t.Fatalf("upload: %d: %s", resp.Code, resp.Body)
	}
	post(t, b, msg("IMG-CAP", user, "Boleh minta brosur Tahsin untuk pagi?", false))
	b.Wait()
	all := snd.all()
	if len(all) != 2 || !strings.Contains(all[1].text, "Berikut brosur Tahsin untuk Kakak") || strings.Contains(all[1].text, "Caption statis galeri") {
		t.Fatalf("contextual caption not used in real image send: %+v", all)
	}
	out := sink.find("message.out")
	if len(out) != 2 || out[1].Data["media_type"] != "gambar" ||
		!strings.Contains(out[1].Data["body"].(string), "Berikut brosur Tahsin untuk Kakak") {
		t.Fatalf("CRM did not store actual image caption: %+v", out)
	}
}

