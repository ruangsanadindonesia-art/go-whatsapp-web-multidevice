package usecase

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	domainCampaign "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/campaign"
	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	"github.com/stretchr/testify/require"
)

var campWIB, _ = time.LoadLocation("Asia/Jakarta")

type campaignNames map[string]string

func (n campaignNames) ChatNames(_ context.Context, _ string, jids []string) (map[string]string, error) {
	out := map[string]string{}
	for _, j := range jids {
		if v, ok := n[j]; ok {
			out[j] = v
		}
	}
	return out, nil
}

func newCampaignTest(t *testing.T) (*serviceCampaign, *ScheduleService, domainChatStorage.IChatStorageRepository) {
	t.Helper()
	sched, repo := newScheduleTestService(t, nil)
	svc := NewCampaignService(repo, sched, campaignNames{"6281111@s.whatsapp.net": "Fulan"}, sched.mediaRoot, campWIB).(*serviceCampaign)
	svc.now = func() time.Time { return scheduleTestNow }
	return svc, sched, repo
}

func campJob(id, phone, msg string, at time.Time, recurrence, status string) *domainChatStorage.ScheduledSend {
	return &domainChatStorage.ScheduledSend{
		ID: id, DeviceID: "device-a", MessageType: "text",
		PayloadJSON: `{"phone":"` + phone + `","message":"` + msg + `"}`, AssetsJSON: "{}",
		Phone: phone, Summary: msg, ScheduledAt: at, NextRunAt: at, Timezone: "Asia/Jakarta",
		Recurrence: recurrence, WeekdaysJSON: "[]", Status: status, CreatedAt: at.Add(-time.Hour),
	}
}

func TestCampaignKeyIgnoresPhoneButNotTime(t *testing.T) {
	at := scheduleTestNow.Add(time.Hour)
	a := campaignKey(campJob("a", "6281111", "halo", at, "once", "active"))
	b := campaignKey(campJob("b", "6282222", "halo", at, "once", "active"))
	c := campaignKey(campJob("c", "6281111", "halo", at.Add(time.Hour), "once", "active"))
	d := campaignKey(campJob("d", "6281111", "lain", at, "once", "active"))
	require.Equal(t, a, b)
	require.NotEqual(t, a, c)
	require.NotEqual(t, a, d)
	require.Len(t, a, 16)
}

func TestCampaignStatusDerivation(t *testing.T) {
	for want, counts := range map[string]map[string]int{
		domainCampaign.StatusActive:    {"active": 1, "paused": 2},
		domainCampaign.StatusPaused:    {"paused": 2, "completed": 1},
		domainCampaign.StatusFailed:    {"failed": 1, "completed": 3},
		domainCampaign.StatusDone:      {"completed": 2, "cancelled": 1},
		domainCampaign.StatusCancelled: {"cancelled": 2},
	} {
		require.Equal(t, want, deriveCampaignStatus(counts), counts)
	}
}

func TestCampaignListGroupsRecipients(t *testing.T) {
	svc, _, repo := newCampaignTest(t)
	at := scheduleTestNow.Add(time.Hour)
	for _, j := range []*domainChatStorage.ScheduledSend{
		campJob("s1", "6281111", "Kelas tahsin", at, "once", "active"),
		campJob("s2", "6282222", "Kelas tahsin", at, "once", "paused"),
		campJob("s3", "6283333", "Info lain", at, "once", "active"),
	} {
		require.NoError(t, repo.CreateScheduledSend(j))
	}
	list, err := svc.List(scheduleTestContext(), domainCampaign.ListFilter{})
	require.NoError(t, err)
	require.Len(t, list, 2)
	var tahsin domainCampaign.Campaign
	for _, c := range list {
		if c.Summary == "Kelas tahsin" {
			tahsin = c
		}
	}
	require.Equal(t, 2, tahsin.Recipients)
	require.Equal(t, domainCampaign.StatusActive, tahsin.Status)
	require.Equal(t, 1, tahsin.StatusCounts["paused"])

	other, err := svc.List(context.Background(), domainCampaign.ListFilter{})
	require.Error(t, err, "no device in context")
	require.Nil(t, other)
}

