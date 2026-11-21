// Package analytics reads chat storage for the panel's CS analytics.
// Read-only; it shares the chat storage *sql.DB.
package analytics

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	domainAnalytics "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/analytics"
)

// slack widens SQL bounds: stored timestamps are text in whatever offset the
// writer used, so string comparison is only trusted to within a day. Exact
// in-range classification happens in Go after parsing.
const slack = 24 * time.Hour

// personal chats only: no groups, broadcasts, status or newsletters.
const personal = `(chat_jid LIKE '%@s.whatsapp.net' OR chat_jid LIKE '%@lid')`

type SQLiteReader struct{ db *sql.DB }

func NewSQLiteReader(db *sql.DB) *SQLiteReader { return &SQLiteReader{db: db} }

var _ domainAnalytics.Reader = (*SQLiteReader)(nil)

// MessagesInRange returns the device's personal-chat messages in [from, to),
// plus context rows (flag Context): every message within a day of the range
// and, per chat that has in-range traffic, the last message before and the
// first after that window. Sorted by chat, then time.
func (r *SQLiteReader) MessagesInRange(ctx context.Context, deviceID string, from, to time.Time) ([]domainAnalytics.MessageRow, error) {
	lo, hi := from.Add(-slack), to.Add(slack)
	inWindow := `device_id = ? AND ` + personal + ` AND timestamp >= ? AND timestamp < ?`
	chats := `chat_jid IN (SELECT DISTINCT chat_jid FROM messages WHERE ` + inWindow + `)`
	// SQLite returns the bare columns of the row holding MAX()/MIN().
	query := `
		SELECT chat_jid, timestamp, is_from_me FROM messages WHERE ` + inWindow + `
		UNION ALL
		SELECT chat_jid, MAX(timestamp), is_from_me FROM messages
			WHERE device_id = ? AND timestamp < ? AND ` + chats + ` GROUP BY chat_jid
		UNION ALL
		SELECT chat_jid, MIN(timestamp), is_from_me FROM messages
			WHERE device_id = ? AND timestamp >= ? AND ` + chats + ` GROUP BY chat_jid`
	args := []any{
		deviceID, lo, hi,
		deviceID, lo, deviceID, lo, hi,
		deviceID, hi, deviceID, lo, hi,
	}
	rs, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("analytics query: %w", err)
	}
	defer rs.Close()

	var out []domainAnalytics.MessageRow
	for rs.Next() {
		var (
			chat   string
			rawTS  any
			fromMe sql.NullBool
		)
		if err := rs.Scan(&chat, &rawTS, &fromMe); err != nil {
			return nil, fmt.Errorf("analytics scan: %w", err)
		}
		ts, err := parseTimestamp(rawTS)
		if err != nil {
			return nil, err
		}
		out = append(out, domainAnalytics.MessageRow{
			ChatJID:   chat,
			Timestamp: ts,
			IsFromMe:  fromMe.Bool,
			Context:   ts.Before(from) || !ts.Before(to),
		})
	}
	if err := rs.Err(); err != nil {
		return nil, fmt.Errorf("analytics rows: %w", err)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ChatJID != out[j].ChatJID {
			return out[i].ChatJID < out[j].ChatJID
		}
		return out[i].Timestamp.Before(out[j].Timestamp)
	})
	return out, nil
}

// Layouts the SQLite drivers use for TIMESTAMP text (mattn/go-sqlite3 and
// modernc.org/sqlite). Aggregates (MAX/MIN) come back as text, not time.Time.
var tsLayouts = []string{
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02T15:04:05.999999999-07:00",
	"2006-01-02 15:04:05.999999999 -0700 MST",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02T15:04:05.999999999",
	"2006-01-02 15:04:05",
	time.RFC3339Nano,
}

func parseTimestamp(v any) (time.Time, error) {
	switch t := v.(type) {
	case time.Time:
		return t, nil
	case string:
		return parseTSString(t)
	case []byte:
		return parseTSString(string(t))
	case int64:
		return time.Unix(t, 0), nil
	}
	return time.Time{}, fmt.Errorf("analytics: unsupported timestamp %T", v)
}

func parseTSString(s string) (time.Time, error) {
	for _, layout := range tsLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("analytics: unparseable timestamp %q", s)
}

// ChatNames returns stored non-empty chat names for jids on deviceID.
func (r *SQLiteReader) ChatNames(ctx context.Context, deviceID string, jids []string) (map[string]string, error) {
	out := map[string]string{}
	const batch = 500 // stay under SQLite's bound-parameter limit
	for start := 0; start < len(jids); start += batch {
		part := jids[start:min(start+batch, len(jids))]
		args := make([]any, 0, len(part)+1)
		args = append(args, deviceID)
		marks := make([]string, len(part))
		for i, j := range part {
			marks[i] = "?"
			args = append(args, j)
		}
		rs, err := r.db.QueryContext(ctx,
			`SELECT jid, name FROM chats WHERE device_id = ? AND name <> '' AND jid IN (`+strings.Join(marks, ",")+`)`, args...)
		if err != nil {
			return nil, fmt.Errorf("analytics chat names: %w", err)
		}
		for rs.Next() {
			var jid, name string
			if err := rs.Scan(&jid, &name); err != nil {
				rs.Close()
				return nil, fmt.Errorf("analytics chat names scan: %w", err)
			}
			out[jid] = name
		}
		err = rs.Err()
		rs.Close()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
