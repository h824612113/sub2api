package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type accountAutoProbeSettingRepoStub struct {
	values map[string]string
}

func (r *accountAutoProbeSettingRepoStub) Get(_ context.Context, key string) (*Setting, error) {
	value, err := r.GetValue(context.Background(), key)
	if err != nil {
		return nil, err
	}
	return &Setting{Key: key, Value: value}, nil
}

func (r *accountAutoProbeSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if value, ok := r.values[key]; ok {
		return value, nil
	}
	return "", ErrSettingNotFound
}

func (r *accountAutoProbeSettingRepoStub) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

func (r *accountAutoProbeSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			result[key] = value
		}
	}
	return result, nil
}

func (r *accountAutoProbeSettingRepoStub) SetMultiple(_ context.Context, values map[string]string) error {
	for key, value := range values {
		r.values[key] = value
	}
	return nil
}

func (r *accountAutoProbeSettingRepoStub) GetAll(_ context.Context) (map[string]string, error) {
	result := make(map[string]string, len(r.values))
	for key, value := range r.values {
		result[key] = value
	}
	return result, nil
}

func (r *accountAutoProbeSettingRepoStub) Delete(_ context.Context, key string) error {
	delete(r.values, key)
	return nil
}

func TestAccountAutoProbeSettings_DefaultsAndRoundTrip(t *testing.T) {
	repo := &accountAutoProbeSettingRepoStub{values: map[string]string{}}
	settings := NewSettingService(repo, nil)

	got, err := settings.GetAccountAutoProbeSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, &AccountAutoProbeSettings{Enabled: false, IntervalMinutes: 30, AutoRecover: true}, got)

	want := &AccountAutoProbeSettings{Enabled: true, IntervalMinutes: 5, AutoRecover: false}
	require.NoError(t, settings.SetAccountAutoProbeSettings(context.Background(), want))
	require.JSONEq(t, `{"enabled":true,"interval_minutes":5,"auto_recover":false}`, repo.values[SettingKeyAccountAutoProbeSettings])

	got, err = settings.GetAccountAutoProbeSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestAccountAutoProbeSettings_RejectsOutOfRangeInterval(t *testing.T) {
	repo := &accountAutoProbeSettingRepoStub{values: map[string]string{}}
	settings := NewSettingService(repo, nil)

	for _, interval := range []int{0, -1, accountAutoProbeMaxIntervalMinutes + 1} {
		err := settings.SetAccountAutoProbeSettings(context.Background(), &AccountAutoProbeSettings{
			Enabled:         true,
			IntervalMinutes: interval,
		})
		require.Error(t, err)
	}
}

func TestAccountAutoProbeSchedulingSample_UsesFreshSnapshot(t *testing.T) {
	now := time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)
	firstToken := 240
	data, err := json.Marshal(&AccountAutoProbeSnapshot{
		Status:        AccountAutoProbeStatusHealthy,
		FirstTokenMs:  &firstToken,
		LastAttemptAt: now.Add(-time.Minute),
		NextProbeAt:   now.Add(time.Minute),
		FreshUntil:    now.Add(time.Hour),
	})
	require.NoError(t, err)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))

	account := &Account{ID: 42, Extra: map[string]any{AccountAutoProbeExtraKey: raw}}
	errorRate, ttft, hasTTFT, ok := accountAutoProbeSchedulingSample(account, now)
	require.True(t, ok)
	require.Zero(t, errorRate)
	require.Equal(t, 240.0, ttft)
	require.True(t, hasTTFT)

	raw["status"] = AccountAutoProbeStatusFailed
	account.Extra[AccountAutoProbeExtraKey] = raw
	errorRate, _, _, ok = accountAutoProbeSchedulingSample(account, now)
	require.True(t, ok)
	require.Equal(t, 1.0, errorRate)

	raw["fresh_until"] = now.Add(-time.Second).Format(time.RFC3339)
	account.Extra[AccountAutoProbeExtraKey] = raw
	_, _, _, ok = accountAutoProbeSchedulingSample(account, now)
	require.False(t, ok)
}

func TestAccountAutoProbeEligibility(t *testing.T) {
	require.True(t, accountAutoProbeEligible(&Account{Status: StatusActive, Schedulable: true}))
	require.False(t, accountAutoProbeEligible(&Account{Status: StatusActive, Schedulable: false}))
	require.True(t, accountAutoProbeEligible(&Account{Status: StatusError, Schedulable: false}))
	require.False(t, accountAutoProbeEligible(&Account{
		Status:      StatusActive,
		Schedulable: true,
		Extra:       map[string]any{AccountAutoProbeEnabledExtraKey: false},
	}))
}

func TestAccountAutoProbeFailureDelayBacksOffAndCaps(t *testing.T) {
	base := 5 * time.Minute
	require.Equal(t, base, accountAutoProbeFailureDelay(base, 1))
	require.Equal(t, 10*time.Minute, accountAutoProbeFailureDelay(base, 2))
	require.Equal(t, accountAutoProbeMaxFailureBackoff, accountAutoProbeFailureDelay(base, 20))
}

func TestAccountAutoProbeErrorCodeDoesNotPersistUpstreamBody(t *testing.T) {
	result := &ScheduledTestResult{Status: "failed", ErrorMessage: "API returned 401: token=secret-value"}
	require.Equal(t, "http_401", accountAutoProbeErrorCode(nil, result))
	require.NotContains(t, accountAutoProbeErrorCode(nil, result), "secret-value")
	require.Equal(t, "timeout", accountAutoProbeErrorCode(context.DeadlineExceeded, nil))
	require.False(t, errors.Is(nil, context.DeadlineExceeded))
}
