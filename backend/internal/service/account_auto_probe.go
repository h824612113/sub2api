package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

const (
	AccountAutoProbeExtraKey        = "account_auto_probe"
	AccountAutoProbeEnabledExtraKey = "account_auto_probe_enabled"

	accountAutoProbeDefaultIntervalMinutes = 30
	accountAutoProbeMinIntervalMinutes     = 1
	accountAutoProbeMaxIntervalMinutes     = 24 * 60
	accountAutoProbeCycleInterval          = time.Minute
	accountAutoProbeRequestTimeout         = 45 * time.Second
	accountAutoProbeMaxPerCycle            = 20
	accountAutoProbeConcurrency            = 4
	accountAutoProbeMaxFailureBackoff      = 6 * time.Hour
	accountAutoProbeLeaderLockKey          = "account:auto-probe:leader"
	accountAutoProbeLeaderLockTTL          = 5 * time.Minute
)

const (
	AccountAutoProbeStatusHealthy = "healthy"
	AccountAutoProbeStatusFailed  = "failed"
)

var ErrAccountAutoProbeUnavailable = infraerrors.ServiceUnavailable(
	"ACCOUNT_AUTO_PROBE_UNAVAILABLE", "automatic account probing is unavailable",
)

// AccountAutoProbeSettings controls automatic connectivity tests for managed accounts.
// The feature is opt-in because every probe consumes upstream quota.
type AccountAutoProbeSettings struct {
	Enabled         bool `json:"enabled"`
	IntervalMinutes int  `json:"interval_minutes"`
	AutoRecover     bool `json:"auto_recover"`
}

// AccountAutoProbeSnapshot is persisted in accounts.extra. It intentionally stores
// an error category instead of an upstream response body, which can contain secrets.
type AccountAutoProbeSnapshot struct {
	Status        string     `json:"status"`
	LatencyMs     int64      `json:"latency_ms,omitempty"`
	FirstTokenMs  *int       `json:"first_token_ms,omitempty"`
	LastAttemptAt time.Time  `json:"last_attempt_at"`
	NextProbeAt   time.Time  `json:"next_probe_at"`
	FreshUntil    time.Time  `json:"fresh_until"`
	FailureCount  int        `json:"failure_count,omitempty"`
	LastErrorCode string     `json:"last_error_code,omitempty"`
	RecoveredAt   *time.Time `json:"recovered_at,omitempty"`
}

type accountAutoProbeDueLister interface {
	ListDueAccountAutoProbeAccounts(context.Context, time.Time, int) ([]Account, error)
}

// GetAccountAutoProbeSettings returns safe defaults when no setting has been saved.
func (s *SettingService) GetAccountAutoProbeSettings(ctx context.Context) (*AccountAutoProbeSettings, error) {
	defaults := defaultAccountAutoProbeSettings()
	if s == nil || s.settingRepo == nil {
		return defaults, nil
	}

	raw, err := s.settingRepo.GetValue(ctx, SettingKeyAccountAutoProbeSettings)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return defaults, nil
		}
		return nil, fmt.Errorf("get automatic account probe settings: %w", err)
	}
	if strings.TrimSpace(raw) == "" {
		return defaults, nil
	}

	settings := *defaults
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return nil, fmt.Errorf("parse automatic account probe settings: %w", err)
	}
	if settings.IntervalMinutes == 0 {
		settings.IntervalMinutes = defaults.IntervalMinutes
	}
	normalizeAccountAutoProbeSettings(&settings)
	return &settings, nil
}

