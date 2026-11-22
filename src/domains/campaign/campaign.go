// Package campaign holds DTOs for the panel's schedule menu. A campaign is a
// read-time grouping of scheduled_sends rows (one per recipient) that share
// device, message type, payload (minus recipient) and time pattern.
package campaign

import (
	"context"
	"time"
)

// Derived campaign statuses.
const (
	StatusActive    = "aktif"
	StatusPaused    = "dijeda"
	StatusDone      = "selesai"
	StatusFailed    = "gagal"
	StatusCancelled = "dibatalkan"
)

type Campaign struct {
	Key             string         `json:"key"`
	MessageType     string         `json:"message_type"`
	Summary         string         `json:"summary"`
	Status          string         `json:"status"`
	Recurrence      string         `json:"recurrence"`
	Weekdays        []int          `json:"weekdays,omitempty"`
	DayOfMonth      int            `json:"day_of_month,omitempty"`
	Timezone        string         `json:"timezone"`
	ScheduledAt     time.Time      `json:"scheduled_at"`
	NextRunAt       *time.Time     `json:"next_run_at,omitempty"`
	EndAt           *time.Time     `json:"end_at,omitempty"`
	OccurrenceLimit int            `json:"occurrence_limit,omitempty"`
	Recipients      int            `json:"recipients"`
	StatusCounts    map[string]int `json:"status_counts"`
	Sent            int            `json:"sent"`   // logged sent runs, all time
	Failed          int            `json:"failed"` // logged failed runs, all time
	LastRunAt       *time.Time     `json:"last_run_at,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
}

type Recipient struct {
	ScheduleID      string     `json:"schedule_id"`
	Phone           string     `json:"phone"`
	Name            string     `json:"name"`
	Status          string     `json:"status"` // raw schedule status
	OccurrenceCount int        `json:"occurrence_count"`
	NextRunAt       *time.Time `json:"next_run_at,omitempty"`
	LastRunAt       *time.Time `json:"last_run_at,omitempty"`
	LastError       string     `json:"last_error,omitempty"`
}

type Run struct {
	ScheduleID   string    `json:"schedule_id"`
	Phone        string    `json:"phone"`
	CampaignKey  string    `json:"campaign_key,omitempty"`
	Summary      string    `json:"summary,omitempty"`
	OccurrenceAt time.Time `json:"occurrence_at"`
	ExecutedAt   time.Time `json:"executed_at"`
	Status       string    `json:"status"`
	MessageID    string    `json:"message_id,omitempty"`
	Error        string    `json:"error,omitempty"`
}

type Media struct {
	Field       string `json:"field"`
	Filename    string `json:"filename,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	URL         string `json:"url,omitempty"`         // external URL given at schedule time
	ScheduleID  string `json:"schedule_id,omitempty"` // stored upload, served by the asset endpoint
}

// MessagePreview is the scheduled message decoded for display.
type MessagePreview struct {
	Type             string   `json:"type"`
	Text             string   `json:"text,omitempty"`
	Caption          string   `json:"caption,omitempty"`
	Media            *Media   `json:"media,omitempty"`
	ViewOnce         bool     `json:"view_once,omitempty"`
	VoiceNote        bool     `json:"voice_note,omitempty"`
	Link             string   `json:"link,omitempty"`
	Latitude         string   `json:"latitude,omitempty"`
	Longitude        string   `json:"longitude,omitempty"`
	ContactName      string   `json:"contact_name,omitempty"`
	ContactPhone     string   `json:"contact_phone,omitempty"`
	PollQuestion     string   `json:"poll_question,omitempty"`
	PollOptions      []string `json:"poll_options,omitempty"`
	PollMaxAnswer    int      `json:"poll_max_answer,omitempty"`
	ForwardMessageID string   `json:"forward_message_id,omitempty"`
	Raw              string   `json:"raw,omitempty"` // set when the payload could not be decoded
}

type Detail struct {
	Campaign
	Message       MessagePreview `json:"message"`
	RecipientList []Recipient    `json:"recipient_list"`
	Runs          []Run          `json:"runs"`
}

type Totals struct {
	Sent            int     `json:"sent"`
	Failed          int     `json:"failed"`
	SuccessRate     float64 `json:"success_rate"` // 0..1; 0 when nothing ran
	Recipients      int     `json:"recipients"`   // unique phones reached (sent)
	Campaigns       int     `json:"campaigns"`    // campaigns with runs in range
	ActiveCampaigns int     `json:"active_campaigns"`
	Upcoming        int     `json:"upcoming"` // projected sends from now to the range end
}

type DayPoint struct {
	Day      string `json:"day"` // YYYY-MM-DD
	Sent     int    `json:"sent"`
	Failed   int    `json:"failed"`
	Upcoming int    `json:"upcoming"`
}

type TypeCount struct {
	Type   string `json:"type"`
	Sent   int    `json:"sent"`
	Failed int    `json:"failed"`
}

type CampaignStat struct {
	Key         string `json:"key"`
	Summary     string `json:"summary"`
	MessageType string `json:"message_type"`
	Status      string `json:"status"`
	Sent        int    `json:"sent"`
	Failed      int    `json:"failed"`
}

type Dashboard struct {
	From           string         `json:"from"`
	To             string         `json:"to"`
	Approximate    bool           `json:"approximate"`
	LogSince       *time.Time     `json:"log_since,omitempty"`
	Totals         Totals         `json:"totals"`
	Daily          []DayPoint     `json:"daily"`
	ByType         []TypeCount    `json:"by_type"`
	TopCampaigns   []CampaignStat `json:"top_campaigns"`
	RecentFailures []Run          `json:"recent_failures"`
}

// CalendarEvent is one campaign occurrence: logged (sent/failed) or projected
// (upcoming/paused).
type CalendarEvent struct {
	Key         string    `json:"key"`
	Summary     string    `json:"summary"`
	MessageType string    `json:"message_type"`
	At          time.Time `json:"at"`
	Kind        string    `json:"kind"` // sent | failed | partial | upcoming | paused
	Recipients  int       `json:"recipients"`
	Sent        int       `json:"sent"`
	Failed      int       `json:"failed"`
}

type ListFilter struct {
	From, To time.Time // campaigns with a run or an occurrence in range; zero = all
	Status   string
	Query    string
}

type ActionResult struct {
	Changed int `json:"changed"`
	Skipped int `json:"skipped"`
}

type Asset struct {
	Path        string
	Filename    string
	ContentType string
}

type ICampaignUsecase interface {
	List(ctx context.Context, filter ListFilter) ([]Campaign, error)
	Get(ctx context.Context, key string) (*Detail, error)
	Dashboard(ctx context.Context, from, to time.Time) (Dashboard, error)
	Calendar(ctx context.Context, from, to time.Time) ([]CalendarEvent, error)
	Action(ctx context.Context, key, action string) (ActionResult, error)
	Asset(ctx context.Context, scheduleID, field string) (*Asset, error)
	Location() *time.Location
}
