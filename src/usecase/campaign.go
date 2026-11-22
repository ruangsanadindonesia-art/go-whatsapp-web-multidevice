package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	domainCampaign "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/campaign"
	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	domainSend "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/send"
	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/validations"
)

// ErrCampaignNotFound aliases the domain error.
var ErrCampaignNotFound = domainCampaign.ErrNotFound

// maxProjectedOccurrences caps projected occurrences per schedule per request.
const maxProjectedOccurrences = 500

// ChatNameLookup resolves stored chat names (see infrastructure/analytics).
type ChatNameLookup interface {
	ChatNames(ctx context.Context, deviceID string, jids []string) (map[string]string, error)
}

type serviceCampaign struct {
	repo      domainChatStorage.IChatStorageRepository
	schedules domainSend.IScheduleUsecase
	names     ChatNameLookup
	mediaRoot string
	loc       *time.Location
	now       func() time.Time
}

func NewCampaignService(repo domainChatStorage.IChatStorageRepository, schedules domainSend.IScheduleUsecase, names ChatNameLookup, mediaRoot string, loc *time.Location) domainCampaign.ICampaignUsecase {
	return &serviceCampaign{repo: repo, schedules: schedules, names: names, mediaRoot: mediaRoot, loc: loc, now: time.Now}
}

func (s *serviceCampaign) Location() *time.Location { return s.loc }

// group is one campaign with its member schedules.
type group struct {
	key  string
	jobs []*domainChatStorage.ScheduledSend
}

