BEGIN;

UPDATE groups
SET
  daily_limit_usd = NULL,
  weekly_limit_usd = CASE id
    WHEN 5 THEN 500
    WHEN 30 THEN 500
    WHEN 7 THEN 300
    WHEN 29 THEN 300
    WHEN 8 THEN 150
    WHEN 28 THEN 150
    WHEN 13 THEN 1000
    WHEN 31 THEN 1000
    WHEN 34 THEN 2000
    WHEN 41 THEN 2000
    ELSE weekly_limit_usd
  END,
  monthly_limit_usd = CASE id
    WHEN 5 THEN 2000
    WHEN 30 THEN 2000
    WHEN 7 THEN 1200
    WHEN 29 THEN 1200
    WHEN 8 THEN 600
    WHEN 28 THEN 600
    WHEN 13 THEN 4000
    WHEN 31 THEN 4000
    WHEN 34 THEN 8000
    WHEN 41 THEN 8000
    ELSE monthly_limit_usd
  END,
  description = CASE id
    WHEN 5 THEN 'Pro 订阅分组：每周 500 美元额度，每月 2000 美元额度。' || E'\n' ||
      'quota_pool=plan_pro_monthly' || E'\n' ||
      'quota_pool_weekly_limit=500' || E'\n' ||
      'quota_pool_monthly_limit=2000' || E'\n' ||
      'subscription_bundle_groups=5,30'
    WHEN 30 THEN 'gpt 20x' || E'\n' ||
      'quota_pool=plan_pro_monthly' || E'\n' ||
      'quota_pool_weekly_limit=500' || E'\n' ||
      'quota_pool_monthly_limit=2000' || E'\n' ||
      'subscription_bundle_groups=5,30'
    WHEN 7 THEN 'Plus 订阅分组：每周 300 美元额度，每月 1200 美元额度。' || E'\n' ||
      'quota_pool=plan_plus_monthly' || E'\n' ||
      'quota_pool_weekly_limit=300' || E'\n' ||
      'quota_pool_monthly_limit=1200' || E'\n' ||
      'subscription_bundle_groups=7,29'
    WHEN 29 THEN '(gpt 20x池子)' || E'\n' ||
      'quota_pool=plan_plus_monthly' || E'\n' ||
      'quota_pool_weekly_limit=300' || E'\n' ||
      'quota_pool_monthly_limit=1200' || E'\n' ||
      'subscription_bundle_groups=7,29'
    WHEN 8 THEN 'Basic 订阅分组：每周 150 美元额度，每月 600 美元额度。' || E'\n' ||
      'quota_pool=plan_basic_monthly' || E'\n' ||
      'quota_pool_weekly_limit=150' || E'\n' ||
      'quota_pool_monthly_limit=600' || E'\n' ||
      'subscription_bundle_groups=8,28'
    WHEN 28 THEN 'gpt 20x池子' || E'\n' ||
      'quota_pool=plan_basic_monthly' || E'\n' ||
      'quota_pool_weekly_limit=150' || E'\n' ||
      'quota_pool_monthly_limit=600' || E'\n' ||
      'subscription_bundle_groups=8,28'
    WHEN 13 THEN 'Max 订阅分组：每周 1000 美元额度，每月 4000 美元额度。' || E'\n' ||
      'quota_pool=plan_max_monthly' || E'\n' ||
      'quota_pool_weekly_limit=1000' || E'\n' ||
      'quota_pool_monthly_limit=4000' || E'\n' ||
      'subscription_bundle_groups=13,31'
    WHEN 31 THEN 'Gpt 20x' || E'\n' ||
      'quota_pool=plan_max_monthly' || E'\n' ||
      'quota_pool_weekly_limit=1000' || E'\n' ||
      'quota_pool_monthly_limit=4000' || E'\n' ||
      'subscription_bundle_groups=13,31'
    WHEN 34 THEN 'Ultra 订阅分组：每周 2000 美元额度，每月 8000 美元额度。' || E'\n' ||
      'quota_pool=plan_ultra_monthly' || E'\n' ||
      'quota_pool_weekly_limit=2000' || E'\n' ||
      'quota_pool_monthly_limit=8000' || E'\n' ||
      'subscription_bundle_groups=34,41'
    WHEN 41 THEN 'Gpt 20x' || E'\n' ||
      'quota_pool=plan_ultra_monthly' || E'\n' ||
      'quota_pool_weekly_limit=2000' || E'\n' ||
      'quota_pool_monthly_limit=8000' || E'\n' ||
      'subscription_bundle_groups=34,41'
    ELSE description
  END,
  updated_at = now()
WHERE id IN (5,7,8,13,28,29,30,31,34,41);

UPDATE subscription_plans
SET
  features = CASE id
    WHEN 1 THEN '每周 USD 500 使用额度' || E'\n' || '每月总额度 USD 2000' || E'\n' || '支持最新 GPT-5.5 / 5.4 等 Codex 模型' || E'\n' || 'Pro 优先级通道'
    WHEN 2 THEN '每周 USD 300 使用额度' || E'\n' || '每月总额度 USD 1200' || E'\n' || '支持最新 GPT-5.5 / 5.4 等 Codex 模型' || E'\n' || 'Plus 优先级通道'
    WHEN 3 THEN '每周 USD 150 使用额度' || E'\n' || '每月总额度 USD 600' || E'\n' || '支持最新 GPT-5.5 / 5.4 等 Codex 模型' || E'\n' || 'Basic 优先级通道'
    WHEN 4 THEN '每周 USD 1000 使用额度' || E'\n' || '每月总额度 USD 4000' || E'\n' || '支持最新 GPT-5.5 / 5.4 等 Codex 模型' || E'\n' || 'Max 优先级通道'
    WHEN 5 THEN '每周 USD 2000 使用额度' || E'\n' || '每月总额度 USD 8000' || E'\n' || '支持最新 GPT-5.5 / 5.4 等 Codex 模型' || E'\n' || 'Ultra 优先级通道'
    ELSE features
  END,
  updated_at = now()
WHERE id IN (1,2,3,4,5);

UPDATE groups
SET daily_limit_usd = NULL, updated_at = now()
WHERE subscription_type = 'subscription' AND daily_limit_usd = 0;

COMMIT;
