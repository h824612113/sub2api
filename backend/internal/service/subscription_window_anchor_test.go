package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAnchoredSubscriptionWindowStart_WeeklyFromSubscriptionStartTime(t *testing.T) {
	loc := time.FixedZone("CST", 8*60*60)
	startsAt := time.Date(2026, 6, 11, 12, 0, 0, 0, loc)
	now := time.Date(2026, 6, 29, 21, 0, 0, 0, loc)

	got := anchoredSubscriptionWindowStart(startsAt, now, 7)

	require.True(t, got.Equal(time.Date(2026, 6, 25, 12, 0, 0, 0, loc)))
}

func TestUserSubscriptionNeedsWeeklyReset_ManualResetInsideCurrentAnchoredWindow(t *testing.T) {
	loc := time.FixedZone("CST", 8*60*60)
	startsAt := time.Date(2026, 5, 29, 21, 54, 54, 0, loc)
	manualResetWindow := time.Date(2026, 6, 28, 21, 48, 0, 0, loc)
	sub := &UserSubscription{
		StartsAt:          startsAt,
		WeeklyWindowStart: &manualResetWindow,
	}

	require.False(t, sub.NeedsWeeklyResetAt(time.Date(2026, 6, 29, 21, 48, 0, 0, loc)))
	require.False(t, sub.NeedsWeeklyResetAt(time.Date(2026, 7, 3, 21, 0, 0, 0, loc)))
	require.True(t, sub.NeedsWeeklyResetAt(time.Date(2026, 7, 3, 22, 0, 0, 0, loc)))
}

func TestUserSubscriptionNeedsMonthlyReset_AnchoredEveryThirtyDays(t *testing.T) {
	loc := time.FixedZone("CST", 8*60*60)
	startsAt := time.Date(2026, 5, 29, 21, 54, 54, 0, loc)
	oldWindow := startsAt
	sub := &UserSubscription{
		StartsAt:           startsAt,
		MonthlyWindowStart: &oldWindow,
	}

	require.True(t, sub.NeedsMonthlyResetAt(time.Date(2026, 6, 29, 12, 0, 0, 0, loc)))
}
