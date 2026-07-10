//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/dgraph-io/ristretto"
	"github.com/stretchr/testify/require"
)

func TestUsageService_InvalidateUsageCaches(t *testing.T) {
	invalidator := &authCacheInvalidatorStub{}
	svc := &UsageService{authCacheInvalidator: invalidator}

	svc.invalidateUsageCaches(context.Background(), 7, false)
	require.Empty(t, invalidator.userIDs)

	svc.invalidateUsageCaches(context.Background(), 7, true)
	require.Equal(t, []int64{7}, invalidator.userIDs)
}

func TestRedeemService_InvalidateRedeemCaches_AuthCache(t *testing.T) {
	invalidator := &authCacheInvalidatorStub{}
	svc := &RedeemService{authCacheInvalidator: invalidator}

	svc.invalidateRedeemCaches(context.Background(), 11, &RedeemCode{Type: RedeemTypeBalance})
	svc.invalidateRedeemCaches(context.Background(), 11, &RedeemCode{Type: RedeemTypeConcurrency})
	groupID := int64(3)
	svc.invalidateRedeemCaches(context.Background(), 11, &RedeemCode{Type: RedeemTypeSubscription, GroupID: &groupID})

	require.Equal(t, []int64{11, 11, 11}, invalidator.userIDs)
}

func TestRedeemService_InvalidateRedeemCaches_SubscriptionBundleL1AfterCommit(t *testing.T) {
	cache, err := ristretto.NewCache(&ristretto.Config{NumCounters: 1_000, MaxCost: 100, BufferItems: 64})
	require.NoError(t, err)
	t.Cleanup(cache.Close)

	const userID int64 = 11
	const primaryGroupID int64 = 3
	const siblingGroupID int64 = 4
	subscriptionService := &SubscriptionService{
		subCacheL1: cache,
		groupRepo: &subscriptionGroupRepoByIDStub{groups: map[int64]*Group{
			primaryGroupID: {
				ID:               primaryGroupID,
				Status:           StatusActive,
				SubscriptionType: SubscriptionTypeSubscription,
				Description:      "subscription_bundle_groups=3,4",
			},
			siblingGroupID: {
				ID:               siblingGroupID,
				Status:           StatusActive,
				SubscriptionType: SubscriptionTypeSubscription,
				Description:      "subscription_bundle_groups=3,4",
			},
		}},
	}

	require.True(t, cache.Set(subCacheKey(userID, primaryGroupID), &UserSubscription{ID: 101}, 1))
	require.True(t, cache.Set(subCacheKey(userID, siblingGroupID), &UserSubscription{ID: 102}, 1))
	cache.Wait()

	svc := &RedeemService{subscriptionService: subscriptionService}
	groupID := primaryGroupID
	svc.invalidateRedeemCaches(context.Background(), userID, &RedeemCode{Type: RedeemTypeSubscription, GroupID: &groupID})

	_, primaryCached := cache.Get(subCacheKey(userID, primaryGroupID))
	_, siblingCached := cache.Get(subCacheKey(userID, siblingGroupID))
	require.False(t, primaryCached)
	require.False(t, siblingCached)
}
