package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
)

const (
	registrationReminderReportType       = "registration_disabled_reminder"
	registrationReminderLeaderLockKey    = "registration:disabled_reminder:leader"
	registrationReminderLastRunKeyPrefix = "registration:disabled_reminder:last_run:"
	registrationReminderLockTTL          = 5 * time.Minute
	registrationReminderTickInterval     = 1 * time.Minute
	registrationReminderDefaultSchedule  = "0 9 * * *"
)

var registrationReminderCronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

type registrationReminderEmailSender interface {
	SendEmail(ctx context.Context, to, subject, body string) error
}

type RegistrationReminderService struct {
	settingRepo SettingRepository
	userService *UserService
	emailSender registrationReminderEmailSender
	redisClient *redis.Client
	cfg         *config.Config
	instanceID  string
	loc         *time.Location
	lockEnabled bool
	warnNoRedis sync.Once
	startOnce   sync.Once
	stopOnce    sync.Once
	stopCtx     context.Context
	stop        context.CancelFunc
	wg          sync.WaitGroup
}

func NewRegistrationReminderService(
	settingRepo SettingRepository,
	userService *UserService,
	emailService *EmailService,
	redisClient *redis.Client,
	cfg *config.Config,
) *RegistrationReminderService {
	lockOn := cfg == nil || strings.TrimSpace(cfg.RunMode) != config.RunModeSimple
	loc := time.Local
	if cfg != nil && strings.TrimSpace(cfg.Timezone) != "" {
		if parsed, err := time.LoadLocation(strings.TrimSpace(cfg.Timezone)); err == nil && parsed != nil {
			loc = parsed
		}
	}
	return &RegistrationReminderService{
		settingRepo: settingRepo,
		userService: userService,
		emailSender: emailService,
		redisClient: redisClient,
		cfg:         cfg,
		instanceID:  fmt.Sprintf("registration-reminder-%d", time.Now().UnixNano()),
		loc:         loc,
		lockEnabled: lockOn,
	}
}

func ProvideRegistrationReminderService(
	settingRepo SettingRepository,
	userService *UserService,
	emailService *EmailService,
	redisClient *redis.Client,
	cfg *config.Config,
) *RegistrationReminderService {
	svc := NewRegistrationReminderService(settingRepo, userService, emailService, redisClient, cfg)
	svc.Start()
	return svc
}

func (s *RegistrationReminderService) Start() {
	s.StartWithContext(context.Background())
}

func (s *RegistrationReminderService) StartWithContext(ctx context.Context) {
	if s == nil || s.settingRepo == nil || s.userService == nil || s.emailSender == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	s.startOnce.Do(func() {
		s.stopCtx, s.stop = context.WithCancel(ctx)
		s.wg.Add(1)
		go s.run()
	})
}

func (s *RegistrationReminderService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		if s.stop != nil {
			s.stop()
		}
	})
	s.wg.Wait()
}

func (s *RegistrationReminderService) run() {
	defer s.wg.Done()

	ticker := time.NewTicker(registrationReminderTickInterval)
	defer ticker.Stop()

	s.runOnce()
	for {
		select {
		case <-ticker.C:
			s.runOnce()
		case <-s.stopCtx.Done():
			return
		}
	}
}

func (s *RegistrationReminderService) runOnce() {
	if s == nil || s.settingRepo == nil || s.userService == nil || s.emailSender == nil {
		return
	}

	ctx, cancel := context.WithTimeout(s.stopCtx, 30*time.Second)
	defer cancel()

	enabled, err := s.isRegistrationEnabled(ctx)
	if err != nil || enabled {
		return
	}

	release, ok := s.tryAcquireLeaderLock(ctx)
	if !ok {
		return
	}
	if release != nil {
		defer release()
	}

	now := time.Now()
	if s.loc != nil {
		now = now.In(s.loc)
	}
	if !s.shouldRunNow(ctx, now) {
		return
	}

	recipients := s.resolveRecipients(ctx)
	if len(recipients) == 0 {
		return
	}

	siteName := s.getSiteName(ctx)
	subject := fmt.Sprintf("[%s] 注册仍处于关闭状态", siteName)
	body := buildRegistrationDisabledReminderEmailHTML(siteName, now)
	sent := false
	for _, to := range recipients {
		sendCtx, sendCancel := context.WithTimeout(context.Background(), 20*time.Second)
		err := s.emailSender.SendEmail(sendCtx, to, subject, body)
		sendCancel()
		if err == nil {
			sent = true
		}
	}
	if sent {
		s.setLastRunAt(ctx, registrationReminderReportType, now)
	}
}