// campaignKey identifies the campaign a schedule belongs to.
func campaignKey(job *domainChatStorage.ScheduledSend) string {
	var payload map[string]any
	canonical := job.PayloadJSON
	if err := json.Unmarshal([]byte(job.PayloadJSON), &payload); err == nil {
		delete(payload, "phone")
		if b, err := json.Marshal(payload); err == nil { // map keys marshal sorted
			canonical = string(b)
		}
	}
	// Asset paths differ per job; the filenames identify the media.
	var assets map[string]scheduledAsset
	var files []string
	if json.Unmarshal([]byte(job.AssetsJSON), &assets) == nil {
		for field, a := range assets {
			files = append(files, field+"="+a.Filename)
		}
		sort.Strings(files)
	}
	parts := []string{
		job.DeviceID, job.MessageType, canonical, strings.Join(files, ","),
		job.ScheduledAt.UTC().Truncate(time.Minute).Format(time.RFC3339),
		job.Recurrence, job.Timezone, job.WeekdaysJSON, fmt.Sprint(job.DayOfMonth),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x1f")))
	return hex.EncodeToString(sum[:])[:16]
}

func deriveCampaignStatus(counts map[string]int) string {
	switch {
	case counts[scheduleStatusActive]+counts[scheduleStatusRunning] > 0:
		return domainCampaign.StatusActive
	case counts[scheduleStatusPaused] > 0:
		return domainCampaign.StatusPaused
	case counts[scheduleStatusFailed] > 0:
		return domainCampaign.StatusFailed
	case counts[scheduleStatusCompleted] > 0:
		return domainCampaign.StatusDone
	default:
		return domainCampaign.StatusCancelled
	}
}

func (s *serviceCampaign) device(ctx context.Context) (string, error) {
	id := scheduleDeviceID(ctx)
	if id == "" {
		return "", pkgError.ValidationError("campaign: device is required")
	}
	return id, nil
}

// load returns every schedule of the device grouped into campaigns, plus a
// schedule-id → group index.
func (s *serviceCampaign) load(ctx context.Context) (string, []*group, map[string]*group, error) {
	device, err := s.device(ctx)
	if err != nil {
		return "", nil, nil, err
	}
	jobs, err := s.repo.ListScheduledSends(domainChatStorage.ScheduledSendFilter{DeviceID: device})
	if err != nil {
		return "", nil, nil, err
	}
	byKey := map[string]*group{}
	var groups []*group
	byJob := map[string]*group{}
	for _, job := range jobs {
		key := campaignKey(job)
		g := byKey[key]
		if g == nil {
			g = &group{key: key}
			byKey[key] = g
			groups = append(groups, g)
		}
		g.jobs = append(g.jobs, job)
		byJob[job.ID] = g
	}
	return device, groups, byJob, nil
}

func (s *serviceCampaign) runs(device string, from, to time.Time, scheduleIDs []string) ([]*domainChatStorage.ScheduledSendRun, error) {
	return s.repo.ListScheduledSendRuns(domainChatStorage.ScheduledSendRunFilter{DeviceID: device, From: from, To: to, ScheduleIDs: scheduleIDs})
}

func summarizeGroup(g *group, runs []*domainChatStorage.ScheduledSendRun) domainCampaign.Campaign {
	first := g.jobs[0]
	c := domainCampaign.Campaign{
		Key: g.key, MessageType: first.MessageType, Summary: first.Summary, Recurrence: first.Recurrence,
		DayOfMonth: first.DayOfMonth, Timezone: first.Timezone, ScheduledAt: first.ScheduledAt, EndAt: first.EndAt,
		OccurrenceLimit: first.OccurrenceLimit, Recipients: len(g.jobs), StatusCounts: map[string]int{}, CreatedAt: first.CreatedAt,
	}
	_ = json.Unmarshal([]byte(first.WeekdaysJSON), &c.Weekdays)
	for _, job := range g.jobs {
		c.StatusCounts[job.Status]++
		if isPending(job.Status) && (c.NextRunAt == nil || job.NextRunAt.Before(*c.NextRunAt)) {
			t := job.NextRunAt
			c.NextRunAt = &t
		}
		if job.LastRunAt != nil && (c.LastRunAt == nil || job.LastRunAt.After(*c.LastRunAt)) {
			t := *job.LastRunAt
			c.LastRunAt = &t
		}
		if job.CreatedAt.Before(c.CreatedAt) {
			c.CreatedAt = job.CreatedAt
		}
	}
	for _, r := range runs {
		if r.Status == "sent" {
			c.Sent++
		} else {
			c.Failed++
		}
	}
	c.Status = deriveCampaignStatus(c.StatusCounts)
	return c
}

func isPending(status string) bool {
	return status == scheduleStatusActive || status == scheduleStatusRunning || status == scheduleStatusPaused
}

func jobIDs(g *group) []string {
	ids := make([]string, len(g.jobs))
	for i, j := range g.jobs {
		ids[i] = j.ID
	}
	return ids
}

func (s *serviceCampaign) List(ctx context.Context, filter domainCampaign.ListFilter) ([]domainCampaign.Campaign, error) {
	device, groups, byJob, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	allRuns, err := s.runs(device, time.Time{}, time.Time{}, nil)
	if err != nil {
		return nil, err
	}
	runsBy := map[*group][]*domainChatStorage.ScheduledSendRun{}
	inRange := map[*group]bool{}
	for _, r := range allRuns {
		if g := byJob[r.ScheduleID]; g != nil {
			runsBy[g] = append(runsBy[g], r)
			if !filter.From.IsZero() && !r.ExecutedAt.Before(filter.From) && r.ExecutedAt.Before(filter.To) {
				inRange[g] = true
			}
		}
	}
	q := strings.ToLower(strings.TrimSpace(filter.Query))
	out := make([]domainCampaign.Campaign, 0, len(groups))
	for _, g := range groups {
		c := summarizeGroup(g, runsBy[g])
		if filter.Status != "" && c.Status != filter.Status {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(c.Summary), q) && !groupHasPhone(g, q) {
			continue
		}
		if !filter.From.IsZero() && !inRange[g] && len(s.project(g, filter.From, filter.To)) == 0 {
			continue
		}
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if (a.NextRunAt != nil) != (b.NextRunAt != nil) {
			return a.NextRunAt != nil
		}
		if a.NextRunAt != nil {
			return a.NextRunAt.Before(*b.NextRunAt)
		}
		return lastActivity(a).After(lastActivity(b))
	})
	return out, nil
}

