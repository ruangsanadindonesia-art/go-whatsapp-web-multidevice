package usecase

import (
	"context"
	"errors"
	"testing"

	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	domainSend "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/send"
	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	"github.com/stretchr/testify/require"
)

type runSendStub struct {
	domainSend.ISendUsecase
	resp domainSend.GenericResponse
	err  error
}

func (s *runSendStub) SendText(context.Context, domainSend.MessageRequest) (domainSend.GenericResponse, error) {
	return s.resp, s.err
}

func runsOf(t *testing.T, repo domainChatStorage.IChatStorageRepository) []*domainChatStorage.ScheduledSendRun {
	t.Helper()
	runs, err := repo.ListScheduledSendRuns(domainChatStorage.ScheduledSendRunFilter{DeviceID: "device-a"})
	require.NoError(t, err)
	return runs
}

func TestScheduleDeliverRecordsSentRun(t *testing.T) {
	service, repo := newScheduleTestService(t, &runSendStub{resp: domainSend.GenericResponse{MessageID: "msg-1"}})
	job := claimScheduleTestJob(t, service, repo, newScheduleTestJob("run-ok", "once", scheduleTestNow))
	require.NoError(t, service.deliver(scheduleTestContext(), job))

	runs := runsOf(t, repo)
	require.Len(t, runs, 1)
	require.Equal(t, "sent", runs[0].Status)
	require.Equal(t, "msg-1", runs[0].MessageID)
	require.Equal(t, "run-ok", runs[0].ScheduleID)
	require.True(t, runs[0].OccurrenceAt.Equal(job.NextRunAt))
	require.True(t, runs[0].ExecutedAt.Equal(scheduleTestNow))
}

func TestScheduleDeliverRecordsFailedRunOnPermanentError(t *testing.T) {
	service, repo := newScheduleTestService(t, &runSendStub{err: pkgError.ValidationError("bad phone")})
	job := claimScheduleTestJob(t, service, repo, newScheduleTestJob("run-bad", "once", scheduleTestNow))
	require.Error(t, service.deliver(scheduleTestContext(), job))

	runs := runsOf(t, repo)
	require.Len(t, runs, 1)
	require.Equal(t, "failed", runs[0].Status)
	require.Contains(t, runs[0].Error, "bad phone")
}

func TestScheduleDeliverRetryIsNotLogged(t *testing.T) {
	service, repo := newScheduleTestService(t, &runSendStub{err: errors.New("timeout")})
	job := claimScheduleTestJob(t, service, repo, newScheduleTestJob("run-retry", "once", scheduleTestNow))
	_ = service.deliver(scheduleTestContext(), job)
	require.Empty(t, runsOf(t, repo))
}

func TestScheduleDeliverRecordsFailedRunWhenAttemptsExhausted(t *testing.T) {
	service, repo := newScheduleTestService(t, &runSendStub{err: errors.New("timeout")})
	j := newScheduleTestJob("run-skip", "daily", scheduleTestNow)
	j.Attempts = scheduleMaxAttempts - 1
	job := claimScheduleTestJob(t, service, repo, j)
	_ = service.deliver(scheduleTestContext(), job)
	runs := runsOf(t, repo)
	require.Len(t, runs, 1)
	require.Equal(t, "failed", runs[0].Status)
}

// A run-log write failure must not change the send outcome.
type failingRunRepo struct {
	domainChatStorage.IChatStorageRepository
}

func (failingRunRepo) RecordScheduledSendRun(*domainChatStorage.ScheduledSendRun) error {
	return errors.New("database is locked")
}

func TestScheduleDeliverIgnoresRunLogErrors(t *testing.T) {
	service, repo := newScheduleTestService(t, &runSendStub{resp: domainSend.GenericResponse{MessageID: "msg-2"}})
	job := claimScheduleTestJob(t, service, repo, newScheduleTestJob("run-logfail", "once", scheduleTestNow))
	service.repo = failingRunRepo{repo}
	require.NoError(t, service.deliver(scheduleTestContext(), job))

	stored, err := repo.GetScheduledSend("device-a", "run-logfail")
	require.NoError(t, err)
	require.Equal(t, scheduleStatusCompleted, stored.Status)
	require.Equal(t, "msg-2", stored.LastMessageID)
}