func (s *RegistrationReminderService) shouldRunNow(ctx context.Context, now time.Time) bool {
	lastRun := s.getLastRunAt(ctx, registrationReminderReportType)
	return s.shouldRunNowFromLastRun(lastRun, now)
}

func (s *RegistrationReminderService) shouldRunNowFromLastRun(lastRun, now time.Time) bool {
	sched, err := registrationReminderCronParser.Parse(registrationReminderDefaultSchedule)
	if err != nil {
		return false
	}
	base := lastRun
	if base.IsZero() {
		base = now.Add(-1 * time.Minute)
	}
	next := sched.Next(base)
	if next.IsZero() {
		return false
	}
	return !next.After(now)
}

func (s *RegistrationReminderService) isRegistrationEnabled(ctx context.Context) (bool, error) {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyRegistrationEnabled)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(value) == "true", nil
}

func (s *RegistrationReminderService) resolveRecipients(ctx context.Context) []string {
	recipients := s.getAccountQuotaNotifyEmails(ctx)
	if len(recipients) > 0 {
		return recipients
	}
	admin, err := s.userService.GetFirstAdmin(ctx)
	if err != nil || admin == nil {
		return nil
	}
	email := strings.TrimSpace(admin.Email)
	if email == "" {
		return nil
	}
	return []string{email}
}

func (s *RegistrationReminderService) getAccountQuotaNotifyEmails(ctx context.Context) []string {
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyAccountQuotaNotifyEmails)
	if err != nil || strings.TrimSpace(raw) == "" || raw == "[]" {
		return nil
	}
	return filterVerifiedEmails(ParseNotifyEmails(raw))
}

func (s *RegistrationReminderService) getSiteName(ctx context.Context) string {
	name, err := s.settingRepo.GetValue(ctx, SettingKeySiteName)
	if err != nil || strings.TrimSpace(name) == "" {
		return defaultSiteName
	}
	return strings.TrimSpace(name)
}

func (s *RegistrationReminderService) tryAcquireLeaderLock(ctx context.Context) (func(), bool) {
	if s == nil || !s.lockEnabled {
		return nil, true
	}
	if s.redisClient == nil {
		s.warnNoRedis.Do(func() {
			log.Printf("[RegistrationReminder] redis not configured; running without distributed lock")
		})
		return nil, true
	}
	ok, err := s.redisClient.SetNX(ctx, registrationReminderLeaderLockKey, s.instanceID, registrationReminderLockTTL).Result()
	if err != nil {
		log.Printf("[RegistrationReminder] leader lock SetNX failed; skipping this cycle: %v", err)
		return nil, false
	}
	if !ok {
		return nil, false
	}
	return func() {
		_, _ = opsScheduledReportReleaseScript.Run(ctx, s.redisClient, []string{registrationReminderLeaderLockKey}, s.instanceID).Result()
	}, true
}

func (s *RegistrationReminderService) getLastRunAt(ctx context.Context, reportType string) time.Time {
	if s == nil || s.redisClient == nil {
		return time.Time{}
	}
	key := registrationReminderLastRunKeyPrefix + strings.TrimSpace(reportType)
	raw, err := s.redisClient.Get(ctx, key).Result()
	if err != nil || strings.TrimSpace(raw) == "" {
		return time.Time{}
	}
	sec, err := time.Parse(time.RFC3339, strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}
	}
	if s.loc != nil {
		return sec.In(s.loc)
	}
	return sec.UTC()
}

func (s *RegistrationReminderService) setLastRunAt(ctx context.Context, reportType string, t time.Time) {
	if s == nil || s.redisClient == nil {
		return
	}
	if t.IsZero() {
		t = time.Now().UTC()
	}
	key := registrationReminderLastRunKeyPrefix + strings.TrimSpace(reportType)
	_ = s.redisClient.Set(ctx, key, t.UTC().Format(time.RFC3339), 14*24*time.Hour).Err()
}

func buildRegistrationDisabledReminderEmailHTML(siteName string, now time.Time) string {
	return fmt.Sprintf(`
<h2>%s</h2>
<p>中转站当前仍处于关闭注册状态。</p>
<p>The registration gate is still disabled.</p>
<ul>
  <li><b>Site</b>: %s</li>
  <li><b>Reminder Time</b>: %s</li>
  <li><b>Action</b>: 如需重新开放注册，请在后台设置中打开 <code>registration_enabled</code>。</li>
</ul>
`,
		htmlEscape("注册关闭提醒 / Registration Disabled Reminder"),
		htmlEscape(strings.TrimSpace(siteName)),
		htmlEscape(now.UTC().Format(time.RFC3339)),
	)
}
