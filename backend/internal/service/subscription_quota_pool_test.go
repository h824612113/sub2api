package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type quotaPoolUserSubRepoStub struct {
	userSubRepoNoop

	activeByUser  map[int64][]UserSubscription
	activeByGroup map[int64]*UserSubscription
	listSubs      []UserSubscription
}

func (s *quotaPoolUserSubRepoStub) ListActiveByUserID(_ context.Context, userID int64) ([]UserSubscription, error) {
	subs := s.activeByUser[userID]
	out := make([]UserSubscription, len(subs))
	copy(out, subs)
	return out, nil
}

func (s *quotaPoolUserSubRepoStub) GetActiveByUserIDAndGroupID(_ context.Context, _, groupID int64) (*UserSubscription, error) {
	sub := s.activeByGroup[groupID]
	if sub == nil {
		return nil, ErrSubscriptionNotFound
	}
	cp := *sub
	return &cp, nil
}

func (s *quotaPoolUserSubRepoStub) List(_ context.Context, params pagination.PaginationParams, _, _ *int64, _, _, _, _ string) ([]UserSubscription, *pagination.PaginationResult, error) {
	subs := s.listSubs
	if subs == nil {
		for _, userSubs := range s.activeByUser {
			subs = append(subs, userSubs...)
		}
	}
	out := make([]UserSubscription, len(subs))
	copy(out, subs)
	return out, &pagination.PaginationResult{
		Total:    int64(len(out)),
		Page:     params.Page,
		PageSize: params.PageSize,
		Pages:    1,
	}, nil
}

func quotaPoolSub(userID, groupID int64, group *Group, dailyUsage, weeklyUsage, monthlyUsage float64) UserSubscription {
	windowStart := time.Now().Add(-time.Hour)
	return UserSubscription{
		UserID:             userID,
		GroupID:            groupID,
		Status:             SubscriptionStatusActive,
		ExpiresAt:          time.Now().Add(24 * time.Hour),
		DailyWindowStart:   &windowStart,
		WeeklyWindowStart:  &windowStart,
		MonthlyWindowStart: &windowStart,
		DailyUsageUSD:      dailyUsage,
		WeeklyUsageUSD:     weeklyUsage,
		MonthlyUsageUSD:    monthlyUsage,
		Group:              group,
	}
}

func TestGroupQuotaPoolKey(t *testing.T) {
	require.Equal(t, "", (*Group)(nil).QuotaPoolKey())
	require.Equal(t, "shared", (&Group{Description: "foo\n quota_pool=shared \nbar"}).QuotaPoolKey())
	require.Equal(t, "legacy", (&Group{Description: "sub2api.quota_pool=legacy"}).QuotaPoolKey())
	require.Equal(t, "", (&Group{Description: "foo\nbar"}).QuotaPoolKey())
}

func TestAggregatePooledSubscriptionQuota_NoPoolUsesCurrentSubscriptionLimits(t *testing.T) {
	group := &Group{
		WeeklyLimitUSD:  ptrFloat64QuotaPool(10),
		MonthlyLimitUSD: ptrFloat64QuotaPool(100),
	}
	sub := quotaPoolSub(1, 10, group, 1, 6, 30)
	staleWeekly := time.Now().Add(-8 * 24 * time.Hour)
	sub.WeeklyWindowStart = &staleWeekly

	quota, err := aggregatePooledSubscriptionQuota(context.Background(), userSubRepoNoop{}, &sub, group)
	require.NoError(t, err)
	require.True(t, quota.HasWeeklyPool)
	require.Equal(t, 10.0, quota.WeeklyLimit)
	require.Zero(t, quota.WeeklyUsage)
	require.True(t, quota.HasMonthlyPool)
	require.Equal(t, 100.0, quota.MonthlyLimit)
	require.Equal(t, 30.0, quota.MonthlyUsage)
}