func TestCampaignPreviewDecodesTypes(t *testing.T) {
	cases := []struct {
		typ, payload, assets string
		check                func(p domainCampaign.MessagePreview)
	}{
		{"text", `{"phone":"1","message":"Assalamu'alaikum"}`, "{}", func(p domainCampaign.MessagePreview) { require.Equal(t, "Assalamu'alaikum", p.Text) }},
		{"image", `{"phone":"1","caption":"Brosur","image_url":"https://x/b.jpg","view_once":true}`, "{}", func(p domainCampaign.MessagePreview) {
			require.Equal(t, "Brosur", p.Caption)
			require.Equal(t, "https://x/b.jpg", p.Media.URL)
			require.True(t, p.ViewOnce)
		}},
		{"file", `{"phone":"1","caption":"PDF"}`, `{"file":{"path":"/x/file-a.pdf","filename":"a.pdf","content_type":"application/pdf"}}`, func(p domainCampaign.MessagePreview) {
			require.Equal(t, "a.pdf", p.Media.Filename)
			require.Equal(t, "job1", p.Media.ScheduleID)
		}},
		{"poll", `{"phone":"1","question":"Jadwal?","options":["Pagi","Malam"],"max_answer":1}`, "{}", func(p domainCampaign.MessagePreview) {
			require.Equal(t, []string{"Pagi", "Malam"}, p.PollOptions)
			require.Equal(t, 1, p.PollMaxAnswer)
		}},
		{"location", `{"phone":"1","latitude":"-6.2","longitude":"106.8"}`, "{}", func(p domainCampaign.MessagePreview) { require.Equal(t, "-6.2", p.Latitude) }},
		{"contact", `{"phone":"1","contact_name":"Admin","contact_phone":"628"}`, "{}", func(p domainCampaign.MessagePreview) { require.Equal(t, "Admin", p.ContactName) }},
		{"link", `{"phone":"1","link":"https://rs.id","caption":"Daftar"}`, "{}", func(p domainCampaign.MessagePreview) { require.Equal(t, "https://rs.id", p.Link) }},
		{"audio", `{"phone":"1","ptt":true}`, `{"audio":{"path":"/x/a.ogg","filename":"a.ogg"}}`, func(p domainCampaign.MessagePreview) { require.True(t, p.VoiceNote) }},
		{"text", `{broken`, "{}", func(p domainCampaign.MessagePreview) { require.Equal(t, "{broken", p.Raw) }},
	}
	for _, c := range cases {
		job := &domainChatStorage.ScheduledSend{ID: "job1", MessageType: c.typ, PayloadJSON: c.payload, AssetsJSON: c.assets}
		p := decodePreview(job)
		require.Equal(t, c.typ, p.Type)
		c.check(p)
	}
}

func TestCampaignCalendarProjectsAndCaps(t *testing.T) {
	svc, _, repo := newCampaignTest(t)
	start := time.Date(2026, 11, 23, 8, 0, 0, 0, campWIB)
	require.NoError(t, repo.CreateScheduledSend(campJob("d1", "6281111", "Harian", start, "daily", "active")))
	paused := campJob("p1", "6282222", "Jeda", start.Add(30*time.Minute), "once", "paused")
	require.NoError(t, repo.CreateScheduledSend(paused))

	from := time.Date(2026, 11, 1, 0, 0, 0, 0, campWIB)
	events, err := svc.Calendar(scheduleTestContext(), from, from.AddDate(2, 0, 0))
	require.NoError(t, err)
	var daily, pausedN int
	for _, e := range events {
		switch e.Summary {
		case "Harian":
			daily++
			require.Equal(t, "upcoming", e.Kind)
		case "Jeda":
			pausedN++
			require.Equal(t, "paused", e.Kind)
		}
	}
	require.Equal(t, maxProjectedOccurrences, daily, "projection is capped")
	require.Equal(t, 1, pausedN)
	require.True(t, events[0].At.Equal(start.UTC()) || events[0].At.Equal(start), events[0].At)
}

func TestCampaignCalendarRespectsOccurrenceLimit(t *testing.T) {
	svc, _, repo := newCampaignTest(t)
	start := time.Date(2026, 11, 23, 8, 0, 0, 0, campWIB)
	j := campJob("l1", "6281111", "Tiga kali", start, "daily", "active")
	j.OccurrenceLimit, j.OccurrenceCount = 3, 1
	require.NoError(t, repo.CreateScheduledSend(j))
	events, err := svc.Calendar(scheduleTestContext(), start.AddDate(0, 0, -1), start.AddDate(0, 1, 0))
	require.NoError(t, err)
	require.Len(t, events, 2, "limit 3, one already sent")
}

