package service

import (
	"context"
	"time"
)

type pooledSubscriptionQuota struct {
	DailyUsage   float64
	DailyLimit   float64
	HasDailyPool bool

	WeeklyUsage   float64
	WeeklyLimit   float64
	HasWeeklyPool bool

	MonthlyUsage   float64
	MonthlyLimit   float64
	HasMonthlyPool bool
}

type pooledSubscriptionWindows struct {
	DailyWindowStart   *time.Time
	WeeklyWindowStart  *time.Time
	MonthlyWindowStart *time.Time
}

func normalizedWeeklyUsage(sub *UserSubscription) float64 {
	return normalizedWeeklyUsageAt(sub, time.Now())
}

func normalizedWeeklyUsageAt(sub *UserSubscription, now time.Time) float64 {
	if sub == nil || sub.canAutomaticallyResetWeeklyAt(now) {
		return 0
	}
	return sub.WeeklyUsageUSD
}

func normalizedDailyUsage(sub *UserSubscription) float64 {
	return normalizedDailyUsageAt(sub, time.Now())
}

func normalizedDailyUsageAt(sub *UserSubscription, now time.Time) float64 {
	if sub == nil || sub.canAutomaticallyResetDailyAt(now) {
		return 0
	}
	return sub.DailyUsageUSD
}

func normalizedMonthlyUsage(sub *UserSubscription) float64 {
	return normalizedMonthlyUsageAt(sub, time.Now())
}

func normalizedMonthlyUsageAt(sub *UserSubscription, now time.Time) float64 {
	if sub == nil || sub.canAutomaticallyResetMonthlyAt(now) {
		return 0
	}
	return sub.MonthlyUsageUSD
}

func aggregatePooledSubscriptionQuota(ctx context.Context, repo UserSubscriptionRepository, current *UserSubscription, group *Group) (*pooledSubscriptionQuota, error) {
	return aggregatePooledSubscriptionQuotaAt(ctx, repo, current, group, time.Now())
}

func aggregatePooledSubscriptionQuotaAt(ctx context.Context, repo UserSubscriptionRepository, current *UserSubscription, group *Group, now time.Time) (*pooledSubscriptionQuota, error) {
	result := &pooledSubscriptionQuota{}
	if current == nil || group == nil {
		return result, nil
	}

	poolKey := group.QuotaPoolKey()
	if poolKey == "" {
		if group.HasDailyLimit() {
			result.HasDailyPool = true
			result.DailyUsage = normalizedDailyUsageAt(current, now)
			result.DailyLimit = *group.DailyLimitUSD
		}
		if group.HasWeeklyLimit() {
			result.HasWeeklyPool = true
			result.WeeklyUsage = normalizedWeeklyUsageAt(current, now)
			result.WeeklyLimit = *group.WeeklyLimitUSD
		}
		if group.HasMonthlyLimit() {
			result.HasMonthlyPool = true
			result.MonthlyUsage = normalizedMonthlyUsageAt(current, now)
			result.MonthlyLimit = *group.MonthlyLimitUSD
		}
		return result, nil
	}

	subs, err := repo.ListActiveByUserID(ctx, current.UserID)
	if err != nil {
		return nil, err
	}

	for i := range subs {
		sub := &subs[i]
		if sub.Group == nil || sub.Group.QuotaPoolKey() != poolKey || sub.Status != SubscriptionStatusActive || !sub.ExpiresAt.After(now) {
			continue
		}

		if sub.Group.HasDailyLimit() {
			result.HasDailyPool = true
			result.DailyLimit += *sub.Group.DailyLimitUSD
			result.DailyUsage += normalizedDailyUsageAt(sub, now)
		}

		if sub.Group.HasWeeklyLimit() {
			result.HasWeeklyPool = true
			result.WeeklyLimit += *sub.Group.WeeklyLimitUSD
			result.WeeklyUsage += normalizedWeeklyUsageAt(sub, now)
		}

		if sub.Group.HasMonthlyLimit() {
			result.HasMonthlyPool = true
			result.MonthlyLimit += *sub.Group.MonthlyLimitUSD
			result.MonthlyUsage += normalizedMonthlyUsageAt(sub, now)
		}
	}

	if fixed := group.QuotaPoolDailyLimit(); fixed != nil {
		result.HasDailyPool = true
		result.DailyLimit = *fixed
	}
	if fixed := group.QuotaPoolWeeklyLimit(); fixed != nil {
		result.HasWeeklyPool = true
		result.WeeklyLimit = *fixed
	}
	if fixed := group.QuotaPoolMonthlyLimit(); fixed != nil {
		result.HasMonthlyPool = true
		result.MonthlyLimit = *fixed
	}

	if !result.HasDailyPool && group.HasDailyLimit() {
		result.HasDailyPool = true
		result.DailyUsage = normalizedDailyUsageAt(current, now)
		result.DailyLimit = *group.DailyLimitUSD
	}
	if !result.HasWeeklyPool && group.HasWeeklyLimit() {
		result.HasWeeklyPool = true
		result.WeeklyUsage = normalizedWeeklyUsageAt(current, now)
		result.WeeklyLimit = *group.WeeklyLimitUSD
	}
	if !result.HasMonthlyPool && group.HasMonthlyLimit() {
		result.HasMonthlyPool = true
		result.MonthlyUsage = normalizedMonthlyUsageAt(current, now)
		result.MonthlyLimit = *group.MonthlyLimitUSD
	}

	return result, nil
}
