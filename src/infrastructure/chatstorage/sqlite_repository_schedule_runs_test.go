package chatstorage

import (
	"testing"
	"time"

	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	"github.com/stretchr/testify/require"
)

func TestSQLiteRepositoryScheduledSendRuns(t *testing.T) {
	repo := newTestSQLiteRepository(t)
	base := time.Date(2026, 11, 20, 2, 0, 0, 0, time.UTC)
	for i, r := range []domainChatStorage.ScheduledSendRun{
		{ScheduleID: "s1", DeviceID: "dev-a", Status: "sent", MessageID: "m1"},
		{ScheduleID: "s2", DeviceID: "dev-a", Status: "failed", Error: "boom"},
		{ScheduleID: "s1", DeviceID: "dev-b", Status: "sent"},
		{ScheduleID: "s3", DeviceID: "dev-a", Status: "sent"},
	} {
		r.OccurrenceAt = base.Add(time.Duration(i) * 24 * time.Hour)
		r.ExecutedAt = r.OccurrenceAt.Add(5 * time.Second)
		require.NoError(t, repo.RecordScheduledSendRun(&r))
	}

	runs, err := repo.ListScheduledSendRuns(domainChatStorage.ScheduledSendRunFilter{
		DeviceID: "dev-a", From: base, To: base.Add(2 * 24 * time.Hour),
	})
	require.NoError(t, err)
	require.Len(t, runs, 2, "range is [from,to) and device-scoped")
	require.Equal(t, "s1", runs[0].ScheduleID)
	require.Equal(t, "m1", runs[0].MessageID)
	require.True(t, runs[0].ExecutedAt.Equal(base.Add(5*time.Second)), runs[0].ExecutedAt)
	require.Equal(t, "failed", runs[1].Status)
	require.Equal(t, "boom", runs[1].Error)
	require.NotZero(t, runs[0].ID)

	byID, err := repo.ListScheduledSendRuns(domainChatStorage.ScheduledSendRunFilter{DeviceID: "dev-a", ScheduleIDs: []string{"s3"}})
	require.NoError(t, err)
	require.Len(t, byID, 1)
	require.Equal(t, "s3", byID[0].ScheduleID)

	first, err := repo.FirstScheduledSendRunAt("dev-a")
	require.NoError(t, err)
	require.NotNil(t, first)
	require.True(t, first.Equal(base.Add(5*time.Second)))
	none, err := repo.FirstScheduledSendRunAt("dev-z")
	require.NoError(t, err)
	require.Nil(t, none)
}
