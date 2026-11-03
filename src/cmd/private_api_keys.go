package cmd

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/sqlite"
)

// Private integration keys are deliberately separate from Basic Auth and MCP
// OAuth. A Bearer key is valid ONLY for the explicitly registered /api/v1
// routes, never for legacy REST, UI, MCP, or the CS Bridge.
type privateAPIKey struct {
	ID         string
	Name       string
	Prefix     string
	DeviceID   string
	CanGroup   bool
	CanRESTTools bool
	CanMCPTools bool
	CreatedAt  int64
	ExpiresAt  int64
	RevokedAt  sql.NullInt64
}

func (k privateAPIKey) ExpiryLabel() string { return time.Unix(k.ExpiresAt,0).UTC().Format("02 Jan 2006") }

type privateAPIToolGrants struct {
	REST bool
	MCP bool
}

type privateAPISendRecord struct {
	ID          string
	KeyID       string
	Fingerprint string
	State       string
	MessageID   string
}

type privateAPIRateWindow struct {
	Started time.Time
	Count   int
}

type privateAPIStore struct {
	db *sql.DB
	mu sync.Mutex
	rate map[string]privateAPIRateWindow
}

var privateAPIIdempotencyPattern = regexp.MustCompile("^[a-zA-Z0-9:_-]{8,128}$")

func openPrivateAPIStore(file string) (*privateAPIStore, error) {
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open(sqlite.DriverName, sqlite.FormatChatStorageURI("file:"+filepath.ToSlash(file), true, true))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	statements := []string{
		"CREATE TABLE IF NOT EXISTS private_api_keys (" +
			"id TEXT PRIMARY KEY, name TEXT NOT NULL, prefix TEXT NOT NULL UNIQUE," +
			"token_hash TEXT NOT NULL UNIQUE, device_id TEXT NOT NULL, can_send_group INTEGER NOT NULL DEFAULT 0," +
			"created_at INTEGER NOT NULL, expires_at INTEGER NOT NULL, revoked_at INTEGER)",
		"CREATE TABLE IF NOT EXISTS private_api_requests (" +
			"id TEXT PRIMARY KEY, key_id TEXT NOT NULL, idempotency_key TEXT NOT NULL," +
			"fingerprint TEXT NOT NULL, state TEXT NOT NULL, message_id TEXT NOT NULL DEFAULT ''," +
			"created_at INTEGER NOT NULL, UNIQUE(key_id, idempotency_key)," +
			"FOREIGN KEY(key_id) REFERENCES private_api_keys(id))",
		"CREATE INDEX IF NOT EXISTS idx_private_api_requests_owner ON private_api_requests(key_id, id)",
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	// Additive upgrade. Existing keys do not inherit additional tool access.
	for _, column := range []string{"can_rest_tools", "can_mcp_tools"} {
		rows, err := db.Query("PRAGMA table_info(private_api_keys)")
		if err != nil { _ = db.Close(); return nil, err }
		found := false
		for rows.Next() {
			var cid, notNull, pk int
			var name, colType string
			var defaultValue sql.NullString
			if err = rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &pk); err != nil { break }
			if name == column { found = true }
		}
		rowsErr := rows.Err()
		_ = rows.Close()
		if err != nil { _ = db.Close(); return nil, err }
		if rowsErr != nil { _ = db.Close(); return nil, rowsErr }
		if !found {
			if _, err := db.Exec("ALTER TABLE private_api_keys ADD COLUMN " + column + " INTEGER NOT NULL DEFAULT 0"); err != nil {
				_ = db.Close(); return nil, err
			}
		}
	}
	return &privateAPIStore{db: db, rate: make(map[string]privateAPIRateWindow)}, nil
}

func (s *privateAPIStore) Close() error { return s.db.Close() }

func defaultPrivateAPIStorePath() string {
	return filepath.Join(config.PathStorages, "private-api-keys.db")
}

