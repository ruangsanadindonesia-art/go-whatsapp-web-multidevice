# Menu Jadwal Campaign (`/panel/jadwal`) — Spec & Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:executing-plans. Steps use `- [ ]`.

**Goal:** A "Jadwal" menu in the Ruang Sanad panel with a campaign dashboard, a calendar, a campaign list, and a campaign detail view that shows the full scheduled message, its recipients and their send history, plus pause/resume/cancel.

**Architecture:** Schedules stay in `scheduled_sends` (one row per recipient). Campaigns are a **read-time grouping** (no schema change for grouping). A new table `scheduled_send_runs` logs every final send outcome so the dashboard can count per date. The scheduler writes a run row on success and on final failure. New read APIs live under `/api/app/campaigns/*` on the device-scoped group.

**Tech:** Go/Fiber/SQLite; React panel (`web/`), Ruang Sanad tokens.

## Decisions (user, 2026-11-22)

| Topic | Decision |
|---|---|
| Campaign | Automatic grouping: schedules with the same device, message type, payload (minus recipient) and time pattern form one campaign. |
| Actions | View, plus pause/resume/cancel, for admin and CS (CS only on its own device). |
| History | New run log, accurate **from deployment onward**. Older data is approximated from `last_run_at`/`occurrence_count`, flagged as approximate. |
| Date filter | Hari ini, Kemarin, 7/14/30 hari, Bulan ini, single date, custom range. |
| Calendar | Past days: real runs from the log. Future days: projected occurrences (once/daily/weekly/monthly) up to end_at/limit. |
| Delivery | Separate PR, branch `claude/schedule-campaign-panel` on top of PR #23. |

## Global Constraints

- Times are shown in WIB (`Asia/Jakarta`); date params are `YYYY-MM-DD` in `CS_SERVICE_TZ`. Max range 366 days.
- Repository interface changes reach all 3 layers (`domains/chatstorage/interfaces.go`, `sqlite_repository.go`, `infrastructure/whatsapp/chatstorage_wrapper.go`).
- A run-log write failure must never fail or retry a send: log a warning and continue.
- Device scope: everything is filtered by `scheduleDeviceID(ctx)` (the same id schedules are stored under).
- Calendar projection is capped at 500 occurrences per schedule and per request range.
- Media is served only for a schedule on the caller's device, and only from the job's own stored assets (no path input).
- Indonesian copy, no emoji, "Ahad", dot time separator (Ruang Sanad DS).

## Campaign key

`sha256(device_id | message_type | canonical payload JSON without "phone" | scheduled_at UTC minute | recurrence | timezone | weekdays_json | day_of_month)`, first 16 hex chars. Payload keys are sorted by re-marshalling a `map[string]any`. Asset paths differ per job, so the hash uses asset **filenames**, not paths.

## Campaign status (derived from member schedules)

Priority order: any `running` or `active` → `aktif`; else all `paused` → `dijeda`; mixed paused/active counts as aktif; all in {completed} → `selesai`; any `failed` and none active → `gagal sebagian`/`gagal`; all `cancelled` → `dibatalkan`.

## Review Focus

1. **A run-log insert fails** (e.g. a locked DB). The send must still complete; the schedule state must be unchanged. → Task 2.
2. **A recurring schedule with no end** is projected into a 1-year calendar range. The 500 cap must apply and the response must stay fast. → Task 3.
3. **A CS account opens another device's campaign or asset** with a guessed key or id. → 403/404. → Task 4.
4. **Payload JSON from older versions or a broken payload** must not crash the detail view: show the summary and a raw fallback. → Task 3.
5. **Custom range where from > to**, or a single date → a correct inclusive range or a 400. → Tasks 4 and 5.

---

### Task 1: Run log table + repository (3 layers)
- [ ] Migration: `scheduled_send_runs(id INTEGER PK AUTOINCREMENT, schedule_id, device_id, occurrence_at TIMESTAMP, executed_at TIMESTAMP, status VARCHAR(16) /* sent|failed */, message_id, error TEXT)`, plus index `(device_id, executed_at)` and index `(schedule_id, executed_at)`.
- [ ] Domain: `ScheduledSendRun` struct; `ScheduledSendRunFilter{DeviceID string; ScheduleIDs []string; From, To time.Time; Limit int}`.
- [ ] Interface: `RecordScheduledSendRun(run *ScheduledSendRun) error`, `ListScheduledSendRuns(filter ScheduledSendRunFilter) ([]*ScheduledSendRun, error)`. Implement in sqlite and in the device wrapper (pass-through).
- [ ] Tests (sqlite): record+list by device and range; ScheduleIDs filter; device isolation.