func TestAggregatePooledSubscriptionQuota_SumsOnlyMatchingWindowPools(t *testing.T) {
	groupA := &Group{
		ID:              10,
		Description:     "quota_pool=openai_shared",
		WeeklyLimitUSD:  ptrFloat64QuotaPool(5),
		MonthlyLimitUSD: ptrFloat64QuotaPool(50),
	}
	groupB := &Group{
		ID:              11,
		Description:     "quota_pool=openai_shared",
		WeeklyLimitUSD:  ptrFloat64QuotaPool(5),
		MonthlyLimitUSD: ptrFloat64QuotaPool(50),
	}
	groupMonthlyOnly := &Group{
		ID:              12,
		Description:     "quota_pool=openai_shared",
		MonthlyLimitUSD: ptrFloat64QuotaPool(50),
	}
	groupOther := &Group{
		ID:              13,
		Description:     "quota_pool=other_pool",
		WeeklyLimitUSD:  ptrFloat64QuotaPool(99),
		MonthlyLimitUSD: ptrFloat64QuotaPool(999),
	}

	current := quotaPoolSub(1, 10, groupA, 1, 6, 40)
	second := quotaPoolSub(1, 11, groupB, 1, 1, 20)
	monthlyOnly := quotaPoolSub(1, 12, groupMonthlyOnly, 1, 9, 30)
	staleWeekly := quotaPoolSub(1, 14, groupB, 1, 7, 70)
	staleAt := time.Now().Add(-8 * 24 * time.Hour)
	staleWeekly.WeeklyWindowStart = &staleAt
	otherPool := quotaPoolSub(1, 13, groupOther, 1, 50, 500)

	repo := &quotaPoolUserSubRepoStub{
		activeByUser: map[int64][]UserSubscription{
			1: {current, second, monthlyOnly, staleWeekly, otherPool},
		},
	}

	quota, err := aggregatePooledSubscriptionQuota(context.Background(), repo, &current, groupA)
	require.NoError(t, err)
	require.True(t, quota.HasWeeklyPool)
	require.Equal(t, 15.0, quota.WeeklyLimit)
	require.Equal(t, 7.0, quota.WeeklyUsage)
	require.True(t, quota.HasMonthlyPool)
	require.Equal(t, 200.0, quota.MonthlyLimit)
	require.Equal(t, 160.0, quota.MonthlyUsage)
}

func TestSubscriptionServiceList_NormalizesPooledDisplayPerUser(t *testing.T) {
	groupA := &Group{
		ID:              10,
		Description:     "quota_pool=openai_shared\nquota_pool_weekly_limit=150\nquota_pool_monthly_limit=600",
		WeeklyLimitUSD:  ptrFloat64QuotaPool(150),
		MonthlyLimitUSD: ptrFloat64QuotaPool(600),
	}
	groupB := &Group{
		ID:              11,
		Description:     "quota_pool=openai_shared\nquota_pool_weekly_limit=150\nquota_pool_monthly_limit=600",
		WeeklyLimitUSD:  ptrFloat64QuotaPool(150),
		MonthlyLimitUSD: ptrFloat64QuotaPool(600),
	}

	userOneMain := quotaPoolSub(1, 10, groupA, 0, 6.54, 126.35)
	userOneSibling := quotaPoolSub(1, 11, groupB, 0, 0, 0)
	userTwoMain := quotaPoolSub(2, 10, groupA, 0, 0, 31.84)
	userTwoSibling := quotaPoolSub(2, 11, groupB, 0, 0, 0)
	repo := &quotaPoolUserSubRepoStub{
		activeByUser: map[int64][]UserSubscription{
			1: {userOneMain, userOneSibling},
			2: {userTwoMain, userTwoSibling},
		},
		listSubs: []UserSubscription{userOneSibling, userTwoMain},
	}
	svc := NewSubscriptionService(nil, repo, nil, nil, nil)

	subs, _, err := svc.List(context.Background(), 1, 20, nil, nil, "", "", "", "")
	require.NoError(t, err)
	require.Len(t, subs, 2)

	byUserGroup := make(map[[2]int64]UserSubscription, len(subs))
	for _, sub := range subs {
		byUserGroup[[2]int64{sub.UserID, sub.GroupID}] = sub
	}

	require.Equal(t, 6.54, byUserGroup[[2]int64{1, 11}].WeeklyUsageUSD)
	require.Equal(t, 126.35, byUserGroup[[2]int64{1, 11}].MonthlyUsageUSD)

	_, hasUserOneMain := byUserGroup[[2]int64{1, 10}]
	require.False(t, hasUserOneMain)

	require.Equal(t, 31.84, byUserGroup[[2]int64{2, 10}].MonthlyUsageUSD)
	require.Zero(t, byUserGroup[[2]int64{2, 10}].WeeklyUsageUSD)

	require.Equal(t, 150.0, *byUserGroup[[2]int64{1, 11}].Group.WeeklyLimitUSD)
	require.Equal(t, 600.0, *byUserGroup[[2]int64{1, 11}].Group.MonthlyLimitUSD)
}

