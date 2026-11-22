package chatstorage

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
)

// Run-log timestamps are always written in UTC so text comparison in SQL is
// chronological.

func (r *SQLiteRepository) RecordScheduledSendRun(run *domainChatStorage.ScheduledSendRun) error {
	res, err := r.db.Exec(`INSERT INTO scheduled_send_runs (schedule_id, device_id, occurrence_at, executed_at, status, message_id, error)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		run.ScheduleID, run.DeviceID, run.OccurrenceAt.UTC(), run.ExecutedAt.UTC(), run.Status, run.MessageID, run.Error)
	if err != nil {
		return fmt.Errorf("record scheduled send run: %w", err)
	}
	if id, err := res.LastInsertId(); err == nil {
		run.ID = id
	}
	return nil
}

func (r *SQLiteRepository) ListScheduledSendRuns(filter domainChatStorage.ScheduledSendRunFilter) ([]*domainChatStorage.ScheduledSendRun, error) {
	conds := []string{"device_id = ?"}
	args := []any{filter.DeviceID}
	if !filter.From.IsZero() {
		conds = append(conds, "executed_at >= ?")
		args = append(args, filter.From.UTC())
	}
	if !filter.To.IsZero() {
		conds = append(conds, "executed_at < ?")
		args = append(args, filter.To.UTC())
	}
	if len(filter.ScheduleIDs) > 0 {
		marks := make([]string, len(filter.ScheduleIDs))
		for i, id := range filter.ScheduleIDs {
			marks[i] = "?"
			args = append(args, id)
		}
		conds = append(conds, "schedule_id IN ("+strings.Join(marks, ",")+")")
	}
	query := `SELECT id, schedule_id, device_id, occurrence_at, executed_at, status, message_id, error
		FROM scheduled_send_runs WHERE ` + strings.Join(conds, " AND ") + ` ORDER BY executed_at, id`
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list scheduled send runs: %w", err)
	}
	defer rows.Close()
	var out []*domainChatStorage.ScheduledSendRun
	for rows.Next() {
		run := &domainChatStorage.ScheduledSendRun{}
		if err := rows.Scan(&run.ID, &run.ScheduleID, &run.DeviceID, &run.OccurrenceAt, &run.ExecutedAt, &run.Status, &run.MessageID, &run.Error); err != nil {
			return nil, fmt.Errorf("scan scheduled send run: %w", err)
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

func (r *SQLiteRepository) FirstScheduledSendRunAt(deviceID string) (*time.Time, error) {
	var first sql.NullString
	if err := r.db.QueryRow(`SELECT MIN(executed_at) FROM scheduled_send_runs WHERE device_id = ?`, deviceID).Scan(&first); err != nil {
		return nil, fmt.Errorf("first scheduled send run: %w", err)
	}
	if !first.Valid || first.String == "" {
		return nil, nil
	}
	for _, layout := range []string{"2006-01-02 15:04:05.999999999-07:00", "2006-01-02T15:04:05.999999999-07:00", "2006-01-02 15:04:05.999999999 -0700 MST", time.RFC3339Nano, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, first.String); err == nil {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("first scheduled send run: unparseable time %q", first.String)
}
