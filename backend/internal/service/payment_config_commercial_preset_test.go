package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionplan"
)

func TestApplyCommercialRelayPresetCreatesGroupsAndPlans(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := &PaymentConfigService{entClient: client}

	result, err := svc.ApplyCommercialRelayPreset(ctx)
	if err != nil {
		t.Fatalf("ApplyCommercialRelayPreset returned error: %v", err)
	}

	if result.CreatedGroups != 7 || result.UpdatedGroups != 0 {
		t.Fatalf("group summary = created %d updated %d, want 7/0", result.CreatedGroups, result.UpdatedGroups)
	}
	if result.CreatedPlans != 7 || result.UpdatedPlans != 0 {
		t.Fatalf("plan summary = created %d updated %d, want 7/0", result.CreatedPlans, result.UpdatedPlans)
	}

	groupCount, err := client.Group.Query().
		Where(group.NameHasPrefix("relay-openai-"), group.DeletedAtIsNil()).
		Count(ctx)
	if err != nil {
		t.Fatalf("count preset groups: %v", err)
	}
	if groupCount != 7 {
		t.Fatalf("preset group count = %d, want 7", groupCount)
	}

	planCount, err := client.SubscriptionPlan.Query().
		Where(subscriptionplan.ProductNameHasPrefix("relay_openai_")).
		Count(ctx)
	if err != nil {
		t.Fatalf("count preset plans: %v", err)
	}
	if planCount != 7 {
		t.Fatalf("preset plan count = %d, want 7", planCount)
	}

	maxPlan, err := client.SubscriptionPlan.Query().
		Where(subscriptionplan.ProductNameEQ("relay_openai_max_31d")).
		Only(ctx)
	if err != nil {
		t.Fatalf("query max plan: %v", err)
	}
	if maxPlan.Price != 529 {
		t.Fatalf("max plan price = %v, want 529", maxPlan.Price)
	}
	if maxPlan.ValidityDays != 31 {
		t.Fatalf("max plan validity days = %d, want 31", maxPlan.ValidityDays)
	}

	maxGroup, err := client.Group.Query().
		Where(group.NameEQ("relay-openai-max"), group.DeletedAtIsNil()).
		Only(ctx)
	if err != nil {
		t.Fatalf("query max group: %v", err)
	}
	if maxGroup.Platform != PlatformOpenAI {
		t.Fatalf("max group platform = %q, want %q", maxGroup.Platform, PlatformOpenAI)
	}
	if maxGroup.SubscriptionType != SubscriptionTypeSubscription {
		t.Fatalf("max group subscription_type = %q, want %q", maxGroup.SubscriptionType, SubscriptionTypeSubscription)
	}
	if maxGroup.DailyLimitUsd == nil || *maxGroup.DailyLimitUsd != 200 {
		t.Fatalf("max group daily limit = %v, want 200", maxGroup.DailyLimitUsd)
	}
	if maxGroup.WeeklyLimitUsd == nil || *maxGroup.WeeklyLimitUsd != 1200 {
		t.Fatalf("max group weekly limit = %v, want 1200", maxGroup.WeeklyLimitUsd)
	}
}

func TestApplyCommercialRelayPresetIsIdempotentAndRestoresPresetValues(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := &PaymentConfigService{entClient: client}

	if _, err := svc.ApplyCommercialRelayPreset(ctx); err != nil {
		t.Fatalf("first ApplyCommercialRelayPreset returned error: %v", err)
	}

	if err := client.Group.Update().
		Where(group.NameEQ("relay-openai-standard"), group.DeletedAtIsNil()).
		SetRateMultiplier(9.99).
		Exec(ctx); err != nil {
		t.Fatalf("mutate preset group: %v", err)
	}
	if err := client.SubscriptionPlan.Update().
		Where(subscriptionplan.ProductNameEQ("relay_openai_pro_31d")).
		SetPrice(1).
		Exec(ctx); err != nil {
		t.Fatalf("mutate preset plan: %v", err)
	}

	result, err := svc.ApplyCommercialRelayPreset(ctx)
	if err != nil {
		t.Fatalf("second ApplyCommercialRelayPreset returned error: %v", err)
	}

	if result.CreatedGroups != 0 || result.CreatedPlans != 0 {
		t.Fatalf("second apply created groups/plans = %d/%d, want 0/0", result.CreatedGroups, result.CreatedPlans)
	}
	if result.UpdatedGroups != 7 || result.UpdatedPlans != 7 {
		t.Fatalf("second apply updated groups/plans = %d/%d, want 7/7", result.UpdatedGroups, result.UpdatedPlans)
	}

	groupCount, err := client.Group.Query().
		Where(group.NameHasPrefix("relay-openai-"), group.DeletedAtIsNil()).
		Count(ctx)
	if err != nil {
		t.Fatalf("count preset groups after reapply: %v", err)
	}
	if groupCount != 7 {
		t.Fatalf("preset group count after reapply = %d, want 7", groupCount)
	}

	proPlan, err := client.SubscriptionPlan.Query().
		Where(subscriptionplan.ProductNameEQ("relay_openai_pro_31d")).
		Only(ctx)
	if err != nil {
		t.Fatalf("query pro plan after reapply: %v", err)
	}
	if proPlan.Price != 279 {
		t.Fatalf("pro plan price after reapply = %v, want 279", proPlan.Price)
	}
	if proPlan.OriginalPrice == nil || *proPlan.OriginalPrice != 299 {
		t.Fatalf("pro plan original price after reapply = %v, want 299", proPlan.OriginalPrice)
	}
	if proPlan.ValidityDays != 31 {
		t.Fatalf("pro plan validity days after reapply = %d, want 31", proPlan.ValidityDays)
	}

	standardGroup, err := client.Group.Query().
		Where(group.NameEQ("relay-openai-standard"), group.DeletedAtIsNil()).
		Only(ctx)
	if err != nil {
		t.Fatalf("query standard group after reapply: %v", err)
	}
	if standardGroup.RateMultiplier != 1.15 {
		t.Fatalf("standard group rate multiplier after reapply = %v, want 1.15", standardGroup.RateMultiplier)
	}
}
