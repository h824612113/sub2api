package service

import (
	"context"
	"strconv"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/dgraph-io/ristretto"
	"github.com/stretchr/testify/require"
)

func TestWithSubscriptionUpdateTx_ReusesExistingTransaction(t *testing.T) {
	existingTx := &dbent.Tx{}
	ctx := dbent.NewTxContext(context.Background(), existingTx)
	svc := &SubscriptionService{entClient: &dbent.Client{}}

	called := false
	err := svc.withSubscriptionUpdateTx(ctx, func(txCtx context.Context) error {
		called = true
		require.Same(t, existingTx, dbent.TxFromContext(txCtx))
		return nil
	})

	require.NoError(t, err)
	require.True(t, called)
}

func TestMaybeInvalidateAssignmentCaches_DefersForOuterTransactionOwner(t *testing.T) {
	cache, err := ristretto.NewCache(&ristretto.Config{NumCounters: 1_000, MaxCost: 100, BufferItems: 64})
	require.NoError(t, err)
	t.Cleanup(cache.Close)

	svc := &SubscriptionService{subCacheL1: cache}
	key := subCacheKey(7, 9)
	require.True(t, cache.Set(key, &UserSubscription{ID: 42}, 1))
	cache.Wait()

	svc.maybeInvalidateAssignmentCaches(7, 9, true)
	_, cachedBeforeCommit := cache.Get(key)
	require.True(t, cachedBeforeCommit, "outer transaction must retain caches until its owner commits")

	svc.maybeInvalidateAssignmentCaches(7, 9, false)
	cache.Wait()
	_, cachedAfterCommit := cache.Get(key)
	require.False(t, cachedAfterCommit, "post-commit invalidation must remove the cached subscription")
}

type groupRepoNoop struct{}

func (groupRepoNoop) Create(context.Context, *Group) error { panic("unexpected Create call") }
func (groupRepoNoop) GetByID(context.Context, int64) (*Group, error) {
	panic("unexpected GetByID call")
}
func (groupRepoNoop) GetByIDLite(context.Context, int64) (*Group, error) {
	panic("unexpected GetByIDLite call")
}
func (groupRepoNoop) Update(context.Context, *Group) error { panic("unexpected Update call") }
func (groupRepoNoop) Delete(context.Context, int64) error  { panic("unexpected Delete call") }
func (groupRepoNoop) DeleteCascade(context.Context, int64) ([]int64, error) {
	panic("unexpected DeleteCascade call")
}
func (groupRepoNoop) List(context.Context, pagination.PaginationParams) ([]Group, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}
func (groupRepoNoop) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string, *bool) ([]Group, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}
func (groupRepoNoop) ListActive(context.Context) ([]Group, error) {
	panic("unexpected ListActive call")
}
func (groupRepoNoop) ListActiveByPlatform(context.Context, string) ([]Group, error) {
	panic("unexpected ListActiveByPlatform call")
}
func (groupRepoNoop) ExistsByName(context.Context, string) (bool, error) {
	panic("unexpected ExistsByName call")
}
func (groupRepoNoop) GetAccountCount(context.Context, int64) (int64, int64, error) {
	panic("unexpected GetAccountCount call")
}
func (groupRepoNoop) DeleteAccountGroupsByGroupID(context.Context, int64) (int64, error) {
	panic("unexpected DeleteAccountGroupsByGroupID call")
}
func (groupRepoNoop) GetAccountIDsByGroupIDs(context.Context, []int64) ([]int64, error) {
	panic("unexpected GetAccountIDsByGroupIDs call")
}
func (groupRepoNoop) BindAccountsToGroup(context.Context, int64, []int64) error {
	panic("unexpected BindAccountsToGroup call")
}
func (groupRepoNoop) UpdateSortOrders(context.Context, []GroupSortOrderUpdate) error {
	panic("unexpected UpdateSortOrders call")
}

