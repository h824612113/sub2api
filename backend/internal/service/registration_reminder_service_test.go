//go:build unit

package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type registrationReminderSettingRepoStub struct {
	values map[string]string
}

func (s *registrationReminderSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (s *registrationReminderSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if v, ok := s.values[key]; ok {
		return v, nil
	}
	return "", ErrSettingNotFound
}

func (s *registrationReminderSettingRepoStub) Set(context.Context, string, string) error {
	return nil
}

func (s *registrationReminderSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	return nil, nil
}

func (s *registrationReminderSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	return nil
}

func (s *registrationReminderSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return s.values, nil
}

func (s *registrationReminderSettingRepoStub) Delete(context.Context, string) error {
	return nil
}

type registrationReminderSenderStub struct {
	sent []sentReminderEmail
}

type sentReminderEmail struct {
	to      string
	subject string
	body    string
}

func (s *registrationReminderSenderStub) SendEmail(_ context.Context, to, subject, body string) error {
	s.sent = append(s.sent, sentReminderEmail{
		to:      to,
		subject: subject,
		body:    body,
	})
	return nil
}

type registrationReminderUserRepoStub struct {
	mockUserRepo
	firstAdmin *User
}

func (s *registrationReminderUserRepoStub) GetFirstAdmin(context.Context) (*User, error) {
	if s.firstAdmin == nil {
		return nil, ErrUserNotFound
	}
	return s.firstAdmin, nil
}

func TestRegistrationReminderRunOnce_SendsToVerifiedNotifyEmails(t *testing.T) {
	repo := &registrationReminderSettingRepoStub{
		values: map[string]string{
			SettingKeyRegistrationEnabled:      "false",
			SettingKeySiteName:                 "Test Site",
			SettingKeyAccountQuotaNotifyEmails: `[{"email":"ops@example.com","disabled":false,"verified":true},{"email":"skip@example.com","disabled":true,"verified":true}]`,
		},
	}
	userSvc := NewUserService(&registrationReminderUserRepoStub{
		firstAdmin: &User{Email: "admin@example.com"},
	}, repo, nil, nil)
	sender := &registrationReminderSenderStub{}
	svc := NewRegistrationReminderService(repo, userSvc, nil, nil, &config.Config{})
	svc.emailSender = sender

	svc.runOnceAt(context.Background(), time.Date(2026, 5, 15, 9, 0, 0, 0, time.UTC))

	require.Len(t, sender.sent, 1)
	require.Equal(t, "ops@example.com", sender.sent[0].to)
	require.Contains(t, sender.sent[0].subject, "Test Site")
	require.Contains(t, sender.sent[0].body, "registration_enabled")
}

func TestRegistrationReminderRunOnce_FallsBackToAdmin(t *testing.T) {
	repo := &registrationReminderSettingRepoStub{
		values: map[string]string{
			SettingKeyRegistrationEnabled: "false",
			SettingKeySiteName:            "Fallback Site",
		},
	}
	userSvc := NewUserService(&registrationReminderUserRepoStub{
		firstAdmin: &User{Email: "admin@example.com"},
	}, repo, nil, nil)
	sender := &registrationReminderSenderStub{}
	svc := NewRegistrationReminderService(repo, userSvc, nil, nil, &config.Config{})
	svc.emailSender = sender

	svc.runOnceAt(context.Background(), time.Date(2026, 5, 15, 9, 0, 0, 0, time.UTC))

	require.Len(t, sender.sent, 1)
	require.Equal(t, "admin@example.com", sender.sent[0].to)
}

func TestRegistrationReminderRunOnce_SkipsWhenEnabled(t *testing.T) {
	repo := &registrationReminderSettingRepoStub{
		values: map[string]string{
			SettingKeyRegistrationEnabled: "true",
		},
	}
	userSvc := NewUserService(&registrationReminderUserRepoStub{
		firstAdmin: &User{Email: "admin@example.com"},
	}, repo, nil, nil)
	sender := &registrationReminderSenderStub{}
	svc := NewRegistrationReminderService(repo, userSvc, nil, nil, &config.Config{})
	svc.emailSender = sender

	svc.runOnceAt(context.Background(), time.Date(2026, 5, 15, 9, 0, 0, 0, time.UTC))

	require.Empty(t, sender.sent)
}

func TestRegistrationReminderShouldRunNow_OncePerDay(t *testing.T) {
	svc := NewRegistrationReminderService(nil, nil, nil, nil, &config.Config{})
	now := time.Date(2026, 5, 15, 9, 0, 0, 0, time.UTC)

	require.True(t, svc.shouldRunNowFromLastRun(time.Time{}, now))
	require.False(t, svc.shouldRunNowFromLastRun(now.Add(-10*time.Minute), now))
	require.True(t, svc.shouldRunNowFromLastRun(now.Add(-24*time.Hour), now))
}

func (s *RegistrationReminderService) runOnceAt(ctx context.Context, now time.Time) {
	if s == nil || s.settingRepo == nil || s.userService == nil || s.emailSender == nil {
		return
	}
	enabled, err := s.isRegistrationEnabled(ctx)
	if err != nil || enabled {
		return
	}
	if !s.shouldRunNowFromLastRun(time.Time{}, now) {
		return
	}
	recipients := s.resolveRecipients(ctx)
	if len(recipients) == 0 {
		return
	}
	siteName := s.getSiteName(ctx)
	subject := fmt.Sprintf("[%s] 注册仍处于关闭状态", siteName)
	body := buildRegistrationDisabledReminderEmailHTML(siteName, now)
	for _, to := range recipients {
		_ = s.emailSender.SendEmail(ctx, to, subject, body)
	}
}