// SetAccountAutoProbeSettings validates and persists automatic probe settings.
func (s *SettingService) SetAccountAutoProbeSettings(ctx context.Context, settings *AccountAutoProbeSettings) error {
	if s == nil || s.settingRepo == nil {
		return ErrAccountAutoProbeUnavailable
	}
	if settings == nil {
		return infraerrors.BadRequest("INVALID_ACCOUNT_AUTO_PROBE_SETTINGS", "settings cannot be nil")
	}
	if settings.IntervalMinutes < accountAutoProbeMinIntervalMinutes || settings.IntervalMinutes > accountAutoProbeMaxIntervalMinutes {
		return infraerrors.BadRequest(
			"INVALID_ACCOUNT_AUTO_PROBE_INTERVAL",
			fmt.Sprintf("interval_minutes must be between %d and %d", accountAutoProbeMinIntervalMinutes, accountAutoProbeMaxIntervalMinutes),
		)
	}
	normalizeAccountAutoProbeSettings(settings)
	raw, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal automatic account probe settings: %w", err)
	}
	return s.settingRepo.Set(ctx, SettingKeyAccountAutoProbeSettings, string(raw))
}

func defaultAccountAutoProbeSettings() *AccountAutoProbeSettings {
	return &AccountAutoProbeSettings{
		Enabled:         false,
		IntervalMinutes: accountAutoProbeDefaultIntervalMinutes,
		AutoRecover:     true,
	}
}

func normalizeAccountAutoProbeSettings(settings *AccountAutoProbeSettings) {
	if settings.IntervalMinutes < accountAutoProbeMinIntervalMinutes {
		settings.IntervalMinutes = accountAutoProbeMinIntervalMinutes
	}
	if settings.IntervalMinutes > accountAutoProbeMaxIntervalMinutes {
		settings.IntervalMinutes = accountAutoProbeMaxIntervalMinutes
	}
}

// AccountAutoProbeService periodically sends a real, minimal model request through
// each eligible account. Results are available to both the admin UI and scheduler.
type AccountAutoProbeService struct {
	accountRepo        AccountRepository
	accountTestService *AccountTestService
	settingService     *SettingService
	rateLimitService   *RateLimitService

	parentCtx    context.Context
	parentCancel context.CancelFunc
	wakeCh       chan struct{}
	wg           sync.WaitGroup
	mu           sync.Mutex
	cycleMu      sync.Mutex
	started      bool
	stopped      bool
	probeGroup   singleflight.Group
	probeSlots   chan struct{}
	now          func() time.Time
	lockCache    LeaderLockCache
	db           *sql.DB
	instanceID   string
}

func NewAccountAutoProbeService(
	accountRepo AccountRepository,
	accountTestService *AccountTestService,
	settingService *SettingService,
	rateLimitService *RateLimitService,
) *AccountAutoProbeService {
	ctx, cancel := context.WithCancel(context.Background())
	return &AccountAutoProbeService{
		accountRepo:        accountRepo,
		accountTestService: accountTestService,
		settingService:     settingService,
		rateLimitService:   rateLimitService,
		parentCtx:          ctx,
		parentCancel:       cancel,
		wakeCh:             make(chan struct{}, 1),
		probeSlots:         make(chan struct{}, accountAutoProbeConcurrency),
		now:                time.Now,
		instanceID:         uuid.NewString(),
	}
}

func ProvideAccountAutoProbeService(
	accountRepo AccountRepository,
	accountTestService *AccountTestService,
	settingService *SettingService,
	rateLimitService *RateLimitService,
	lockCache LeaderLockCache,
	db *sql.DB,
) *AccountAutoProbeService {
	service := NewAccountAutoProbeService(accountRepo, accountTestService, settingService, rateLimitService)
	service.SetLeaderLock(lockCache, db)
	service.Start()
	return service
}

func (s *AccountAutoProbeService) SetLeaderLock(lockCache LeaderLockCache, db *sql.DB) {
	if s == nil {
		return
	}
	s.lockCache = lockCache
	s.db = db
}

func (s *AccountAutoProbeService) Start() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.started || s.stopped {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.wg.Add(1)
	s.mu.Unlock()
	go s.runLoop()
}

func (s *AccountAutoProbeService) Stop() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	s.parentCancel()
	s.mu.Unlock()
	s.wg.Wait()
}