type subscriptionGroupRepoStub struct {
	groupRepoNoop
	group *Group
}

func (s *subscriptionGroupRepoStub) GetByID(context.Context, int64) (*Group, error) {
	return s.group, nil
}

type subscriptionGroupRepoByIDStub struct {
	groupRepoNoop
	groups map[int64]*Group
}

func (s *subscriptionGroupRepoByIDStub) GetByID(_ context.Context, id int64) (*Group, error) {
	group := s.groups[id]
	if group == nil {
		return nil, ErrGroupNotFound
	}
	cp := *group
	return &cp, nil
}

type userSubRepoNoop struct{}

func (userSubRepoNoop) Create(context.Context, *UserSubscription) error {
	panic("unexpected Create call")
}
func (userSubRepoNoop) GetByID(context.Context, int64) (*UserSubscription, error) {
	panic("unexpected GetByID call")
}
func (userSubRepoNoop) GetByIDIncludeDeleted(context.Context, int64) (*UserSubscription, error) {
	panic("unexpected GetByIDIncludeDeleted call")
}
func (userSubRepoNoop) GetByUserIDAndGroupID(context.Context, int64, int64) (*UserSubscription, error) {
	panic("unexpected GetByUserIDAndGroupID call")
}
func (userSubRepoNoop) GetActiveByUserIDAndGroupID(context.Context, int64, int64) (*UserSubscription, error) {
	panic("unexpected GetActiveByUserIDAndGroupID call")
}
func (userSubRepoNoop) Update(context.Context, *UserSubscription) error {
	panic("unexpected Update call")
}
func (userSubRepoNoop) Delete(context.Context, int64) error { panic("unexpected Delete call") }
func (userSubRepoNoop) Restore(context.Context, int64, string) (*UserSubscription, error) {
	panic("unexpected Restore call")
}
func (userSubRepoNoop) ListByUserID(context.Context, int64) ([]UserSubscription, error) {
	panic("unexpected ListByUserID call")
}
func (userSubRepoNoop) ListActiveByUserID(context.Context, int64) ([]UserSubscription, error) {
	panic("unexpected ListActiveByUserID call")
}
func (userSubRepoNoop) ListByGroupID(context.Context, int64, pagination.PaginationParams) ([]UserSubscription, *pagination.PaginationResult, error) {
	panic("unexpected ListByGroupID call")
}
func (userSubRepoNoop) List(context.Context, pagination.PaginationParams, *int64, *int64, string, string, string, string) ([]UserSubscription, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}
func (userSubRepoNoop) ExistsByUserIDAndGroupID(context.Context, int64, int64) (bool, error) {
	panic("unexpected ExistsByUserIDAndGroupID call")
}
func (userSubRepoNoop) ExistsActiveByUserIDAndGroupID(context.Context, int64, int64) (bool, error) {
	panic("unexpected ExistsActiveByUserIDAndGroupID call")
}
func (userSubRepoNoop) ExtendExpiry(context.Context, int64, time.Time) error {
	panic("unexpected ExtendExpiry call")
}
func (userSubRepoNoop) UpdateStatus(context.Context, int64, string) error {
	panic("unexpected UpdateStatus call")
}
func (userSubRepoNoop) UpdateNotes(context.Context, int64, string) error {
	panic("unexpected UpdateNotes call")
}
func (userSubRepoNoop) ActivateWindows(context.Context, int64, time.Time) error {
	panic("unexpected ActivateWindows call")
}
func (userSubRepoNoop) ResetUsageWindows(context.Context, int64, bool, bool, bool, time.Time) error {
	panic("unexpected ResetUsageWindows call")
}
func (userSubRepoNoop) ResetDailyUsage(context.Context, int64, *time.Time, time.Time) error {
	panic("unexpected ResetDailyUsage call")
}
func (userSubRepoNoop) ResetWeeklyUsage(context.Context, int64, *time.Time, time.Time) error {
	panic("unexpected ResetWeeklyUsage call")
}
func (userSubRepoNoop) ResetMonthlyUsage(context.Context, int64, *time.Time, time.Time) error {
	panic("unexpected ResetMonthlyUsage call")
}
func (userSubRepoNoop) IncrementUsage(context.Context, int64, float64) error {
	panic("unexpected IncrementUsage call")
}
func (userSubRepoNoop) BatchUpdateExpiredStatus(context.Context) (int64, error) {
	panic("unexpected BatchUpdateExpiredStatus call")
}

