package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Pause marks a chat where the AI must stay silent (admin took over).
type Pause struct {
	Until  time.Time `json:"until"`
	Reason string    `json:"reason"`
}

// OutboundRecord remembers successful outbound sends across bridge restarts.
type OutboundRecord struct {
	MessageID string    `json:"wa_message_id"`
	At        time.Time `json:"at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// OutboundNote is trusted staff-authored text supplied as context, not instructions.
type OutboundNote struct {
	Text    string    `json:"text"`
	Purpose string    `json:"purpose"`
	At      time.Time `json:"at"`
}

const outboundNoteWindow = 30 * 24 * time.Hour
const outboundKeyWindow = 7 * 24 * time.Hour
const outboundTextWindow = 24 * time.Hour

// Store keeps the state that must survive a restart: paused chats and the
// phone-number → chat-id map used by admin commands.
type Store struct {
	mu     sync.Mutex
	path   string
	Paused map[string]Pause  `json:"paused"`
	Phones map[string]string `json:"phones"`
	// PaymentSent records when payment info was last sent to a chat, so a later
	// image from that chat is treated as a likely transfer proof.
	PaymentSent map[string]time.Time `json:"payment_sent"`
	Outbound     map[string]OutboundRecord `json:"outbound"`
	LastOutbound map[string]OutboundNote   `json:"last_outbound"`
	SeenChats    map[string]time.Time     `json:"seen_chats"`
}

func LoadStore(path string) (*Store, error) {
	s := &Store{path: path, Paused: map[string]Pause{}, Phones: map[string]string{}, PaymentSent: map[string]time.Time{}, Outbound: map[string]OutboundRecord{}, LastOutbound: map[string]OutboundNote{}, SeenChats: map[string]time.Time{}}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, s); err != nil {
		return nil, err
	}
	if s.Paused == nil {
		s.Paused = map[string]Pause{}
	}
	if s.Phones == nil {
		s.Phones = map[string]string{}
	}
	if s.PaymentSent == nil {
		s.PaymentSent = map[string]time.Time{}
	}
	if s.Outbound == nil {
		s.Outbound = map[string]OutboundRecord{}
	}
	if s.LastOutbound == nil {
		s.LastOutbound = map[string]OutboundNote{}
	}
	if s.SeenChats == nil {
		s.SeenChats = map[string]time.Time{}
	}
	return s, nil
}

// save writes atomically; the caller holds s.mu.
func (s *Store) save() error {
	if s.path == "" {
		return nil
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) PausedUntil(chatID string, now time.Time) (Pause, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.Paused[chatID]
	if !ok {
		return Pause{}, false
	}
	if !now.Before(p.Until) {
		delete(s.Paused, chatID)
		_ = s.save()
		return Pause{}, false
	}
	return p, true
}

func (s *Store) SetPause(chatID string, until time.Time, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Paused[chatID] = Pause{Until: until, Reason: reason}
	return s.save()
}

func (s *Store) ClearPause(chatID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Paused, chatID)
	return s.save()
}

func (s *Store) RememberPhone(phone, chatID string) {
	if phone == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Phones[phone] == chatID {
		return
	}
	s.Phones[phone] = chatID
	_ = s.save()
}

// ChatForPhone returns the chat id last seen for a phone, or the plain
// phone JID when the number never wrote to the CS number.
func (s *Store) ChatForPhone(phone string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.Phones[phone]; ok {
		return c
	}
	return phone + "@s.whatsapp.net"
}

func (s *Store) ActivePauses(now time.Time) map[string]Pause {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]Pause{}
	for k, p := range s.Paused {
		if now.Before(p.Until) {
			out[k] = p
		}
	}
	return out
}

const paymentWindow = 14 * 24 * time.Hour

func (s *Store) MarkPaymentSent(chatID string, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, t := range s.PaymentSent {
		if now.Sub(t) > paymentWindow {
			delete(s.PaymentSent, k)
		}
	}
	s.PaymentSent[chatID] = now
	_ = s.save()
}

// PaymentSentRecently reports whether payment info went to chatID within the window.
func (s *Store) PaymentSentRecently(chatID string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.PaymentSent[chatID]
	return ok && now.Sub(t) <= paymentWindow
}

// FindOutbound returns an existing successful send while its key is valid.
func (s *Store) FindOutbound(key string, now time.Time) (OutboundRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.Outbound[key]
	return rec, ok && rec.MessageID != "" && now.Before(rec.ExpiresAt)
}

// RecordOutbound saves the idempotency key and the latest note together,
// through the same atomic state-file replacement as the rest of the bridge.
// The caller must serialize sends for a chat before invoking this method.
func (s *Store) RecordOutbound(key, chatID, messageID, text, purpose string, now time.Time, expiry time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, rec := range s.Outbound {
		if !now.Before(rec.ExpiresAt) || now.Sub(rec.At) > outboundKeyWindow {
			delete(s.Outbound, k)
		}
	}
	for chat, note := range s.LastOutbound {
		if now.Sub(note.At) > outboundNoteWindow {
			delete(s.LastOutbound, chat)
		}
	}
	s.Outbound[key] = OutboundRecord{MessageID: messageID, At: now, ExpiresAt: now.Add(expiry)}
	runes := []rune(text)
	if len(runes) > 600 {
		text = string(runes[:600])
	}
	s.LastOutbound[chatID] = OutboundNote{Text: text, Purpose: purpose, At: now}
	return s.save()
}

// RecentOutbound gives Hermes context from the last 30 days, if any.
func (s *Store) RecentOutbound(chatID string, now time.Time) *OutboundNote {
	s.mu.Lock()
	defer s.mu.Unlock()
	note, ok := s.LastOutbound[chatID]
	if !ok || now.Sub(note.At) > outboundNoteWindow || note.At.After(now) {
		return nil
	}
	return &note
}


// RememberLeadChat records a real incoming lead chat, including @lid ids. A
// Dashboard cannot initiate outbound to arbitrary numbers not seen by CS.
func (s *Store) RememberLeadChat(chatID string, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.SeenChats[chatID]; exists {
		return
	}
	for chat, at := range s.SeenChats {
		if now.Sub(at) > 180*24*time.Hour {
			delete(s.SeenChats, chat)
		}
	}
	s.SeenChats[chatID] = now
	_ = s.save()
}

func (s *Store) HasSeenLeadChat(chatID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.SeenChats[chatID]; ok {
		return true
	}
	// Backward compatibility: existing bridge installations already retain the
	// phone -> chat map, populated by legitimate incoming lead messages.
	for _, known := range s.Phones {
		if known == chatID {
			return true
		}
	}
	return false
}

