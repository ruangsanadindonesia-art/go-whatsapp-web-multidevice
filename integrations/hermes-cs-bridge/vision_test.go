package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// visionSender is a fakeSender that can also fetch images.
type visionSender struct {
	*fakeSender
	mu    sync.Mutex
	refs  []imageRef
	err   error
	image string
}

func (v *visionSender) FetchImage(_ context.Context, ref imageRef, _ int64) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.refs = append(v.refs, ref)
	if v.err != nil {
		return "", v.err
	}
	return v.image, nil
}

type visionCall struct {
	user   string
	images []string
}

// visionAgent records image calls; refuse=true answers like an old Hermes.
type visionAgent struct {
	fakeAgent
	vmu    sync.Mutex
	vcalls []visionCall
	refuse bool
}

func (a *visionAgent) ReplyWithImages(_ context.Context, _, _, user string, images []string) (string, error) {
	a.vmu.Lock()
	defer a.vmu.Unlock()
	a.vcalls = append(a.vcalls, visionCall{user, images})
	if a.refuse {
		return "", &hermesStatusError{status: 400, body: "unsupported_content_type"}
	}
	return a.reply, a.err
}

func newVisionBridge(t *testing.T, snd *visionSender, agent Agent) *Bridge {
	t.Helper()
	cfg := testConfig()
	cfg.VisionEnabled, cfg.VisionMaxBytes = true, 5<<20
	b, fs, _ := newTestBridge(t, cfg, &fakeAgent{})
	snd.fakeSender = fs
	b.gowa, b.agent = snd, agent
	return b
}

func imageMsg(id, caption string, image any) map[string]any {
	ev := msg(id, user, caption, false)
	ev["payload"].(map[string]any)["image"] = image
	return ev
}

func TestImageIsPassedToVisionAgent(t *testing.T) {
	snd := &visionSender{image: "data:image/jpeg;base64,AAAA"}
	agent := &visionAgent{fakeAgent: fakeAgent{reply: "Terima kasih, Kak"}}
	b := newVisionBridge(t, snd, agent)
	b.store.MarkPaymentSent(user, time.Now())

	post(t, b, imageMsg("IMG1", "", "statics/media/bukti.jpeg"))
	time.Sleep(100 * time.Millisecond)
	b.Wait()

	if len(agent.vcalls) != 1 || len(agent.vcalls[0].images) != 1 || agent.vcalls[0].images[0] != snd.image {
		t.Fatalf("vision calls = %+v", agent.vcalls)
	}
	if !strings.Contains(agent.vcalls[0].user, "terlampir") || !strings.Contains(agent.vcalls[0].user, "BUKTI TRANSFER") {
		t.Fatalf("note = %q", agent.vcalls[0].user)
	}
	if len(snd.refs) != 1 || snd.refs[0].Path != "statics/media/bukti.jpeg" || snd.refs[0].MessageID != "IMG1" {
		t.Fatalf("refs = %+v", snd.refs)
	}
	if len(agent.all()) != 0 {
		t.Fatal("text-only Reply must not be used when the image was read")
	}
}

func TestImageFetchFailureFallsBackToTextNote(t *testing.T) {
	snd := &visionSender{err: errors.New("gone")}
	agent := &visionAgent{fakeAgent: fakeAgent{reply: "ok"}}
	b := newVisionBridge(t, snd, agent)

	post(t, b, imageMsg("IMG2", "ini jadwalnya?", map[string]any{"path": "statics/media/x.jpeg", "caption": "ini jadwalnya?"}))
	time.Sleep(100 * time.Millisecond)
	b.Wait()

	calls := agent.all()
	if len(agent.vcalls) != 0 || len(calls) != 1 {
		t.Fatalf("vision=%d text=%d", len(agent.vcalls), len(calls))
	}
	if !strings.Contains(calls[0].user, "ini jadwalnya?") || !strings.Contains(calls[0].user, "tidak bisa Anda buka") {
		t.Fatalf("user = %q", calls[0].user)
	}
}

func TestHermesRefusingImagesRetriesTextOnly(t *testing.T) {
	snd := &visionSender{image: "data:image/png;base64,AAAA"}
	agent := &visionAgent{fakeAgent: fakeAgent{reply: "ok"}, refuse: true}
	b := newVisionBridge(t, snd, agent)

	post(t, b, imageMsg("IMG3", "", "statics/media/y.png"))
	time.Sleep(100 * time.Millisecond)
	b.Wait()

	calls := agent.all()
	if len(agent.vcalls) != 1 || len(calls) != 1 || !strings.Contains(calls[0].user, "tidak bisa membuka") {
		t.Fatalf("vision=%d text=%+v", len(agent.vcalls), calls)
	}
	for _, m := range snd.all() {
		if m.text == "FALLBACK" {
			t.Fatal("retry should have answered, not the fallback")
		}
	}
}

func TestStaticPath(t *testing.T) {
	for in, want := range map[string]string{
		"statics/media/a b.jpeg":    "/statics/media/a%20b.jpeg",
		"./statics/media/628/x.jpg": "/statics/media/628/x.jpg",
		"statics/../etc/passwd":     "",
		"/etc/passwd":               "",
		"media/x.jpg":               "",
	} {
		if got := staticPath(in); got != want {
			t.Errorf("staticPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGowaFetchImage(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path+"?"+r.URL.RawQuery)
		switch {
		case strings.HasPrefix(r.URL.Path, "/message/"):
			_ = json.NewEncoder(w).Encode(map[string]any{"results": map[string]string{"file_path": "statics/media/628/dl.png"}})
		case strings.HasPrefix(r.URL.Path, "/statics/"):
			_, _ = w.Write(png)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	cfg := testConfig()
	cfg.GowaURL = srv.URL
	g := NewGowaClient(cfg)

	got, err := g.FetchImage(context.Background(), imageRef{MessageID: "M1", ChatID: user}, 1<<20)
	if err != nil || !strings.HasPrefix(got, "data:image/png;base64,") {
		t.Fatalf("got=%q err=%v", got, err)
	}
	if len(paths) != 2 || !strings.HasPrefix(paths[0], "/message/M1/download?phone=6281234%40s.whatsapp.net") {
		t.Fatalf("paths = %v", paths)
	}
	if _, err := g.FetchImage(context.Background(), imageRef{Path: "statics/media/x.png"}, 4); err == nil {
		t.Fatal("oversized image must fail")
	}
}