func TestSubscriptionServiceValidateAndCheckLimits_AllowsIndividualWeeklyOverageWithinPool(t *testing.T) {
	groupA := &Group{
		ID:              10,
		DailyLimitUSD:   ptrFloat64QuotaPool(5),
		WeeklyLimitUSD:  ptrFloat64QuotaPool(5),
		MonthlyLimitUSD: ptrFloat64QuotaPool(50),
		Description:     "quota_pool=openai_shared",
	}
	groupB := &Group{
		ID:              11,
		WeeklyLimitUSD:  ptrFloat64QuotaPool(5),
		MonthlyLimitUSD: ptrFloat64QuotaPool(50),
		Description:     "quota_pool=openai_shared",
	}

	current := quotaPoolSub(1, 10, groupA, 1, 6, 40)
	second := quotaPoolSub(1, 11, groupB, 0, 1, 10)
	repo := &quotaPoolUserSubRepoStub{
		activeByUser: map[int64][]UserSubscription{
			1: {current, second},
		},
	}
	svc := NewSubscriptionService(nil, repo, nil, nil, nil)

	needsMaintenance, err := svc.ValidateAndCheckLimits(context.Background(), &current, groupA)
	require.NoError(t, err)
	require.False(t, needsMaintenance)
}

func TestSubscriptionServiceValidateAndCheckLimits_RejectsPooledWeeklyOverage(t *testing.T) {
	groupA := &Group{
		ID:              10,
		DailyLimitUSD:   ptrFloat64QuotaPool(5),
		WeeklyLimitUSD:  ptrFloat64QuotaPool(5),
		MonthlyLimitUSD: ptrFloat64QuotaPool(50),
		Description:     "quota_pool=openai_shared",
	}
	groupB := &Group{
		ID:              11,
		WeeklyLimitUSD:  ptrFloat64QuotaPool(5),
		MonthlyLimitUSD: ptrFloat64QuotaPool(50),
		Description:     "quota_pool=openai_shared",
	}

	current := quotaPoolSub(1, 10, groupA, 1, 6, 40)
	second := quotaPoolSub(1, 11, groupB, 0, 5, 10)
	repo := &quotaPoolUserSubRepoStub{
		activeByUser: map[int64][]UserSubscription{
			1: {current, second},
		},
	}
	svc := NewSubscriptionService(nil, repo, nil, nil, nil)

	_, err := svc.ValidateAndCheckLimits(context.Background(), &current, groupA)
	require.ErrorIs(t, err, ErrWeeklyLimitExceeded)
}

func TestBillingCacheServiceCheckSubscriptionEligibility_AllowsIndividualWeeklyOverageWithinPool(t *testing.T) {
	groupA := &Group{
		ID:              10,
		DailyLimitUSD:   ptrFloat64QuotaPool(5),
		WeeklyLimitUSD:  ptrFloat64QuotaPool(5),
		MonthlyLimitUSD: ptrFloat64QuotaPool(50),
		Description:     "quota_pool=openai_shared",
	}
	groupB := &Group{
		ID:              11,
		WeeklyLimitUSD:  ptrFloat64QuotaPool(5),
		MonthlyLimitUSD: ptrFloat64QuotaPool(50),
		Description:     "quota_pool=openai_shared",
	}

	current := quotaPoolSub(1, 10, groupA, 1, 6, 40)
	second := quotaPoolSub(1, 11, groupB, 0, 1, 10)
	repo := &quotaPoolUserSubRepoStub{
		activeByUser: map[int64][]UserSubscription{
			1: {current, second},
		},
		activeByGroup: map[int64]*UserSubscription{
			10: &current,
		},
	}
	svc := NewBillingCacheService(nil, nil, repo, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(svc.Stop)

	err := svc.checkSubscriptionEligibility(context.Background(), 1, groupA, &current)
	require.NoError(t, err)
}

func ptrFloat64QuotaPool(v float64) *float64 {
	return &v
}