type subscriptionUserSubRepoStub struct {
	userSubRepoNoop

	nextID      int64
	byID        map[int64]*UserSubscription
	byUserGroup map[string]*UserSubscription
	createCalls int
	deletedIDs  []int64
}

func newSubscriptionUserSubRepoStub() *subscriptionUserSubRepoStub {
	return &subscriptionUserSubRepoStub{
		nextID:      1,
		byID:        make(map[int64]*UserSubscription),
		byUserGroup: make(map[string]*UserSubscription),
	}
}

func (s *subscriptionUserSubRepoStub) key(userID, groupID int64) string {
	return strconvFormatInt(userID) + ":" + strconvFormatInt(groupID)
}

func (s *subscriptionUserSubRepoStub) seed(sub *UserSubscription) {
	if sub == nil {
		return
	}
	cp := *sub
	if cp.ID == 0 {
		cp.ID = s.nextID
		s.nextID++
	}
	s.byID[cp.ID] = &cp
	s.byUserGroup[s.key(cp.UserID, cp.GroupID)] = &cp
}

func (s *subscriptionUserSubRepoStub) ExistsByUserIDAndGroupID(_ context.Context, userID, groupID int64) (bool, error) {
	_, ok := s.byUserGroup[s.key(userID, groupID)]
	return ok, nil
}

func (s *subscriptionUserSubRepoStub) GetByUserIDAndGroupID(_ context.Context, userID, groupID int64) (*UserSubscription, error) {
	sub := s.byUserGroup[s.key(userID, groupID)]
	if sub == nil {
		return nil, ErrSubscriptionNotFound
	}
	cp := *sub
	return &cp, nil
}

func (s *subscriptionUserSubRepoStub) Create(_ context.Context, sub *UserSubscription) error {
	if sub == nil {
		return nil
	}
	s.createCalls++
	cp := *sub
	if cp.ID == 0 {
		cp.ID = s.nextID
		s.nextID++
	}
	sub.ID = cp.ID
	s.byID[cp.ID] = &cp
	s.byUserGroup[s.key(cp.UserID, cp.GroupID)] = &cp
	return nil
}

func (s *subscriptionUserSubRepoStub) GetByID(_ context.Context, id int64) (*UserSubscription, error) {
	sub := s.byID[id]
	if sub == nil {
		return nil, ErrSubscriptionNotFound
	}
	cp := *sub
	return &cp, nil
}

func (s *subscriptionUserSubRepoStub) Delete(_ context.Context, id int64) error {
	sub := s.byID[id]
	if sub == nil {
		return nil
	}
	delete(s.byID, id)
	delete(s.byUserGroup, s.key(sub.UserID, sub.GroupID))
	s.deletedIDs = append(s.deletedIDs, id)
	return nil
}

func (s *subscriptionUserSubRepoStub) ExtendExpiry(_ context.Context, subscriptionID int64, newExpiresAt time.Time) error {
	sub := s.byID[subscriptionID]
	if sub == nil {
		return ErrSubscriptionNotFound
	}
	sub.ExpiresAt = newExpiresAt
	return nil
}

func (s *subscriptionUserSubRepoStub) UpdateStatus(_ context.Context, subscriptionID int64, status string) error {
	sub := s.byID[subscriptionID]
	if sub == nil {
		return ErrSubscriptionNotFound
	}
	sub.Status = status
	return nil
}