func privateAPIRandom(bytesCount int) (string, error) {
	value := make([]byte, bytesCount)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func privateAPITokenHash(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func (s *privateAPIStore) Create(ctx context.Context, name, canonicalDevice string, group bool, permissions ...privateAPIToolGrants) (privateAPIKey, string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 80 || strings.ContainsAny(name, "\r\n\x00") {
		return privateAPIKey{}, "", errors.New("nama integrasi harus 1 sampai 80 karakter")
	}
	if canonicalDevice == "" || len(canonicalDevice) > 255 {
		return privateAPIKey{}, "", errors.New("device_id tidak valid")
	}
	id, err := privateAPIRandom(16)
	if err != nil { return privateAPIKey{}, "", err }
	prefixBytes := make([]byte, 6)
	if _, err := rand.Read(prefixBytes); err != nil { return privateAPIKey{}, "", err }
	prefix := hex.EncodeToString(prefixBytes)
	secret, err := privateAPIRandom(32)
	if err != nil { return privateAPIKey{}, "", err }
	token := "gwa_" + prefix + "_" + secret
	now := time.Now().UTC()
	key := privateAPIKey{
		ID: id, Name: name, Prefix: prefix, DeviceID: canonicalDevice,
		CanGroup: group, CreatedAt: now.Unix(), ExpiresAt: now.Add(90*24*time.Hour).Unix(),
	}
	groupInt := 0
	if group { groupInt = 1 }
	grants := privateAPIToolGrants{}
	if len(permissions) > 0 { grants = permissions[0] }
	key.CanRESTTools, key.CanMCPTools = grants.REST, grants.MCP
	restInt, mcpInt := 0, 0
	if grants.REST { restInt = 1 }
	if grants.MCP { mcpInt = 1 }
	_, err = s.db.ExecContext(ctx,
		"INSERT INTO private_api_keys (id,name,prefix,token_hash,device_id,can_send_group,created_at,expires_at,can_rest_tools,can_mcp_tools) VALUES (?,?,?,?,?,?,?,?,?,?)",
		key.ID, key.Name, key.Prefix, privateAPITokenHash(token), key.DeviceID, groupInt, key.CreatedAt, key.ExpiresAt,restInt,mcpInt)
	if err != nil { return privateAPIKey{}, "", fmt.Errorf("gagal menyimpan API key: %w", err) }
	return key, token, nil
}

func (s *privateAPIStore) List(ctx context.Context) ([]privateAPIKey, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id,name,prefix,device_id,can_send_group,created_at,expires_at,revoked_at,can_rest_tools,can_mcp_tools FROM private_api_keys ORDER BY created_at DESC, name LIMIT 250")
	if err != nil { return nil, err }
	defer rows.Close()
	keys := []privateAPIKey{}
	for rows.Next() {
		var key privateAPIKey
		var group, restAllowed, mcpAllowed int
		if err := rows.Scan(&key.ID,&key.Name,&key.Prefix,&key.DeviceID,&group,&key.CreatedAt,&key.ExpiresAt,&key.RevokedAt,&restAllowed,&mcpAllowed); err != nil {
			return nil, err
		}
		key.CanGroup = group != 0
		key.CanRESTTools, key.CanMCPTools = restAllowed != 0, mcpAllowed != 0
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func (s *privateAPIStore) Revoke(ctx context.Context, id string) (bool, error) {
	result, err := s.db.ExecContext(ctx,
		"UPDATE private_api_keys SET revoked_at=? WHERE id=? AND revoked_at IS NULL",
		time.Now().UTC().Unix(), id)
	if err != nil { return false, err }
	count, err := result.RowsAffected()
	return count > 0, err
}

func (s *privateAPIStore) Authenticate(ctx context.Context, raw string) (privateAPIKey, bool, error) {
	parts := strings.SplitN(raw, "_", 3)
	if len(parts) != 3 || parts[0] != "gwa" || len(parts[1]) != 12 || len(parts[2]) != 43 {
		return privateAPIKey{}, false, nil
	}
	var key privateAPIKey
	var hash string
	var group, restAllowed, mcpAllowed int
	err := s.db.QueryRowContext(ctx,
		"SELECT id,name,prefix,token_hash,device_id,can_send_group,created_at,expires_at,revoked_at,can_rest_tools,can_mcp_tools FROM private_api_keys WHERE prefix=?",
		parts[1]).Scan(&key.ID,&key.Name,&key.Prefix,&hash,&key.DeviceID,&group,&key.CreatedAt,&key.ExpiresAt,&key.RevokedAt,&restAllowed,&mcpAllowed)
	if errors.Is(err,sql.ErrNoRows) { return privateAPIKey{}, false, nil }
	if err != nil { return privateAPIKey{}, false, err }
	provided := privateAPITokenHash(raw)
	if subtle.ConstantTimeCompare([]byte(provided),[]byte(hash)) != 1 ||
		key.RevokedAt.Valid || time.Now().UTC().Unix() >= key.ExpiresAt {
		return privateAPIKey{}, false, nil
	}
	key.CanGroup = group != 0
	key.CanRESTTools, key.CanMCPTools = restAllowed != 0, mcpAllowed != 0
	return key, true, nil
}

// In-process abuse cap. Independent instances need a shared limiter before
// horizontal scaling; this first version runs in one GoWA REST process.
func (s *privateAPIStore) AllowSend(id string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.rate) > 1500 {
		for id, slot := range s.rate {
			if now.Sub(slot.Started) >= time.Minute {
				delete(s.rate,id)
			}
		}
	}
	slot := s.rate[id]
	if now.Sub(slot.Started) >= time.Minute {
		slot = privateAPIRateWindow{Started: now}
	}
	if slot.Count >= 30 { return false }
	slot.Count++
	s.rate[id] = slot
	return true
}

// Reserve before making a WhatsApp call. A duplicate never triggers another
// send, including after crashes or timeouts. A stranded PENDING/UNCERTAIN
// request must be reconciled manually, not automatically replayed.
func (s *privateAPIStore) ReserveSend(ctx context.Context, keyID, idem, fingerprint string) (privateAPISendRecord, bool, error) {
	if !privateAPIIdempotencyPattern.MatchString(idem) { return privateAPISendRecord{}, false, errors.New("invalid Idempotency-Key") }
	id, err := privateAPIRandom(16)
	if err != nil { return privateAPISendRecord{}, false, err }
	result, err := s.db.ExecContext(ctx,
		"INSERT OR IGNORE INTO private_api_requests(id,key_id,idempotency_key,fingerprint,state,created_at) VALUES(?,?,?,?,?,?)",
		id, keyID, idem, fingerprint, "PENDING", time.Now().UTC().Unix())
	if err != nil { return privateAPISendRecord{}, false, err }
	n, err := result.RowsAffected()
	if err != nil { return privateAPISendRecord{}, false, err }
	var record privateAPISendRecord
	err = s.db.QueryRowContext(ctx,
		"SELECT id,key_id,fingerprint,state,message_id FROM private_api_requests WHERE key_id=? AND idempotency_key=?",keyID,idem).
		Scan(&record.ID,&record.KeyID,&record.Fingerprint,&record.State,&record.MessageID)
	if err != nil { return privateAPISendRecord{}, false, err }
	return record, n == 1, nil
}

func (s *privateAPIStore) FinishSend(ctx context.Context, id, state, messageID string) error {
	if state != "SENT" && state != "UNCERTAIN" { return errors.New("invalid result state") }
	_, err := s.db.ExecContext(ctx,
		"UPDATE private_api_requests SET state=?, message_id=? WHERE id=? AND state='PENDING'",state,messageID,id)
	return err
}

func (s *privateAPIStore) GetSend(ctx context.Context, keyID, id string) (privateAPISendRecord, bool, error) {
	var record privateAPISendRecord
	err := s.db.QueryRowContext(ctx,
		"SELECT id,key_id,fingerprint,state,message_id FROM private_api_requests WHERE id=? AND key_id=?",id,keyID).
		Scan(&record.ID,&record.KeyID,&record.Fingerprint,&record.State,&record.MessageID)
	if errors.Is(err,sql.ErrNoRows) { return privateAPISendRecord{}, false, nil }
	if err != nil { return privateAPISendRecord{}, false, err }
	return record, true, nil
}