func groupHasPhone(g *group, q string) bool {
	for _, j := range g.jobs {
		if strings.Contains(j.Phone, q) {
			return true
		}
	}
	return false
}

func lastActivity(c domainCampaign.Campaign) time.Time {
	if c.LastRunAt != nil {
		return *c.LastRunAt
	}
	return c.CreatedAt
}

func (s *serviceCampaign) find(ctx context.Context, key string) (string, *group, error) {
	device, groups, _, err := s.load(ctx)
	if err != nil {
		return "", nil, err
	}
	for _, g := range groups {
		if g.key == key {
			return device, g, nil
		}
	}
	return "", nil, ErrCampaignNotFound
}

func (s *serviceCampaign) Get(ctx context.Context, key string) (*domainCampaign.Detail, error) {
	device, g, err := s.find(ctx, key)
	if err != nil {
		return nil, err
	}
	runs, err := s.runs(device, time.Time{}, time.Time{}, jobIDs(g))
	if err != nil {
		return nil, err
	}
	d := &domainCampaign.Detail{Campaign: summarizeGroup(g, runs), Message: decodePreview(g.jobs[0])}
	phoneOf := map[string]string{}
	jids := make([]string, 0, len(g.jobs))
	for _, j := range g.jobs {
		phoneOf[j.ID] = j.Phone
		jids = append(jids, phoneJID(j.Phone))
	}
	names := map[string]string{}
	if s.names != nil {
		if n, err := s.names.ChatNames(ctx, deviceIDFromContext(ctx), jids); err == nil {
			names = n
		}
	}
	d.RecipientList = make([]domainCampaign.Recipient, 0, len(g.jobs))
	for _, j := range g.jobs {
		r := domainCampaign.Recipient{
			ScheduleID: j.ID, Phone: j.Phone, Name: names[phoneJID(j.Phone)], Status: j.Status,
			OccurrenceCount: j.OccurrenceCount, LastRunAt: j.LastRunAt, LastError: j.LastError,
		}
		if isPending(j.Status) {
			t := j.NextRunAt
			r.NextRunAt = &t
		}
		d.RecipientList = append(d.RecipientList, r)
	}
	d.Runs = make([]domainCampaign.Run, 0, len(runs))
	for i := len(runs) - 1; i >= 0 && len(d.Runs) < 200; i-- { // newest first
		d.Runs = append(d.Runs, toRun(runs[i], phoneOf[runs[i].ScheduleID], "", ""))
	}
	return d, nil
}

func phoneJID(phone string) string {
	if strings.Contains(phone, "@") {
		return phone
	}
	return phone + "@s.whatsapp.net"
}

func toRun(r *domainChatStorage.ScheduledSendRun, phone, key, summary string) domainCampaign.Run {
	return domainCampaign.Run{
		ScheduleID: r.ScheduleID, Phone: phone, CampaignKey: key, Summary: summary, OccurrenceAt: r.OccurrenceAt,
		ExecutedAt: r.ExecutedAt, Status: r.Status, MessageID: r.MessageID, Error: r.Error,
	}
}

// project lists a group's future occurrences in [from, to): pending schedules
// only, capped per schedule.
func (s *serviceCampaign) project(g *group, from, to time.Time) []projected {
	var out []projected
	for _, job := range g.jobs {
		if !isPending(job.Status) {
			continue
		}
		loc, err := time.LoadLocation(job.Timezone)
		if err != nil {
			continue
		}
		var weekdays []int
		_ = json.Unmarshal([]byte(job.WeekdaysJSON), &weekdays)
		spec := validations.ScheduleSpec{ScheduledAt: job.ScheduledAt, Timezone: job.Timezone, Location: loc, Recurrence: job.Recurrence, Weekdays: weekdays, DayOfMonth: job.DayOfMonth, EndAt: job.EndAt, OccurrenceLimit: job.OccurrenceLimit}
		at, count, n := job.NextRunAt, job.OccurrenceCount, 0
		for n < maxProjectedOccurrences && at.Before(to) {
			if job.EndAt != nil && at.After(*job.EndAt) {
				break
			}
			if !at.Before(from) {
				out = append(out, projected{job: job, at: at})
				n++
			}
			count++
			if job.OccurrenceLimit > 0 && count >= job.OccurrenceLimit {
				break
			}
			next, ok := validations.NextScheduleOccurrence(spec, at)
			if !ok || !next.After(at) {
				break
			}
			at = next
		}
	}
	return out
}

