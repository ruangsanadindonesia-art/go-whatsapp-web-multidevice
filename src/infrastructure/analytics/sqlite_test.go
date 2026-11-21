package analytics

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/chatstorage"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/sqlite"
)

func newDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open(sqlite.DriverName, filepath.Join(t.TempDir(), "chat.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := chatstorage.NewStorageRepository(db).InitializeSchema(); err != nil {
		t.Fatal(err)
	}
	return db
}

var day = time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)

func insert(t *testing.T, db *sql.DB, id, device, chat string, ts time.Time, fromMe bool) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO messages (id, chat_jid, device_id, sender, content, timestamp, is_from_me) VALUES (?,?,?,?,?,?,?)`,
		id, chat, device, chat, "x", ts, fromMe)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMessagesInRangeScopedToDevice(t *testing.T) {
	db := newDB(t)
	insert(t, db, "1", "dev1@s.whatsapp.net", "a@s.whatsapp.net", day.Add(time.Hour), false)
	insert(t, db, "2", "dev2@s.whatsapp.net", "a@s.whatsapp.net", day.Add(time.Hour), false)
	rows, err := NewSQLiteReader(db).MessagesInRange(context.Background(), "dev1@s.whatsapp.net", day, day.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ChatJID != "a@s.whatsapp.net" || rows[0].Context {
		t.Fatalf("got %+v", rows)
	}
}

func TestMessagesInRangeExcludesGroups(t *testing.T) {
	db := newDB(t)
	for i, chat := range []string{"g@g.us", "status@broadcast", "n@newsletter", "a@s.whatsapp.net", "b@lid"} {
		insert(t, db, string(rune('1'+i)), "d", chat, day.Add(time.Hour), false)
	}
	rows, err := NewSQLiteReader(db).MessagesInRange(context.Background(), "d", day, day.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("want only personal chats, got %+v", rows)
	}
}

func TestMessagesInRangeIncludesPriorContext(t *testing.T) {
	db := newDB(t)
	chat := "a@s.whatsapp.net"
	insert(t, db, "old", "d", chat, day.Add(-72*time.Hour), true)                  // far before: context via MAX()
	insert(t, db, "in1", "d", chat, day.Add(2*time.Hour), false)                   // in range
	insert(t, db, "late", "d", chat, day.Add(96*time.Hour), true)                  // far after: context via MIN()
	insert(t, db, "other", "d", "z@s.whatsapp.net", day.Add(-72*time.Hour), false) // no in-range rows: ignored
	rows, err := NewSQLiteReader(db).MessagesInRange(context.Background(), "d", day, day.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("want 3 rows (context, in-range, context), got %+v", rows)
	}
	if !rows[0].Context || !rows[0].IsFromMe || rows[1].Context || !rows[2].Context {
		t.Fatalf("bad context flags/order: %+v", rows)
	}
	if !rows[1].Timestamp.Equal(day.Add(2 * time.Hour)) {
		t.Fatalf("timestamp %v", rows[1].Timestamp)
	}
}

func TestMessagesInRangeSortedByChatThenTime(t *testing.T) {
	db := newDB(t)
	insert(t, db, "2", "d", "b@s.whatsapp.net", day.Add(3*time.Hour), false)
	insert(t, db, "1", "d", "a@s.whatsapp.net", day.Add(5*time.Hour), false)
	insert(t, db, "3", "d", "a@s.whatsapp.net", day.Add(1*time.Hour), true)
	rows, _ := NewSQLiteReader(db).MessagesInRange(context.Background(), "d", day, day.Add(24*time.Hour))
	if len(rows) != 3 || rows[0].ChatJID != "a@s.whatsapp.net" || !rows[0].IsFromMe || rows[2].ChatJID != "b@s.whatsapp.net" {
		t.Fatalf("got %+v", rows)
	}
}