func (s *subscriptionUserSubRepoStub) UpdateNotes(_ context.Context, subscriptionID int64, notes string) error {
	sub := s.byID[subscriptionID]
	if sub == nil {
		return ErrSubscriptionNotFound
	}
	sub.Notes = notes
	return nil
}

func (s *subscriptionUserSubRepoStub) Update(_ context.Context, sub *UserSubscription) error {
	if sub == nil {
		return ErrSubscriptionNilInput
	}
	existing := s.byID[sub.ID]
	if existing == nil {
		return ErrSubscriptionNotFound
	}
	oldKey := s.key(existing.UserID, existing.GroupID)
	cp := *sub
	s.byID[cp.ID] = &cp
	if oldKey != s.key(cp.UserID, cp.GroupID) {
		delete(s.byUserGroup, oldKey)
	}
	s.byUserGroup[s.key(cp.UserID, cp.GroupID)] = &cp
	return nil
}

func TestAssignSubscriptionReuseWhenSemanticsMatch(t *testing.T) {
	start := time.Now().UTC().Add(-24 * time.Hour)
	groupRepo := &subscriptionGroupRepoStub{
		group: &Group{ID: 1, SubscriptionType: SubscriptionTypeSubscription},
	}
	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID:        10,
		UserID:    1001,
		GroupID:   1,
		StartsAt:  start,
		ExpiresAt: start.AddDate(0, 0, 30),
		Notes:     "init",
	})

	svc := NewSubscriptionService(groupRepo, subRepo, nil, nil, nil)
	sub, err := svc.AssignSubscription(context.Background(), &AssignSubscriptionInput{
		UserID:       1001,
		GroupID:      1,
		ValidityDays: 30,
		Notes:        "init",
	})
	require.NoError(t, err)
	require.Equal(t, int64(10), sub.ID)
	require.Equal(t, 0, subRepo.createCalls, "reuse should not create new subscription")
}

func TestAssignSubscriptionConflictWhenSemanticsMismatch(t *testing.T) {
	start := time.Now().UTC().Add(-24 * time.Hour)
	groupRepo := &subscriptionGroupRepoStub{
		group: &Group{ID: 1, SubscriptionType: SubscriptionTypeSubscription},
	}
	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID:        11,
		UserID:    2001,
		GroupID:   1,
		StartsAt:  start,
		ExpiresAt: start.AddDate(0, 0, 30),
		Notes:     "old-note",
	})

	svc := NewSubscriptionService(groupRepo, subRepo, nil, nil, nil)
	_, err := svc.AssignSubscription(context.Background(), &AssignSubscriptionInput{
		UserID:       2001,
		GroupID:      1,
		ValidityDays: 30,
		Notes:        "new-note",
	})
	require.Error(t, err)
	require.Equal(t, "SUBSCRIPTION_ASSIGN_CONFLICT", infraerrorsReason(err))
	require.Equal(t, 0, subRepo.createCalls, "conflict should not create or mutate existing subscription")
}

