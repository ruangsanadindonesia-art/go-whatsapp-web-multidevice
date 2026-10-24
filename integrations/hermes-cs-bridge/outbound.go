package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const outboundMaxBody = 16 << 10

var outboundChatRe = regexp.MustCompile(`^[0-9]{5,20}@(s\.whatsapp\.net|lid)$`)

type outboundRequest struct {
	ChatID      string `json:"chat_id"`
	Text        string `json:"text"`
	ReferenceID int64  `json:"reference_id"`
	Purpose     string `json:"purpose"`
}

// outboundKey names an approved Dashboard action, not a WhatsApp recipient.
// Follow-up ids last seven days; confirmations/text retries last 24 hours.
func outboundKey(req outboundRequest) (string, time.Duration) {
	if req.Purpose == "followup" && req.ReferenceID > 0 {
		return "followup:" + intString(req.ReferenceID), outboundKeyWindow
	}
	digest := sha256.Sum256([]byte(req.Text))
	return req.Purpose + ":" + req.ChatID + ":" + hex.EncodeToString(digest[:8]), outboundTextWindow
}

func intString(v int64) string {
	return strconv.FormatInt(v, 10)
}

func (b *Bridge) handleOutbound(w http.ResponseWriter, r *http.Request) {
	// The endpoint is registered only when configured, and is outside the
	// public /cs-ai reverse proxy. The token must also never appear in logs.
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || subtle.ConstantTimeCompare([]byte(token), []byte(b.cfg.OutboundToken)) != 1 {
		jsonErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, outboundMaxBody+1))
	if err != nil || len(body) > outboundMaxBody {
		jsonErr(w, http.StatusBadRequest, "invalid: body melebihi 16 KB atau tidak bisa dibaca")
		return
	}
	var req outboundRequest
	if err := json.Unmarshal(body, &req); err != nil {
		jsonErr(w, http.StatusBadRequest, "invalid: JSON rusak atau tipe field tidak valid")
		return
	}
	req.ChatID = strings.TrimSpace(req.ChatID)
	req.Text = strings.TrimSpace(req.Text)
	if !outboundChatRe.MatchString(req.ChatID) || ignoredChat(req.ChatID) {
		jsonErr(w, http.StatusBadRequest, "invalid: chat_id harus JID lead personal")
		return
	}
	if isAdmin(b.settings.Get(), normalizePhone(req.ChatID)) {
		jsonErr(w, http.StatusBadRequest, "invalid: chat admin")
		return
	}
	if req.Purpose != "followup" && req.Purpose != "payment_verified" {
		jsonErr(w, http.StatusBadRequest, "invalid: purpose tidak dikenal")
		return
	}
	if req.ReferenceID < 0 || (req.Purpose == "payment_verified" && req.ReferenceID != 0) {
		jsonErr(w, http.StatusBadRequest, "invalid: reference_id")
		return
	}
	n := len([]rune(req.Text))
	if n < 1 || n > 1000 {
		jsonErr(w, http.StatusBadRequest, "invalid: text harus 1-1000 karakter")
		return
	}
	if strings.Contains(req.Text, "[[") {
		jsonErr(w, http.StatusBadRequest, "marker")
		return
	}
	// This is an extra anti-broadcast guard: even a token holder cannot send
	// to arbitrary strangers who never contacted this CS WhatsApp number.
	if !b.store.HasSeenLeadChat(req.ChatID) {
		jsonErr(w, http.StatusBadRequest, "invalid: chat belum pernah masuk")
		return
	}
	now := b.now()
	key, expiry := outboundKey(req)
	if rec, found := b.store.FindOutbound(key, now); found {
		jsonOut(w, http.StatusOK, map[string]any{"wa_message_id": rec.MessageID, "duplicate": true})
		return
	}
	if _, paused := b.store.PausedUntil(req.ChatID, now); paused {
		jsonErr(w, http.StatusConflict, "paused")
		return
	}
	// Keep the check for pending/debounce and the TryLock of c.work within
	// b.mu. Otherwise a new incoming message can race the outbound sender.
	b.mu.Lock()
	c := b.chats[req.ChatID]
	if c == nil {
		c = &chat{}
		b.chats[req.ChatID] = c
	}
	busy := len(c.pending) > 0 || c.timer != nil ||
		(!c.lastInbound.IsZero() && now.Sub(c.lastInbound) < b.cfg.OutboundQuietAfterInbound)
	locked := false
	if !busy {
		locked = c.work.TryLock()
	}
	b.mu.Unlock()
	if busy || !locked {
		jsonErr(w, http.StatusConflict, "busy")
		return
	}
	defer c.work.Unlock()

	// Recheck after the per-chat lock: an earlier concurrent send may have
	// completed and saved its idempotency key while this request waited.
	if rec, found := b.store.FindOutbound(key, b.now()); found {
		jsonOut(w, http.StatusOK, map[string]any{"wa_message_id": rec.MessageID, "duplicate": true})
		return
	}
	if _, paused := b.store.PausedUntil(req.ChatID, b.now()); paused {
		jsonErr(w, http.StatusConflict, "paused")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	id, err := b.gowa.SendText(ctx, req.ChatID, req.Text)
	if err != nil || strings.TrimSpace(id) == "" {
		b.log.Warn("outbound send failed", "chat", req.ChatID, "purpose", req.Purpose, "reference_id", req.ReferenceID)
		jsonErr(w, http.StatusBadGateway, "send failed")
		return
	}
	b.rememberSent(id)
	at := b.now()
	if err := b.store.RecordOutbound(key, req.ChatID, id, req.Text, req.Purpose, at, expiry); err != nil {
		// A send might have succeeded without a durable state write; do not
		// pretend this is a successful, idempotent delivery.
		b.log.Error("outbound sent but persistence failed", "chat", req.ChatID, "purpose", req.Purpose, "reference_id", req.ReferenceID, "wa_message_id", id, "err", err)
		jsonErr(w, http.StatusBadGateway, "send failed")
		return
	}
	ref := b.leadFor(req.ChatID)
	b.emit(ref, "message.out", eventIDFor("out", id), at, map[string]any{
		"wa_message_id": id,
		"body": req.Text,
		"sender": "followup",
		"reference_id": req.ReferenceID,
		"purpose": req.Purpose,
	})
	b.log.Info("outbound sent", "chat", req.ChatID, "purpose", req.Purpose, "reference_id", req.ReferenceID, "wa_message_id", id)
	jsonOut(w, http.StatusOK, map[string]string{"wa_message_id": id})
}
