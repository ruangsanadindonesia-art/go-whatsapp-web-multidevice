package chatstorage

import "time"

// ScheduledSend is the durable worker record. PayloadJSON contains the
// normalized send request; AssetsJSON contains managed upload metadata.
// ScheduledSendRun is one final outcome of a scheduled occurrence: sent, or
// failed after its retries. Retries and offline waits are not logged.
type ScheduledSendRun struct {
	ID           int64
	ScheduleID   string
	DeviceID     string
	OccurrenceAt time.Time
	ExecutedAt   time.Time
	Status       string // "sent" or "failed"
	MessageID    string
	Error        string
}

// ScheduledSendRunFilter selects runs of one device; zero From/To are open.
type ScheduledSendRunFilter struct {
	DeviceID    string
	ScheduleIDs []string
	From, To    time.Time // executed_at in [From, To)
	Limit       int
}

type ScheduledSend struct {
	ID              string
	DeviceID        string
	MessageType     string
	PayloadJSON     string
	AssetsJSON      string
	Phone           string
	Summary         string
	ScheduledAt     time.Time
	NextRunAt       time.Time
	Timezone        string
	Recurrence      string
	WeekdaysJSON    string
	DayOfMonth      int
	EndAt           *time.Time
	OccurrenceLimit int
	OccurrenceCount int
	Attempts        int
	Status          string
	LeaseToken      string
	LeaseUntil      *time.Time
	LastRunAt       *time.Time
	LastMessageID   string
	LastError       string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