func TestCampaignDashboardCountsRuns(t *testing.T) {
	svc, _, repo := newCampaignTest(t)
	at := time.Date(2026, 11, 20, 9, 0, 0, 0, campWIB)
	require.NoError(t, repo.CreateScheduledSend(campJob("s1", "6281111", "Kelas", at, "once", "completed")))
	require.NoError(t, repo.CreateScheduledSend(campJob("s2", "6282222", "Kelas", at, "once", "failed")))
	for _, r := range []domainChatStorage.ScheduledSendRun{
		{ScheduleID: "s1", DeviceID: "device-a", OccurrenceAt: at, ExecutedAt: at.Add(time.Second), Status: "sent"},
		{ScheduleID: "s2", DeviceID: "device-a", OccurrenceAt: at, ExecutedAt: at.Add(time.Minute), Status: "failed", Error: "not on whatsapp"},
	} {
		r := r
		require.NoError(t, repo.RecordScheduledSendRun(&r))
	}
	from := time.Date(2026, 11, 20, 0, 0, 0, 0, campWIB)
	d, err := svc.Dashboard(scheduleTestContext(), from, from.AddDate(0, 0, 3))
	require.NoError(t, err)
	require.Equal(t, 1, d.Totals.Sent)
	require.Equal(t, 1, d.Totals.Failed)
	require.InDelta(t, 0.5, d.Totals.SuccessRate, 1e-9)
	require.Equal(t, 1, d.Totals.Recipients)
	require.Equal(t, 1, d.Totals.Campaigns)
	require.Len(t, d.Daily, 3)
	require.Equal(t, "2026-11-20", d.Daily[0].Day)
	require.Equal(t, 1, d.Daily[0].Sent)
	require.Equal(t, []domainCampaign.TypeCount{{Type: "text", Sent: 1, Failed: 1}}, d.ByType)
	require.Len(t, d.TopCampaigns, 1)
	require.Len(t, d.RecentFailures, 1)
	require.Equal(t, "6282222", d.RecentFailures[0].Phone)
	require.False(t, d.Approximate, "log starts inside the range day")

	earlier, err := svc.Dashboard(scheduleTestContext(), from.AddDate(0, 0, -7), from)
	require.NoError(t, err)
	require.True(t, earlier.Approximate, "range starts before the first logged run")
}

func TestCampaignActionPausesMembers(t *testing.T) {
	svc, _, repo := newCampaignTest(t)
	at := scheduleTestNow.Add(time.Hour)
	require.NoError(t, repo.CreateScheduledSend(campJob("s1", "6281111", "Kelas", at, "once", "active")))
	require.NoError(t, repo.CreateScheduledSend(campJob("s2", "6282222", "Kelas", at, "once", "completed")))
	key := campaignKey(campJob("x", "1", "Kelas", at, "once", "active"))

	res, err := svc.Action(scheduleTestContext(), key, "pause")
	require.NoError(t, err)
	require.Equal(t, domainCampaign.ActionResult{Changed: 1, Skipped: 1}, res)
	job, _ := repo.GetScheduledSend("device-a", "s1")
	require.Equal(t, "paused", job.Status)

	_, err = svc.Action(scheduleTestContext(), key, "explode")
	require.Error(t, err)
	_, err = svc.Action(scheduleTestContext(), "nope", "pause")
	require.ErrorIs(t, err, ErrCampaignNotFound)
}

func TestCampaignAssetScopedToJobDir(t *testing.T) {
	svc, sched, repo := newCampaignTest(t)
	dir := filepath.Join(sched.mediaRoot, "scheduled", "img1")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	inside := filepath.Join(dir, "image-a.jpg")
	require.NoError(t, os.WriteFile(inside, []byte("jpg"), 0o600))
	j := campJob("img1", "6281111", "Brosur", scheduleTestNow, "once", "active")
	j.MessageType = "image"
	j.AssetsJSON = `{"image":{"path":"` + inside + `","filename":"a.jpg","content_type":"image/jpeg"},"evil":{"path":"/etc/passwd","filename":"p"}}`
	require.NoError(t, repo.CreateScheduledSend(j))

	a, err := svc.Asset(scheduleTestContext(), "img1", "image")
	require.NoError(t, err)
	require.Equal(t, inside, a.Path)
	require.Equal(t, "image/jpeg", a.ContentType)

	_, err = svc.Asset(scheduleTestContext(), "img1", "evil")
	require.ErrorIs(t, err, ErrCampaignNotFound)
	_, err = svc.Asset(scheduleTestContext(), "missing", "image")
	require.ErrorIs(t, err, ErrCampaignNotFound)
}

func TestCampaignGetDetail(t *testing.T) {
	svc, _, repo := newCampaignTest(t)
	at := scheduleTestNow.Add(time.Hour)
	require.NoError(t, repo.CreateScheduledSend(campJob("s1", "6281111", "Kelas", at, "once", "active")))
	key := campaignKey(campJob("x", "1", "Kelas", at, "once", "active"))
	d, err := svc.Get(scheduleTestContext(), key)
	require.NoError(t, err)
	require.Equal(t, "Kelas", d.Message.Text)
	require.Len(t, d.RecipientList, 1)
	require.Equal(t, "Fulan", d.RecipientList[0].Name)
	require.NotNil(t, d.Runs)

	_, err = svc.Get(scheduleTestContext(), "unknown")
	require.ErrorIs(t, err, ErrCampaignNotFound)
}