type projected struct {
	job *domainChatStorage.ScheduledSend
	at  time.Time
}

func (s *serviceCampaign) Calendar(ctx context.Context, from, to time.Time) ([]domainCampaign.CalendarEvent, error) {
	device, groups, byJob, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	runs, err := s.runs(device, from, to, nil)
	if err != nil {
		return nil, err
	}
	type slot struct {
		key  string
		at   time.Time
		kind string // "log" or projected kind
	}
	events := map[slot]*domainCampaign.CalendarEvent{}
	var order []slot
	add := func(k slot, g *group) *domainCampaign.CalendarEvent {
		e := events[k]
		if e == nil {
			first := g.jobs[0]
			e = &domainCampaign.CalendarEvent{Key: g.key, Summary: first.Summary, MessageType: first.MessageType, At: k.at}
			events[k] = e
			order = append(order, k)
		}
		return e
	}
	for _, r := range runs {
		g := byJob[r.ScheduleID]
		if g == nil {
			continue
		}
		e := add(slot{g.key, r.OccurrenceAt.UTC().Truncate(time.Minute), "log"}, g)
		e.Recipients++
		if r.Status == "sent" {
			e.Sent++
		} else {
			e.Failed++
		}
	}
	now := s.now()
	for _, g := range groups {
		for _, p := range s.project(g, maxTime(from, now.Add(-time.Minute)), to) {
			kind := "upcoming"
			if p.job.Status == scheduleStatusPaused {
				kind = "paused"
			}
			e := add(slot{g.key, p.at.UTC().Truncate(time.Minute), kind}, g)
			e.Kind = kind
			e.Recipients++
		}
	}
	out := make([]domainCampaign.CalendarEvent, 0, len(order))
	for _, k := range order {
		e := events[k]
		if k.kind == "log" {
			switch {
			case e.Failed == 0:
				e.Kind = "sent"
			case e.Sent == 0:
				e.Kind = "failed"
			default:
				e.Kind = "partial"
			}
		}
		out = append(out, *e)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out, nil
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func (s *serviceCampaign) Dashboard(ctx context.Context, from, to time.Time) (domainCampaign.Dashboard, error) {
	device, groups, byJob, err := s.load(ctx)
	if err != nil {
		return domainCampaign.Dashboard{}, err
	}
	runs, err := s.runs(device, from, to, nil)
	if err != nil {
		return domainCampaign.Dashboard{}, err
	}
	d := domainCampaign.Dashboard{
		From: from.In(s.loc).Format("2006-01-02"), To: to.Add(-time.Nanosecond).In(s.loc).Format("2006-01-02"),
		ByType: []domainCampaign.TypeCount{}, TopCampaigns: []domainCampaign.CampaignStat{}, RecentFailures: []domainCampaign.Run{},
	}
	first, err := s.repo.FirstScheduledSendRunAt(device)
	if err != nil {
		return d, err
	}
	d.LogSince = first
	// Runs are logged since this feature shipped; a range that starts on an
	// earlier day may miss sends.
	d.Approximate = first == nil || first.In(s.loc).Format("2006-01-02") > from.In(s.loc).Format("2006-01-02")

	dayIndex := map[string]int{}
	for day := from; day.Before(to); day = day.AddDate(0, 0, 1) {
		key := day.In(s.loc).Format("2006-01-02")
		dayIndex[key] = len(d.Daily)
		d.Daily = append(d.Daily, domainCampaign.DayPoint{Day: key})
	}
	typeIdx := map[string]int{}
	stats := map[*group]*domainCampaign.CampaignStat{}
	phones := map[string]bool{}
	for _, r := range runs {
		g := byJob[r.ScheduleID]
		msgType, phone, summary, key := "?", "", "(jadwal tidak ditemukan)", ""
		if g != nil {
			for _, j := range g.jobs {
				if j.ID == r.ScheduleID {
					phone = j.Phone
				}
			}
			msgType, summary, key = g.jobs[0].MessageType, g.jobs[0].Summary, g.key
		}
		i, ok := typeIdx[msgType]
		if !ok {
			i = len(d.ByType)
			typeIdx[msgType] = i
			d.ByType = append(d.ByType, domainCampaign.TypeCount{Type: msgType})
		}
		di, inDays := dayIndex[r.ExecutedAt.In(s.loc).Format("2006-01-02")]
		if g != nil && stats[g] == nil {
			c := summarizeGroup(g, nil)
			stats[g] = &domainCampaign.CampaignStat{Key: g.key, Summary: c.Summary, MessageType: c.MessageType, Status: c.Status}
		}
		if r.Status == "sent" {
			d.Totals.Sent++
			d.ByType[i].Sent++
			if inDays {
				d.Daily[di].Sent++
			}
			if g != nil {
				stats[g].Sent++
			}
			phones[phone] = true
		} else {
			d.Totals.Failed++
			d.ByType[i].Failed++
			if inDays {
				d.Daily[di].Failed++
			}
			if g != nil {
				stats[g].Failed++
			}
			d.RecentFailures = append(d.RecentFailures, toRun(r, phone, key, summary))
		}
	}
	if total := d.Totals.Sent + d.Totals.Failed; total > 0 {
		d.Totals.SuccessRate = float64(d.Totals.Sent) / float64(total)
	}
	d.Totals.Recipients = len(phones)
	d.Totals.Campaigns = len(stats)
	for _, st := range stats {
		d.TopCampaigns = append(d.TopCampaigns, *st)
	}
	sort.SliceStable(d.TopCampaigns, func(i, j int) bool {
		a, b := d.TopCampaigns[i], d.TopCampaigns[j]
		if a.Sent+a.Failed != b.Sent+b.Failed {
			return a.Sent+a.Failed > b.Sent+b.Failed
		}
		return a.Key < b.Key
	})
	if len(d.TopCampaigns) > 5 {
		d.TopCampaigns = d.TopCampaigns[:5]
	}
	sort.SliceStable(d.RecentFailures, func(i, j int) bool { return d.RecentFailures[i].ExecutedAt.After(d.RecentFailures[j].ExecutedAt) })
	if len(d.RecentFailures) > 10 {
		d.RecentFailures = d.RecentFailures[:10]
	}
	now := s.now()
	for _, g := range groups {
		if deriveCampaignStatus(summarizeGroup(g, nil).StatusCounts) == domainCampaign.StatusActive {
			d.Totals.ActiveCampaigns++
		}
		for _, p := range s.project(g, maxTime(from, now), to) {
			if p.job.Status == scheduleStatusPaused {
				continue
			}
			d.Totals.Upcoming++
			if di, ok := dayIndex[p.at.In(s.loc).Format("2006-01-02")]; ok {
				d.Daily[di].Upcoming++
			}
		}
	}
	return d, nil
}

func (s *serviceCampaign) Action(ctx context.Context, key, action string) (domainCampaign.ActionResult, error) {
	var res domainCampaign.ActionResult
	var apply func(context.Context, string) error
	var allowed []string
	switch action {
	case "pause":
		apply, allowed = s.schedules.Pause, []string{scheduleStatusActive}
	case "resume":
		apply, allowed = s.schedules.Resume, []string{scheduleStatusPaused}
	case "cancel":
		apply, allowed = s.schedules.Cancel, []string{scheduleStatusActive, scheduleStatusPaused, scheduleStatusFailed}
	default:
		return res, pkgError.ValidationError("campaign: unknown action " + action)
	}
	_, g, err := s.find(ctx, key)
	if err != nil {
		return res, err
	}
	for _, job := range g.jobs {
		if !contains(allowed, job.Status) {
			res.Skipped++
			continue
		}
		if err := apply(ctx, job.ID); err != nil {
			res.Skipped++ // a concurrent change (e.g. the worker leased it) is not fatal
			continue
		}
		res.Changed++
	}
	return res, nil
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func (s *serviceCampaign) Asset(ctx context.Context, scheduleID, field string) (*domainCampaign.Asset, error) {
	device, err := s.device(ctx)
	if err != nil {
		return nil, err
	}
	job, err := s.repo.GetScheduledSend(device, scheduleID)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, ErrCampaignNotFound
	}
	var assets map[string]scheduledAsset
	if json.Unmarshal([]byte(job.AssetsJSON), &assets) != nil {
		return nil, ErrCampaignNotFound
	}
	a, ok := assets[field]
	if !ok || a.Path == "" {
		return nil, ErrCampaignNotFound
	}
	root, err := filepath.Abs(filepath.Join(s.mediaRoot, "scheduled", job.ID))
	if err != nil {
		return nil, ErrCampaignNotFound
	}
	path, err := filepath.Abs(a.Path)
	if err != nil || !strings.HasPrefix(path, root+string(filepath.Separator)) {
		return nil, ErrCampaignNotFound
	}
	return &domainCampaign.Asset{Path: path, Filename: a.Filename, ContentType: scheduledAssetContentType(a)}, nil
}

// decodePreview turns a stored payload into a display model; undecodable
// payloads fall back to the raw text.
func decodePreview(job *domainChatStorage.ScheduledSend) domainCampaign.MessagePreview {
	p := domainCampaign.MessagePreview{Type: job.MessageType}
	var f struct {
		Message      string   `json:"message"`
		Caption      string   `json:"caption"`
		ImageURL     *string  `json:"image_url"`
		FileURL      *string  `json:"file_url"`
		VideoURL     *string  `json:"video_url"`
		AudioURL     *string  `json:"audio_url"`
		StickerURL   *string  `json:"sticker_url"`
		ViewOnce     bool     `json:"view_once"`
		PTT          bool     `json:"ptt"`
		Link         string   `json:"link"`
		Latitude     string   `json:"latitude"`
		Longitude    string   `json:"longitude"`
		ContactName  string   `json:"contact_name"`
		ContactPhone string   `json:"contact_phone"`
		Question     string   `json:"question"`
		Options      []string `json:"options"`
		MaxAnswer    int      `json:"max_answer"`
		MessageID    string   `json:"message_id"`
	}
	if err := json.Unmarshal([]byte(job.PayloadJSON), &f); err != nil {
		p.Raw = job.PayloadJSON
		return p
	}
	var assets map[string]scheduledAsset
	_ = json.Unmarshal([]byte(job.AssetsJSON), &assets)
	media := func(field string, url *string) *domainCampaign.Media {
		if a, ok := assets[field]; ok && a.Path != "" {
			return &domainCampaign.Media{Field: field, Filename: a.Filename, ContentType: scheduledAssetContentType(a), ScheduleID: job.ID}
		}
		if url != nil && *url != "" {
			return &domainCampaign.Media{Field: field, URL: *url}
		}
		return nil
	}
	p.Text, p.Caption, p.ViewOnce = f.Message, f.Caption, f.ViewOnce
	switch job.MessageType {
	case "image":
		p.Media = media("image", f.ImageURL)
	case "file":
		p.Media = media("file", f.FileURL)
	case "video":
		p.Media = media("video", f.VideoURL)
	case "audio":
		p.Media, p.VoiceNote = media("audio", f.AudioURL), f.PTT
	case "sticker":
		p.Media = media("sticker", f.StickerURL)
	case "link":
		p.Link = f.Link
	case "location":
		p.Latitude, p.Longitude = f.Latitude, f.Longitude
	case "contact":
		p.ContactName, p.ContactPhone = f.ContactName, f.ContactPhone
	case "poll":
		p.PollQuestion, p.PollOptions, p.PollMaxAnswer = f.Question, f.Options, f.MaxAnswer
	case "forward":
		p.ForwardMessageID = f.MessageID
	}
	return p
}