### Task 2: Scheduler writes runs
- [ ] In `deliver` success → record `sent` (occurrence_at = job.NextRunAt, message_id). Final failure (`FailScheduledSend` path in deliver, and `skipOccurrence`) → record `failed` with the reason. Retries and offline waits are not logged.
- [ ] Tests (usecase, stub repo): success records 1 sent run; payload failure records failed; a repo `RecordScheduledSendRun` error does not change the returned error or the `CompleteScheduledSend` call.

### Task 3: Campaign usecase
- [ ] `domains/campaign`: DTOs `Campaign`, `CampaignDetail`, `Recipient`, `Run`, `Dashboard`, `CalendarEvent`, `MessagePreview`; interface `ICampaignUsecase { List(ctx, filter); Get(ctx, key); Dashboard(ctx, from, to); Calendar(ctx, from, to); Action(ctx, key, action); Asset(ctx, scheduleID, field) }`.
- [ ] Grouping (`campaignKey`), derived status, recipients with per-schedule status / last run / error.
- [ ] `MessagePreview` decoded from the payload by type: text (message), image/video/file/audio/sticker (caption + asset field + filename + url/upload), link (link + caption), location (lat/long), contact (name/phone), poll (question/options/max_answer), forward. Unknown → summary + raw.
- [ ] Dashboard for [from,to): sent, failed, success rate, unique recipients, active campaigns, upcoming (projected) in range, daily series (sent/failed/upcoming), by message type, top campaigns by sends, recent failures (≤10). `approximate: true` when the range starts before the first run-log row.
- [ ] Calendar: runs in range (past) + projections via `validations.NextScheduleOccurrence` for active/paused schedules (paused shown as `paused`), grouped per campaign per occurrence time.
- [ ] Actions: pause/resume/cancel applied to every member schedule allowed for that action; return counts `{changed, skipped}`.
- [ ] Tests: key stability (phone ignored, time pattern matters), status derivation, preview decoding per type + broken JSON, projection cap, dashboard counts from runs, action counts.

### Task 4: REST + scope
- [ ] `GET /api/app/campaigns?from&to&status&q`, `GET /api/app/campaigns/:key`, `GET /api/app/campaigns/dashboard?from&to`, `GET /api/app/campaigns/calendar?from&to`, `POST /api/app/campaigns/:key/{pause|resume|cancel}`, `GET /api/app/campaigns/assets/:schedule_id/:field`.
- [ ] `scopedRoute`: allow `/api/app/campaigns` (GET + POST actions) for restricted accounts with the header device check.
- [ ] Tests: date parsing (inclusive to, from>to → 400, >366 days → 400), unknown key → 404, scope test for another device → 403.

### Task 5: Panel UI (interface-design + dataviz skills)
- [ ] Nav item "Jadwal" → `/jadwal` with tabs Dashboard / Kalender / Daftar; shared `DateRangeFilter` (presets + single date + custom range, native date inputs), state in URL query.
- [ ] Dashboard: KPI strip, daily stacked bar chart (sent/failed + upcoming), type breakdown, top campaigns, recent failures.
- [ ] Calendar: month grid (Senin first), per-day chips per campaign with status colour; day click → day agenda; chip click → detail.
- [ ] Campaign detail drawer/page `/jadwal/c/:key`: WhatsApp-style message preview (media preview from asset endpoint), schedule pattern in words, recipients table (status, last sent, error), run history, pause/resume/cancel with confirmation.
- [ ] Tests (vitest): range presets → from/to; recurrence description; month grid weeks; status labels.
- [ ] Verify in a browser with seeded schedules + runs, desktop and mobile.

### Task 6: Final review, push, PR (base = `claude/focused-brown-ujirt3`)