func TestAssignSubscriptionRenewsExpiredExistingSubscription(t *testing.T) {
	groupRepo := &subscriptionGroupRepoStub{
		group: &Group{ID: 1, SubscriptionType: SubscriptionTypeSubscription},
	}
	subRepo := newSubscriptionUserSubRepoStub()
	expiredAt := time.Now().Add(-48 * time.Hour).UTC()
	oldWindowStart := startOfDay(expiredAt.AddDate(0, 0, -20))
	subRepo.seed(&UserSubscription{
		ID:                 12,
		UserID:             3001,
		GroupID:            1,
		StartsAt:           expiredAt.AddDate(0, 0, -30),
		ExpiresAt:          expiredAt,
		Status:             SubscriptionStatusExpired,
		DailyWindowStart:   &oldWindowStart,
		WeeklyWindowStart:  &oldWindowStart,
		MonthlyWindowStart: &oldWindowStart,
		DailyUsageUSD:      10,
		WeeklyUsageUSD:     20,
		MonthlyUsageUSD:    30,
		Notes:              "old-note",
	})

	svc := NewSubscriptionService(groupRepo, subRepo, nil, nil, nil)
	before := time.Now()
	sub, err := svc.AssignSubscription(context.Background(), &AssignSubscriptionInput{
		UserID:       3001,
		GroupID:      1,
		ValidityDays: 30,
		Notes:        "new-note",
	})
	after := time.Now()

	require.NoError(t, err)
	require.Equal(t, int64(12), sub.ID)
	require.Equal(t, SubscriptionStatusActive, sub.Status)
	require.Equal(t, 0, subRepo.createCalls, "renewing expired subscription should not create a new row")
	require.True(t, sub.StartsAt.After(before) || sub.StartsAt.Equal(before), "renewed start should move forward from now")
	require.True(t, sub.StartsAt.Before(after) || sub.StartsAt.Equal(after), "renewed start should be based on current time")
	require.True(t, sub.ExpiresAt.After(before.AddDate(0, 0, 29)), "renewed expiry should move forward from now")
	require.True(t, sub.ExpiresAt.Before(after.AddDate(0, 0, 31)), "renewed expiry should be based on current time")
	require.NotNil(t, sub.DailyWindowStart)
	require.NotNil(t, sub.WeeklyWindowStart)
	require.NotNil(t, sub.MonthlyWindowStart)
	require.Equal(t, sub.StartsAt, *sub.DailyWindowStart)
	require.Equal(t, sub.StartsAt, *sub.WeeklyWindowStart)
	require.Equal(t, sub.StartsAt, *sub.MonthlyWindowStart)
	require.Equal(t, 0.0, sub.DailyUsageUSD)
	require.Equal(t, 0.0, sub.WeeklyUsageUSD)
	require.Equal(t, 0.0, sub.MonthlyUsageUSD)
	require.Equal(t, "old-note\nnew-note", sub.Notes)
}

func TestBulkAssignSubscriptionCreatedReusedAndConflict(t *testing.T) {
	start := time.Now().UTC().Add(-24 * time.Hour)
	groupRepo := &subscriptionGroupRepoStub{
		group: &Group{ID: 1, SubscriptionType: SubscriptionTypeSubscription},
	}
	subRepo := newSubscriptionUserSubRepoStub()
	// user 1: 语义一致，可 reused
	subRepo.seed(&UserSubscription{
		ID:        21,
		UserID:    1,
		GroupID:   1,
		StartsAt:  start,
		ExpiresAt: start.AddDate(0, 0, 30),
		Notes:     "same-note",
	})
	// user 3: 语义冲突（有效期不一致），应 failed
	subRepo.seed(&UserSubscription{
		ID:        23,
		UserID:    3,
		GroupID:   1,
		StartsAt:  start,
		ExpiresAt: start.AddDate(0, 0, 60),
		Notes:     "same-note",
	})

	svc := NewSubscriptionService(groupRepo, subRepo, nil, nil, nil)
	result, err := svc.BulkAssignSubscription(context.Background(), &BulkAssignSubscriptionInput{
		UserIDs:      []int64{1, 2, 3},
		GroupID:      1,
		ValidityDays: 30,
		AssignedBy:   9,
		Notes:        "same-note",
	})
	require.NoError(t, err)
	require.Equal(t, 2, result.SuccessCount)
	require.Equal(t, 1, result.CreatedCount)
	require.Equal(t, 1, result.ReusedCount)
	require.Equal(t, 1, result.FailedCount)
	require.Equal(t, "reused", result.Statuses[1])
	require.Equal(t, "created", result.Statuses[2])
	require.Equal(t, "failed", result.Statuses[3])
	require.Equal(t, 1, subRepo.createCalls)
}

