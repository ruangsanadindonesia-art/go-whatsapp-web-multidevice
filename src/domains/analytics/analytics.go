// Package analytics holds DTOs and interfaces for the Ruang Sanad panel's
// CS chat analytics (/api/app/analytics/*).
package analytics

import (
	"context"
	"encoding/json"
	"time"
)

// MessageRow is the minimal message data analytics needs. Context rows lie
// just outside the requested range (the last message before it and the
// first after it, per chat): they decide turn boundaries but are not counted.
type MessageRow struct {
	ChatJID   string
	Timestamp time.Time
	IsFromMe  bool
	Context   bool
}

// Turn is one customer turn: consecutive inbound messages and the first
// reply after them. Response counts only service-hours time.
type Turn struct {
	ChatJID   string
	AskedAt   time.Time
	RepliedAt *time.Time
	Response  time.Duration
}

// UnansweredChat is a chat whose latest customer turn has no reply yet.
type UnansweredChat struct {
	ChatJID        string        `json:"chat_jid"`
	Name           string        `json:"name"`
	WaitingSince   time.Time     `json:"waiting_since"`
	WaitingService time.Duration `json:"-"`
}

func (u UnansweredChat) MarshalJSON() ([]byte, error) {
	type alias UnansweredChat
	return json.Marshal(struct {
		alias
		WaitingServiceS int64 `json:"waiting_service_s"`
	}{alias(u), int64(u.WaitingService / time.Second)})
}

// Summary aggregates a range. Durations are serialised in whole seconds.
type Summary struct {
	Inbound        int              `json:"inbound"`
	Outbound       int              `json:"outbound"`
	Turns          int              `json:"turns"`
	Answered       int              `json:"answered"`
	MedianResponse time.Duration    `json:"-"`
	P90Response    time.Duration    `json:"-"`
	Unanswered     []UnansweredChat `json:"unanswered"`
}

func (s Summary) MarshalJSON() ([]byte, error) {
	type alias Summary
	return json.Marshal(struct {
		alias
		MedianResponseS int64 `json:"median_response_s"`
		P90ResponseS    int64 `json:"p90_response_s"`
	}{alias(s), int64(s.MedianResponse / time.Second), int64(s.P90Response / time.Second)})
}

// HourBucket counts messages per day and hour in the service time zone.
type HourBucket struct {
	Day      string `json:"day"` // YYYY-MM-DD
	Hour     int    `json:"hour"`
	Inbound  int    `json:"inbound"`
	Outbound int    `json:"outbound"`
}

// RangeRequest is a half-open time range [From, To).
type RangeRequest struct {
	From, To time.Time
}

// Reader reads chat storage for one device (storage device id).
type Reader interface {
	MessagesInRange(ctx context.Context, deviceID string, from, to time.Time) ([]MessageRow, error)
	// ChatNames maps chat JIDs to stored display names (missing = no name).
	ChatNames(ctx context.Context, deviceID string, jids []string) (map[string]string, error)
}

type IAnalyticsUsecase interface {
	Summary(ctx context.Context, req RangeRequest) (Summary, error)
	Volume(ctx context.Context, req RangeRequest) ([]HourBucket, error)
	ServiceHours() string
	Location() *time.Location
}
