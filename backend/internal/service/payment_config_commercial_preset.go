package service

import (
	"context"
	"fmt"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionplan"
)

type CommercialRelayPresetResult struct {
	CreatedGroups int                     `json:"created_groups"`
	UpdatedGroups int                     `json:"updated_groups"`
	CreatedPlans  int                     `json:"created_plans"`
	UpdatedPlans  int                     `json:"updated_plans"`
	Groups        []CommercialPresetGroup `json:"groups"`
	Plans         []CommercialPresetPlan  `json:"plans"`
}

type CommercialPresetGroup struct {
	ID              int64    `json:"id"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Platform        string   `json:"platform"`
	RateMultiplier  float64  `json:"rate_multiplier"`
	DailyLimitUSD   *float64 `json:"daily_limit_usd"`
	WeeklyLimitUSD  *float64 `json:"weekly_limit_usd"`
	MonthlyLimitUSD *float64 `json:"monthly_limit_usd"`
	RPMLimit        int      `json:"rpm_limit"`
}

type CommercialPresetPlan struct {
	ID           int64   `json:"id"`
	GroupID      int64   `json:"group_id"`
	Name         string  `json:"name"`
	Price        float64 `json:"price"`
	ValidityDays int     `json:"validity_days"`
	ForSale      bool    `json:"for_sale"`
}

type commercialRelayGroupSpec struct {
	Name            string
	Description     string
	RateMultiplier  float64
	DailyLimitUSD   *float64
	WeeklyLimitUSD  *float64
	MonthlyLimitUSD *float64
	RPMLimit        int
	SortOrder       int
}

type commercialRelayPlanSpec struct {
	GroupName     string
	Name          string
	Description   string
	Price         float64
	OriginalPrice *float64
	ValidityDays  int
	Features      string
	ProductName   string
	ForSale       bool
	SortOrder     int
}

func (s *PaymentConfigService) ApplyCommercialRelayPreset(ctx context.Context) (*CommercialRelayPresetResult, error) {
	if s.entClient == nil {
		return nil, fmt.Errorf("payment config ent client not initialized")
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("start commercial preset tx: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	result := &CommercialRelayPresetResult{}
	groupIDs := make(map[string]int64)

	for _, spec := range commercialRelayPresetGroups() {
		entity, created, err := upsertCommercialRelayGroup(ctx, tx.Client(), spec)
		if err != nil {
			return nil, err
		}
		if created {
			result.CreatedGroups++
		} else {
			result.UpdatedGroups++
		}
		groupIDs[spec.Name] = int64(entity.ID)
		result.Groups = append(result.Groups, CommercialPresetGroup{
			ID:              int64(entity.ID),
			Name:            entity.Name,
			Description:     commercialRelayStringValue(entity.Description),
			Platform:        entity.Platform,
			RateMultiplier:  entity.RateMultiplier,
			DailyLimitUSD:   entity.DailyLimitUsd,
			WeeklyLimitUSD:  entity.WeeklyLimitUsd,
			MonthlyLimitUSD: entity.MonthlyLimitUsd,
			RPMLimit:        entity.RpmLimit,
		})
	}

	for _, spec := range commercialRelayPresetPlans() {
		groupID, ok := groupIDs[spec.GroupName]
		if !ok {
			return nil, fmt.Errorf("preset plan %s references unknown group %s", spec.ProductName, spec.GroupName)
		}
		entity, created, err := upsertCommercialRelayPlan(ctx, tx.Client(), groupID, spec)
		if err != nil {
			return nil, err
		}
		if created {
			result.CreatedPlans++
		} else {
			result.UpdatedPlans++
		}
		result.Plans = append(result.Plans, CommercialPresetPlan{
			ID:           int64(entity.ID),
			GroupID:      entity.GroupID,
			Name:         entity.Name,
			Price:        entity.Price,
			ValidityDays: entity.ValidityDays,
			ForSale:      entity.ForSale,
		})
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit commercial preset tx: %w", err)
	}
	committed = true

	return result, nil
}

func upsertCommercialRelayGroup(ctx context.Context, client *dbent.Client, spec commercialRelayGroupSpec) (*dbent.Group, bool, error) {
	existing, err := client.Group.Query().
		Where(group.NameEQ(spec.Name), group.DeletedAtIsNil()).
		Only(ctx)
	if err != nil && !dbent.IsNotFound(err) {
		return nil, false, fmt.Errorf("query preset group %s: %w", spec.Name, err)
	}

	if dbent.IsNotFound(err) {
		created, createErr := client.Group.Create().
			SetName(spec.Name).
			SetDescription(spec.Description).
			SetPlatform(PlatformOpenAI).
			SetSubscriptionType(SubscriptionTypeSubscription).
			SetStatus(StatusActive).
			SetRateMultiplier(spec.RateMultiplier).
			SetIsExclusive(false).
			SetDefaultValidityDays(30).
			SetSupportedModelScopes([]string{}).
			SetSortOrder(spec.SortOrder).
			SetRpmLimit(spec.RPMLimit).
			SetNillableDailyLimitUsd(spec.DailyLimitUSD).
			SetNillableWeeklyLimitUsd(spec.WeeklyLimitUSD).
			SetNillableMonthlyLimitUsd(spec.MonthlyLimitUSD).
			Save(ctx)
		if createErr != nil {
			return nil, false, fmt.Errorf("create preset group %s: %w", spec.Name, createErr)
		}
		return created, true, nil
	}

	updated, updateErr := existing.Update().
		SetDescription(spec.Description).
		SetPlatform(PlatformOpenAI).
		SetSubscriptionType(SubscriptionTypeSubscription).
		SetStatus(StatusActive).
		SetRateMultiplier(spec.RateMultiplier).
		SetIsExclusive(false).
		SetDefaultValidityDays(30).
		SetSupportedModelScopes([]string{}).
		SetSortOrder(spec.SortOrder).
		SetRpmLimit(spec.RPMLimit).
		SetNillableDailyLimitUsd(spec.DailyLimitUSD).
		SetNillableWeeklyLimitUsd(spec.WeeklyLimitUSD).
		SetNillableMonthlyLimitUsd(spec.MonthlyLimitUSD).
		Save(ctx)
	if updateErr != nil {
		return nil, false, fmt.Errorf("update preset group %s: %w", spec.Name, updateErr)
	}
	return updated, false, nil
}

func upsertCommercialRelayPlan(ctx context.Context, client *dbent.Client, groupID int64, spec commercialRelayPlanSpec) (*dbent.SubscriptionPlan, bool, error) {
	existing, err := client.SubscriptionPlan.Query().
		Where(subscriptionplan.ProductNameEQ(spec.ProductName)).
		Only(ctx)
	if err != nil && !dbent.IsNotFound(err) {
		return nil, false, fmt.Errorf("query preset plan %s: %w", spec.ProductName, err)
	}

	if dbent.IsNotFound(err) {
		created, createErr := client.SubscriptionPlan.Create().
			SetGroupID(groupID).
			SetName(spec.Name).
			SetDescription(spec.Description).
			SetPrice(spec.Price).
			SetValidityDays(spec.ValidityDays).
			SetValidityUnit("days").
			SetFeatures(spec.Features).
			SetProductName(spec.ProductName).
			SetForSale(spec.ForSale).
			SetSortOrder(spec.SortOrder).
			SetNillableOriginalPrice(spec.OriginalPrice).
			Save(ctx)
		if createErr != nil {
			return nil, false, fmt.Errorf("create preset plan %s: %w", spec.ProductName, createErr)
		}
		return created, true, nil
	}

	updated := existing.Update().
		SetGroupID(groupID).
		SetName(spec.Name).
		SetDescription(spec.Description).
		SetPrice(spec.Price).
		SetValidityDays(spec.ValidityDays).
		SetValidityUnit("days").
		SetFeatures(spec.Features).
		SetProductName(spec.ProductName).
		SetForSale(spec.ForSale).
		SetSortOrder(spec.SortOrder)
	if spec.OriginalPrice != nil {
		updated = updated.SetOriginalPrice(*spec.OriginalPrice)
	} else {
		updated = updated.ClearOriginalPrice()
	}
	saved, updateErr := updated.Save(ctx)
	if updateErr != nil {
		return nil, false, fmt.Errorf("update preset plan %s: %w", spec.ProductName, updateErr)
	}
	return saved, false, nil
}

func commercialRelayPresetGroups() []commercialRelayGroupSpec {
	return []commercialRelayGroupSpec{
		{
			Name:            "relay-openai-trial",
			Description:     "试用池，限制更严格，用于体验和低价用户隔离。",
			RateMultiplier:  1.60,
			DailyLimitUSD:   commercialRelayFloatPtr(8),
			WeeklyLimitUSD:  commercialRelayFloatPtr(25),
			MonthlyLimitUSD: commercialRelayFloatPtr(60),
			RPMLimit:        12,
			SortOrder:       10,
		},
		{
			Name:            "relay-openai-basic",
			Description:     "入门池，面向轻度用户，控制成本优先。",
			RateMultiplier:  1.30,
			DailyLimitUSD:   commercialRelayFloatPtr(20),
			WeeklyLimitUSD:  commercialRelayFloatPtr(65),
			MonthlyLimitUSD: commercialRelayFloatPtr(260),
			RPMLimit:        24,
			SortOrder:       20,
		},
		{
			Name:            "relay-openai-standard",
			Description:     "标准池，覆盖大多数日常开发与办公使用。",
			RateMultiplier:  1.15,
			DailyLimitUSD:   commercialRelayFloatPtr(80),
			WeeklyLimitUSD:  commercialRelayFloatPtr(250),
			MonthlyLimitUSD: commercialRelayFloatPtr(1000),
			RPMLimit:        40,
			SortOrder:       30,
		},
		{
			Name:            "relay-openai-pro",
			Description:     "Pro 池，适合高频个人开发与中等强度持续调用。",
			RateMultiplier:  1.08,
			DailyLimitUSD:   commercialRelayFloatPtr(80),
			WeeklyLimitUSD:  commercialRelayFloatPtr(400),
			MonthlyLimitUSD: commercialRelayFloatPtr(1600),
			RPMLimit:        60,
			SortOrder:       40,
		},
		{
			Name:            "relay-openai-max",
			Description:     "Max 池，面向重度开发工作流与高并发持续调用。",
			RateMultiplier:  1.00,
			DailyLimitUSD:   commercialRelayFloatPtr(200),
			WeeklyLimitUSD:  commercialRelayFloatPtr(1200),
			MonthlyLimitUSD: commercialRelayFloatPtr(0),
			RPMLimit:        100,
			SortOrder:       50,
		},
		{
			Name:            "relay-openai-team",
			Description:     "团队池，适合小团队协作和稳定高频使用。",
			RateMultiplier:  0.95,
			DailyLimitUSD:   commercialRelayFloatPtr(420),
			WeeklyLimitUSD:  commercialRelayFloatPtr(1400),
			MonthlyLimitUSD: commercialRelayFloatPtr(5600),
			RPMLimit:        140,
			SortOrder:       60,
		},
		{
			Name:            "relay-openai-enterprise",
			Description:     "企业池，预留独立号池和更高并发保障。",
			RateMultiplier:  0.90,
			DailyLimitUSD:   commercialRelayFloatPtr(800),
			WeeklyLimitUSD:  commercialRelayFloatPtr(2600),
			MonthlyLimitUSD: commercialRelayFloatPtr(10500),
			RPMLimit:        220,
			SortOrder:       70,
		},
	}
}

func commercialRelayPresetPlans() []commercialRelayPlanSpec {
	return []commercialRelayPlanSpec{
		{
			GroupName:    "relay-openai-trial",
			Name:         "体验版",
			Description:  "适合体验接入、少量日常调用和购买前验证稳定性。",
			Price:        9.90,
			ValidityDays: 30,
			Features:     "独立试用池\n适合轻量体验\n严格限额控制",
			ProductName:  "relay_openai_trial_30d",
			ForSale:      true,
			SortOrder:    10,
		},
		{
			GroupName:    "relay-openai-basic",
			Name:         "入门版",
			Description:  "适合轻量办公、简单开发辅助和日常使用。",
			Price:        39,
			ValidityDays: 30,
			Features:     "月总额度 $260\n轻量用户友好\n成本控制优先",
			ProductName:  "relay_openai_basic_30d",
			ForSale:      true,
			SortOrder:    20,
		},
		{
			GroupName:    "relay-openai-standard",
			Name:         "标准版",
			Description:  "适合常规开发、脚本生成、文档处理和连续对话。",
			Price:        99,
			ValidityDays: 30,
			Features:     "月总额度 $1000\n覆盖大多数用户场景\n推荐默认套餐",
			ProductName:  "relay_openai_standard_30d",
			ForSale:      true,
			SortOrder:    30,
		},
		{
			GroupName:     "relay-openai-pro",
			Name:          "Pro",
			Description:   "Pro 月度套餐，每周 600 美元额度，每月 2600 美元额度。",
			Price:         279,
			OriginalPrice: commercialRelayFloatPtr(299),
			ValidityDays:  31,
			Features:      "每周 USD 600 使用额度\n每月总额度 USD 2600\n支持最新 GPT-5.6/ 5.5 等 Codex 模型\nPro 优先级通道",
			ProductName:   "relay_openai_pro_31d",
			ForSale:       true,
			SortOrder:     50,
		},
		{
			GroupName:    "relay-openai-max",
			Name:         "Max",
			Description:  "Max 月度套餐，每周 1200 美元额度，每月 5200 美元额度。",
			Price:        529,
			ValidityDays: 31,
			Features:     "每周 USD 1200 使用额度\n每月总额度 USD 5200\n支持最新 GPT-5.6/ 5.5 等 Codex 模型\nMax 优先级通道",
			ProductName:  "relay_openai_max_31d",
			ForSale:      true,
			SortOrder:    40,
		},
		{
			GroupName:    "relay-openai-team",
			Name:         "团队版",
			Description:  "适合小团队共享、持续开发和稳定高频调用。",
			Price:        499,
			ValidityDays: 30,
			Features:     "月总额度 $5600\n团队协作场景\n优先级更高的号池",
			ProductName:  "relay_openai_team_30d",
			ForSale:      true,
			SortOrder:    60,
		},
		{
			GroupName:    "relay-openai-enterprise",
			Name:         "企业版",
			Description:  "适合企业客户、定制支持和更高稳定性保障。",
			Price:        999,
			ValidityDays: 30,
			Features:     "月总额度 $10500\n企业专属号池\n建议私聊交付",
			ProductName:  "relay_openai_enterprise_30d",
			ForSale:      false,
			SortOrder:    70,
		},
	}
}

func commercialRelayFloatPtr(v float64) *float64 {
	return &v
}

func commercialRelayStringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