func (s *AccountAutoProbeService) runLoop() {
	defer s.wg.Done()
	_ = s.RunDue(s.parentCtx)
	ticker := time.NewTicker(accountAutoProbeCycleInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.parentCtx.Done():
			return
		case <-ticker.C:
		case <-s.wakeCh:
		}
		if err := s.RunDue(s.parentCtx); err != nil && !errors.Is(err, context.Canceled) {
			logger.LegacyPrintf("service.account_auto_probe", "run_due_failed: err=%v", err)
		}
	}
}

func (s *AccountAutoProbeService) GetSettings(ctx context.Context) (*AccountAutoProbeSettings, error) {
	if s == nil || s.settingService == nil {
		return defaultAccountAutoProbeSettings(), nil
	}
	return s.settingService.GetAccountAutoProbeSettings(ctx)
}

func (s *AccountAutoProbeService) UpdateSettings(ctx context.Context, settings *AccountAutoProbeSettings) error {
	if s == nil || s.settingService == nil {
		return ErrAccountAutoProbeUnavailable
	}
	if err := s.settingService.SetAccountAutoProbeSettings(ctx, settings); err != nil {
		return err
	}
	if settings.Enabled {
		select {
		case s.wakeCh <- struct{}{}:
		default:
		}
	}
	return nil
}

// RunDue executes one bounded batch. A leader lock prevents duplicate probes in
// multi-instance deployments, while per-account singleflight protects manual calls.
func (s *AccountAutoProbeService) RunDue(ctx context.Context) error {
	if s == nil || s.accountRepo == nil || s.accountTestService == nil {
		return nil
	}
	s.cycleMu.Lock()
	defer s.cycleMu.Unlock()

	settings, err := s.GetSettings(ctx)
	if err != nil {
		return err
	}
	if !settings.Enabled {
		return nil
	}

	release, acquired := tryAcquireSingletonLeaderLock(
		ctx,
		s.lockCache,
		s.db,
		accountAutoProbeLeaderLockKey,
		s.instanceID,
		accountAutoProbeLeaderLockTTL,
	)
	if !acquired {
		return nil
	}
	defer release()

	now := s.currentTime().UTC()
	accounts, err := s.listDueAccounts(ctx, now)
	if err != nil {
		return fmt.Errorf("list automatic account probes: %w", err)
	}

	var group errgroup.Group
	for i := range accounts {
		accountID := accounts[i].ID
		group.Go(func() error {
			if _, probeErr := s.probeScheduledAccount(ctx, accountID, settings); probeErr != nil && !errors.Is(probeErr, context.Canceled) {
				logger.LegacyPrintf("service.account_auto_probe", "probe_failed: account_id=%d err=%v", accountID, probeErr)
			}
			return nil
		})
	}
	return group.Wait()
}

func (s *AccountAutoProbeService) listDueAccounts(ctx context.Context, now time.Time) ([]Account, error) {
	if lister, ok := s.accountRepo.(accountAutoProbeDueLister); ok {
		return lister.ListDueAccountAutoProbeAccounts(ctx, now, accountAutoProbeMaxPerCycle)
	}

	accounts, err := s.accountRepo.ListAllWithFilters(ctx, "", "", "", "", 0, "")
	if err != nil {
		return nil, err
	}
	due := make([]Account, 0, len(accounts))
	for i := range accounts {
		if !accountAutoProbeEligible(&accounts[i]) || !accountAutoProbeDue(&accounts[i], now) {
			continue
		}
		due = append(due, accounts[i])
	}
	sort.SliceStable(due, func(i, j int) bool {
		left := decodeAccountAutoProbeSnapshot(due[i].Extra)
		right := decodeAccountAutoProbeSnapshot(due[j].Extra)
		if left == nil || left.NextProbeAt.IsZero() {
			return right != nil && !right.NextProbeAt.IsZero() || due[i].ID < due[j].ID
		}
		if right == nil || right.NextProbeAt.IsZero() {
			return false
		}
		if left.NextProbeAt.Equal(right.NextProbeAt) {
			return due[i].ID < due[j].ID
		}
		return left.NextProbeAt.Before(right.NextProbeAt)
	})
	if len(due) > accountAutoProbeMaxPerCycle {
		due = due[:accountAutoProbeMaxPerCycle]
	}
	return due, nil
}

