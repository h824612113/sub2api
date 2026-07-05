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
	if sub == nil || sub.NeedsWeeklyReset() {
		return 0
	}
	return sub.WeeklyUsageUSD
}

func normalizedDailyUsage(sub *UserSubscription) float64 {
	if sub == nil || sub.NeedsDailyReset() {
		return 0
	}
	return sub.DailyUsageUSD
}

func normalizedMonthlyUsage(sub *UserSubscription) float64 {
	if sub == nil || sub.NeedsMonthlyReset() {
		return 0
	}
	return sub.MonthlyUsageUSD
}

func aggregatePooledSubscriptionQuota(ctx context.Context, repo UserSubscriptionRepository, current *UserSubscription, group *Group) (*pooledSubscriptionQuota, error) {
	result := &pooledSubscriptionQuota{}
	if current == nil || group == nil {
		return result, nil
	}

	poolKey := group.QuotaPoolKey()
	if poolKey == "" {
		if group.HasDailyLimit() {
			result.HasDailyPool = true
			result.DailyUsage = normalizedDailyUsage(current)
			result.DailyLimit = *group.DailyLimitUSD
		}
		if group.HasWeeklyLimit() {
			result.HasWeeklyPool = true
			result.WeeklyUsage = normalizedWeeklyUsage(current)
			result.WeeklyLimit = *group.WeeklyLimitUSD
		}
		if group.HasMonthlyLimit() {
			result.HasMonthlyPool = true
			result.MonthlyUsage = normalizedMonthlyUsage(current)
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
		if sub.Group == nil || sub.Group.QuotaPoolKey() != poolKey || !sub.IsActive() {
			continue
		}

		if sub.Group.HasDailyLimit() {
			result.HasDailyPool = true
			result.DailyLimit += *sub.Group.DailyLimitUSD
			result.DailyUsage += normalizedDailyUsage(sub)
		}

		if sub.Group.HasWeeklyLimit() {
			result.HasWeeklyPool = true
			result.WeeklyLimit += *sub.Group.WeeklyLimitUSD
			result.WeeklyUsage += normalizedWeeklyUsage(sub)
		}

		if sub.Group.HasMonthlyLimit() {
			result.HasMonthlyPool = true
			result.MonthlyLimit += *sub.Group.MonthlyLimitUSD
			result.MonthlyUsage += normalizedMonthlyUsage(sub)
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
		result.DailyUsage = normalizedDailyUsage(current)
		result.DailyLimit = *group.DailyLimitUSD
	}
	if !result.HasWeeklyPool && group.HasWeeklyLimit() {
		result.HasWeeklyPool = true
		result.WeeklyUsage = normalizedWeeklyUsage(current)
		result.WeeklyLimit = *group.WeeklyLimitUSD
	}
	if !result.HasMonthlyPool && group.HasMonthlyLimit() {
		result.HasMonthlyPool = true
		result.MonthlyUsage = normalizedMonthlyUsage(current)
		result.MonthlyLimit = *group.MonthlyLimitUSD
	}

	return result, nil
}