func TestAssignSubscriptionKeepsWorkingWhenIdempotencyStoreUnavailable(t *testing.T) {
	groupRepo := &subscriptionGroupRepoStub{
		group: &Group{ID: 1, SubscriptionType: SubscriptionTypeSubscription},
	}
	subRepo := newSubscriptionUserSubRepoStub()
	SetDefaultIdempotencyCoordinator(NewIdempotencyCoordinator(failingIdempotencyRepo{}, DefaultIdempotencyConfig()))
	t.Cleanup(func() {
		SetDefaultIdempotencyCoordinator(nil)
	})

	svc := NewSubscriptionService(groupRepo, subRepo, nil, nil, nil)
	sub, err := svc.AssignSubscription(context.Background(), &AssignSubscriptionInput{
		UserID:       9001,
		GroupID:      1,
		ValidityDays: 30,
		Notes:        "new",
	})
	require.NoError(t, err)
	require.NotNil(t, sub)
	require.Equal(t, 1, subRepo.createCalls, "semantic idempotent endpoint should not depend on idempotency store availability")
}

func TestExtendSubscriptionBundleExtendsSiblingSubscription(t *testing.T) {
	start := time.Now().UTC().Add(-24 * time.Hour)
	expiresAt := start.AddDate(0, 0, 30)
	groupRepo := &subscriptionGroupRepoByIDStub{groups: map[int64]*Group{
		5:  {ID: 5, SubscriptionType: SubscriptionTypeSubscription, Status: StatusActive, Description: "subscription_bundle_groups=5,30"},
		30: {ID: 30, SubscriptionType: SubscriptionTypeSubscription, Status: StatusActive, Description: "subscription_bundle_groups=5,30"},
	}}
	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{ID: 101, UserID: 7001, GroupID: 5, StartsAt: start, ExpiresAt: expiresAt, Status: SubscriptionStatusActive})
	subRepo.seed(&UserSubscription{ID: 102, UserID: 7001, GroupID: 30, StartsAt: start, ExpiresAt: expiresAt, Status: SubscriptionStatusActive})

	svc := NewSubscriptionService(groupRepo, subRepo, nil, nil, nil)
	sub, err := svc.ExtendSubscriptionBundle(context.Background(), 101, 3)

	require.NoError(t, err)
	require.Equal(t, int64(101), sub.ID)
	require.Equal(t, expiresAt.AddDate(0, 0, 3), subRepo.byID[101].ExpiresAt)
	require.Equal(t, expiresAt.AddDate(0, 0, 3), subRepo.byID[102].ExpiresAt)
}

func TestExtendExpiredSubscriptionRenewsWindowAnchor(t *testing.T) {
	start := time.Now().UTC().AddDate(0, 0, -45)
	expiresAt := start.AddDate(0, 0, 30)
	oldWindowStart := start.AddDate(0, 0, 21)
	groupRepo := &subscriptionGroupRepoByIDStub{groups: map[int64]*Group{
		5: {ID: 5, SubscriptionType: SubscriptionTypeSubscription, Status: StatusActive},
	}}
	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID:                 301,
		UserID:             9001,
		GroupID:            5,
		StartsAt:           start,
		ExpiresAt:          expiresAt,
		Status:             SubscriptionStatusExpired,
		DailyWindowStart:   &oldWindowStart,
		WeeklyWindowStart:  &oldWindowStart,
		MonthlyWindowStart: &oldWindowStart,
		DailyUsageUSD:      11,
		WeeklyUsageUSD:     22,
		MonthlyUsageUSD:    33,
		Notes:              "old",
	})

	svc := NewSubscriptionService(groupRepo, subRepo, nil, nil, nil)
	before := time.Now()
	sub, err := svc.ExtendSubscription(context.Background(), 301, 30)
	after := time.Now()

	require.NoError(t, err)
	require.Equal(t, int64(301), sub.ID)
	require.Equal(t, SubscriptionStatusActive, sub.Status)
	require.True(t, sub.StartsAt.After(before) || sub.StartsAt.Equal(before))
	require.True(t, sub.StartsAt.Before(after) || sub.StartsAt.Equal(after))
	require.True(t, sub.ExpiresAt.After(before.AddDate(0, 0, 29)))
	require.True(t, sub.ExpiresAt.Before(after.AddDate(0, 0, 31)))
	require.NotNil(t, sub.DailyWindowStart)
	require.NotNil(t, sub.WeeklyWindowStart)
	require.NotNil(t, sub.MonthlyWindowStart)
	require.Equal(t, sub.StartsAt, *sub.DailyWindowStart)
	require.Equal(t, sub.StartsAt, *sub.WeeklyWindowStart)
	require.Equal(t, sub.StartsAt, *sub.MonthlyWindowStart)
	require.Equal(t, 0.0, sub.DailyUsageUSD)
	require.Equal(t, 0.0, sub.WeeklyUsageUSD)
	require.Equal(t, 0.0, sub.MonthlyUsageUSD)
}