func (s *AccountAutoProbeService) probeScheduledAccount(
	ctx context.Context,
	accountID int64,
	settings *AccountAutoProbeSettings,
) (*AccountAutoProbeSnapshot, error) {
	key := fmt.Sprintf("%d", accountID)
	value, err, _ := s.probeGroup.Do(key, func() (any, error) {
		select {
		case s.probeSlots <- struct{}{}:
			defer func() { <-s.probeSlots }()
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		account, loadErr := s.accountRepo.GetByID(ctx, accountID)
		if loadErr != nil {
			return nil, loadErr
		}
		now := s.currentTime().UTC()
		if !accountAutoProbeEligible(account) || !accountAutoProbeDue(account, now) {
			return nil, nil
		}
		return s.probeLoadedAccount(ctx, account, settings)
	})
	if err != nil || value == nil {
		return nil, err
	}
	snapshot, ok := value.(*AccountAutoProbeSnapshot)
	if !ok {
		return nil, errors.New("invalid automatic account probe result")
	}
	return snapshot, nil
}

func (s *AccountAutoProbeService) probeLoadedAccount(
	ctx context.Context,
	account *Account,
	settings *AccountAutoProbeSettings,
) (*AccountAutoProbeSnapshot, error) {
	probeCtx, cancel := context.WithTimeout(ctx, accountAutoProbeRequestTimeout)
	defer cancel()

	result, runErr := s.accountTestService.RunTestBackground(probeCtx, account.ID, "")
	now := s.currentTime().UTC()
	previous := decodeAccountAutoProbeSnapshot(account.Extra)
	failureCount := 0
	if previous != nil {
		failureCount = previous.FailureCount
	}

	interval := time.Duration(settings.IntervalMinutes) * time.Minute
	snapshot := &AccountAutoProbeSnapshot{
		Status:        AccountAutoProbeStatusHealthy,
		LastAttemptAt: now,
		NextProbeAt:   now.Add(interval),
		FreshUntil:    now.Add(maxDuration(3*interval, time.Hour)),
	}
	if result != nil {
		snapshot.LatencyMs = result.LatencyMs
		snapshot.FirstTokenMs = result.FirstTokenMs
	}

	success := runErr == nil && result != nil && result.Status == "success"
	if !success {
		failureCount++
		snapshot.Status = AccountAutoProbeStatusFailed
		snapshot.FailureCount = failureCount
		snapshot.LastErrorCode = accountAutoProbeErrorCode(runErr, result)
		snapshot.NextProbeAt = now.Add(accountAutoProbeFailureDelay(interval, failureCount))
		snapshot.FreshUntil = snapshot.NextProbeAt.Add(maxDuration(2*interval, time.Hour))
	} else if settings.AutoRecover && s.rateLimitService != nil {
		recovery, recoverErr := s.rateLimitService.RecoverAccountAfterSuccessfulTest(ctx, account.ID)
		if recoverErr != nil {
			logger.LegacyPrintf("service.account_auto_probe", "auto_recover_failed: account_id=%d err=%v", account.ID, recoverErr)
		} else if recovery != nil && (recovery.ClearedError || recovery.ClearedRateLimit) {
			recoveredAt := now
			snapshot.RecoveredAt = &recoveredAt
		}
	}

	if err := s.accountRepo.UpdateExtra(ctx, account.ID, map[string]any{AccountAutoProbeExtraKey: snapshot}); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func accountAutoProbeEligible(account *Account) bool {
	if account == nil {
		return false
	}
	if enabled, ok := account.Extra[AccountAutoProbeEnabledExtraKey].(bool); ok && !enabled {
		return false
	}
	if account.Status == StatusError {
		return true
	}
	return account.Status == StatusActive && account.Schedulable
}

func accountAutoProbeDue(account *Account, now time.Time) bool {
	snapshot := decodeAccountAutoProbeSnapshot(account.Extra)
	return snapshot == nil || snapshot.NextProbeAt.IsZero() || !now.Before(snapshot.NextProbeAt)
}

func decodeAccountAutoProbeSnapshot(extra map[string]any) *AccountAutoProbeSnapshot {
	if extra == nil {
		return nil
	}
	raw, ok := extra[AccountAutoProbeExtraKey]
	if !ok || raw == nil {
		return nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var snapshot AccountAutoProbeSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil
	}
	if snapshot.Status != AccountAutoProbeStatusHealthy && snapshot.Status != AccountAutoProbeStatusFailed {
		return nil
	}
	return &snapshot
}

func accountAutoProbeSchedulingSample(account *Account, now time.Time) (errorRate, ttft float64, hasTTFT, ok bool) {
	if account == nil {
		return 0, 0, false, false
	}
	snapshot := decodeAccountAutoProbeSnapshot(account.Extra)
	if snapshot == nil || snapshot.FreshUntil.IsZero() || now.After(snapshot.FreshUntil) {
		return 0, 0, false, false
	}
	if snapshot.Status == AccountAutoProbeStatusFailed {
		errorRate = 1
	}
	if snapshot.FirstTokenMs != nil && *snapshot.FirstTokenMs > 0 {
		ttft = float64(*snapshot.FirstTokenMs)
		hasTTFT = true
	} else if snapshot.LatencyMs > 0 {
		ttft = float64(snapshot.LatencyMs)
		hasTTFT = true
	}
	return errorRate, ttft, hasTTFT, true
}

func accountAutoProbeFailureDelay(base time.Duration, failures int) time.Duration {
	if base <= 0 {
		base = time.Duration(accountAutoProbeDefaultIntervalMinutes) * time.Minute
	}
	shift := failures - 1
	if shift < 0 {
		shift = 0
	}
	if shift > 10 {
		shift = 10
	}
	delay := base * time.Duration(1<<shift)
	if delay > accountAutoProbeMaxFailureBackoff {
		return accountAutoProbeMaxFailureBackoff
	}
	return delay
}

func accountAutoProbeErrorCode(runErr error, result *ScheduledTestResult) string {
	message := ""
	if runErr != nil {
		message = runErr.Error()
	} else if result != nil {
		message = result.ErrorMessage
	}
	lower := strings.ToLower(message)
	switch {
	case errors.Is(runErr, context.DeadlineExceeded), strings.Contains(lower, "deadline exceeded"), strings.Contains(lower, "timeout"):
		return "timeout"
	case strings.Contains(lower, "api returned 401"), strings.Contains(lower, "unauthorized"):
		return "http_401"
	case strings.Contains(lower, "api returned 403"), strings.Contains(lower, "forbidden"):
		return "http_403"
	case strings.Contains(lower, "api returned 429"), strings.Contains(lower, "rate limit"):
		return "http_429"
	case strings.Contains(lower, "api returned 5"):
		return "http_5xx"
	case strings.Contains(lower, "no access token"), strings.Contains(lower, "no api key"), strings.Contains(lower, "credential"):
		return "credential_unavailable"
	case strings.Contains(lower, "proxy"):
		return "proxy_unavailable"
	default:
		return "probe_failed"
	}
}

func maxDuration(left, right time.Duration) time.Duration {
	if left > right {
		return left
	}
	return right
}

func (s *AccountAutoProbeService) currentTime() time.Time {
	if s == nil || s.now == nil {
		return time.Now()
	}
	return s.now()
}