func TestExtendExpiredSubscriptionBundleRenewsSiblingWindowAnchor(t *testing.T) {
	start := time.Now().UTC().AddDate(0, 0, -45)
	expiresAt := start.AddDate(0, 0, 30)
	oldWindowStart := start.AddDate(0, 0, 21)
	groupRepo := &subscriptionGroupRepoByIDStub{groups: map[int64]*Group{
		5:  {ID: 5, SubscriptionType: SubscriptionTypeSubscription, Status: StatusActive, Description: "subscription_bundle_groups=5,30"},
		30: {ID: 30, SubscriptionType: SubscriptionTypeSubscription, Status: StatusActive, Description: "subscription_bundle_groups=5,30"},
	}}
	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID:                 401,
		UserID:             9002,
		GroupID:            5,
		StartsAt:           start,
		ExpiresAt:          expiresAt,
		Status:             SubscriptionStatusExpired,
		WeeklyWindowStart:  &oldWindowStart,
		MonthlyWindowStart: &oldWindowStart,
		WeeklyUsageUSD:     22,
		MonthlyUsageUSD:    33,
	})
	subRepo.seed(&UserSubscription{
		ID:                 402,
		UserID:             9002,
		GroupID:            30,
		StartsAt:           start,
		ExpiresAt:          expiresAt,
		Status:             SubscriptionStatusExpired,
		WeeklyWindowStart:  &oldWindowStart,
		MonthlyWindowStart: &oldWindowStart,
		WeeklyUsageUSD:     44,
		MonthlyUsageUSD:    55,
	})

	svc := NewSubscriptionService(groupRepo, subRepo, nil, nil, nil)
	before := time.Now()
	sub, err := svc.ExtendSubscriptionBundle(context.Background(), 401, 30)
	after := time.Now()

	require.NoError(t, err)
	require.Equal(t, int64(401), sub.ID)
	for _, id := range []int64{401, 402} {
		renewed := subRepo.byID[id]
		require.NotNil(t, renewed)
		require.Equal(t, SubscriptionStatusActive, renewed.Status)
		require.True(t, renewed.StartsAt.After(before) || renewed.StartsAt.Equal(before))
		require.True(t, renewed.StartsAt.Before(after) || renewed.StartsAt.Equal(after))
		require.NotNil(t, renewed.WeeklyWindowStart)
		require.NotNil(t, renewed.MonthlyWindowStart)
		require.Equal(t, renewed.StartsAt, *renewed.WeeklyWindowStart)
		require.Equal(t, renewed.StartsAt, *renewed.MonthlyWindowStart)
		require.Equal(t, 0.0, renewed.WeeklyUsageUSD)
		require.Equal(t, 0.0, renewed.MonthlyUsageUSD)
	}
}

func TestRevokeSubscriptionBundleRevokesSiblingSubscription(t *testing.T) {
	start := time.Now().UTC().Add(-24 * time.Hour)
	groupRepo := &subscriptionGroupRepoByIDStub{groups: map[int64]*Group{
		5:  {ID: 5, SubscriptionType: SubscriptionTypeSubscription, Status: StatusActive, Description: "subscription_bundle_groups=5,30"},
		30: {ID: 30, SubscriptionType: SubscriptionTypeSubscription, Status: StatusActive, Description: "subscription_bundle_groups=5,30"},
	}}
	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{ID: 201, UserID: 8001, GroupID: 5, StartsAt: start, ExpiresAt: start.AddDate(0, 0, 30), Status: SubscriptionStatusActive})
	subRepo.seed(&UserSubscription{ID: 202, UserID: 8001, GroupID: 30, StartsAt: start, ExpiresAt: start.AddDate(0, 0, 30), Status: SubscriptionStatusActive})

	svc := NewSubscriptionService(groupRepo, subRepo, nil, nil, nil)
	err := svc.RevokeSubscriptionBundle(context.Background(), 201)

	require.NoError(t, err)
	require.ElementsMatch(t, []int64{201, 202}, subRepo.deletedIDs)
	_, err = subRepo.GetByID(context.Background(), 201)
	require.ErrorIs(t, err, ErrSubscriptionNotFound)
	_, err = subRepo.GetByID(context.Background(), 202)
	require.ErrorIs(t, err, ErrSubscriptionNotFound)
}

func TestNormalizeAssignValidityDays(t *testing.T) {
	require.Equal(t, 30, normalizeAssignValidityDays(0))
	require.Equal(t, 30, normalizeAssignValidityDays(-5))
	require.Equal(t, MaxValidityDays, normalizeAssignValidityDays(MaxValidityDays+100))
	require.Equal(t, 7, normalizeAssignValidityDays(7))
}

func TestDetectAssignSemanticConflictCases(t *testing.T) {
	start := time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC)
	base := &UserSubscription{
		UserID:    1,
		GroupID:   1,
		StartsAt:  start,
		ExpiresAt: start.AddDate(0, 0, 30),
		Notes:     "same",
	}

	reason, conflict := detectAssignSemanticConflict(base, &AssignSubscriptionInput{
		UserID:       1,
		GroupID:      1,
		ValidityDays: 30,
		Notes:        "same",
	})
	require.False(t, conflict)
	require.Equal(t, "", reason)

	reason, conflict = detectAssignSemanticConflict(base, &AssignSubscriptionInput{
		UserID:       1,
		GroupID:      1,
		ValidityDays: 60,
		Notes:        "same",
	})
	require.True(t, conflict)
	require.Equal(t, "validity_days_mismatch", reason)

	reason, conflict = detectAssignSemanticConflict(base, &AssignSubscriptionInput{
		UserID:       1,
		GroupID:      1,
		ValidityDays: 30,
		Notes:        "other",
	})
	require.True(t, conflict)
	require.Equal(t, "notes_mismatch", reason)
}

func TestAssignSubscriptionGroupTypeValidation(t *testing.T) {
	groupRepo := &subscriptionGroupRepoStub{
		group: &Group{ID: 1, SubscriptionType: SubscriptionTypeStandard},
	}
	subRepo := newSubscriptionUserSubRepoStub()
	svc := NewSubscriptionService(groupRepo, subRepo, nil, nil, nil)

	_, err := svc.AssignSubscription(context.Background(), &AssignSubscriptionInput{
		UserID:       1,
		GroupID:      1,
		ValidityDays: 30,
	})
	require.Error(t, err)
	require.Equal(t, infraerrors.Code(ErrGroupNotSubscriptionType), infraerrors.Code(err))
}

func strconvFormatInt(v int64) string {
	return strconv.FormatInt(v, 10)
}

func infraerrorsReason(err error) string {
	return infraerrors.Reason(err)
}
