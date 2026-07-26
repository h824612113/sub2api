-- Add an Ultra Plus monthly plan by cloning the two Ultra scheduling groups.
-- The paired groups share one fixed quota pool, matching the existing Ultra plan.

INSERT INTO groups (
    name,
    description,
    rate_multiplier,
    is_exclusive,
    status,
    platform,
    subscription_type,
    daily_limit_usd,
    weekly_limit_usd,
    monthly_limit_usd,
    default_validity_days,
    image_price_1k,
    image_price_2k,
    image_price_4k,
    claude_code_only,
    fallback_group_id,
    model_routing,
    model_routing_enabled,
    fallback_group_id_on_invalid_request,
    mcp_xml_inject,
    supported_model_scopes,
    sort_order,
    allow_messages_dispatch,
    default_mapped_model,
    require_oauth_only,
    require_privacy_set,
    messages_dispatch_model_config,
    rpm_limit,
    allow_image_generation,
    image_rate_independent,
    image_rate_multiplier,
    models_list_config,
    peak_rate_enabled,
    peak_start,
    peak_end,
    peak_rate_multiplier,
    batch_image_discount_multiplier,
    batch_image_hold_multiplier,
    allow_batch_image_generation,
    video_rate_independent,
    video_rate_multiplier,
    video_price_480p,
    video_price_720p,
    video_price_1080p,
    web_search_price_per_call,
    max_reasoning_effort,
    reasoning_effort_mappings
)
SELECT
    '套餐-OPENAI-ULTRA-PLUS',
    '',
    source.rate_multiplier,
    source.is_exclusive,
    source.status,
    source.platform,
    source.subscription_type,
    NULL,
    4000,
    16000,
    source.default_validity_days,
    source.image_price_1k,
    source.image_price_2k,
    source.image_price_4k,
    source.claude_code_only,
    source.fallback_group_id,
    source.model_routing,
    source.model_routing_enabled,
    source.fallback_group_id_on_invalid_request,
    source.mcp_xml_inject,
    source.supported_model_scopes,
    source.sort_order,
    source.allow_messages_dispatch,
    source.default_mapped_model,
    source.require_oauth_only,
    source.require_privacy_set,
    source.messages_dispatch_model_config,
    source.rpm_limit,
    source.allow_image_generation,
    source.image_rate_independent,
    source.image_rate_multiplier,
    source.models_list_config,
    source.peak_rate_enabled,
    source.peak_start,
    source.peak_end,
    source.peak_rate_multiplier,
    source.batch_image_discount_multiplier,
    source.batch_image_hold_multiplier,
    source.allow_batch_image_generation,
    source.video_rate_independent,
    source.video_rate_multiplier,
    source.video_price_480p,
    source.video_price_720p,
    source.video_price_1080p,
    source.web_search_price_per_call,
    source.max_reasoning_effort,
    source.reasoning_effort_mappings
FROM groups source
WHERE source.name = '套餐-OPENAI-ULTRA'
  AND source.deleted_at IS NULL
  AND NOT EXISTS (
      SELECT 1
      FROM groups existing
      WHERE existing.name = '套餐-OPENAI-ULTRA-PLUS'
        AND existing.deleted_at IS NULL
  );

INSERT INTO groups (
    name,
    description,
    rate_multiplier,
    is_exclusive,
    status,
    platform,
    subscription_type,
    daily_limit_usd,
    weekly_limit_usd,
    monthly_limit_usd,
    default_validity_days,
    image_price_1k,
    image_price_2k,
    image_price_4k,
    claude_code_only,
    fallback_group_id,
    model_routing,
    model_routing_enabled,
    fallback_group_id_on_invalid_request,
    mcp_xml_inject,
    supported_model_scopes,
    sort_order,
    allow_messages_dispatch,
    default_mapped_model,
    require_oauth_only,
    require_privacy_set,
    messages_dispatch_model_config,
    rpm_limit,
    allow_image_generation,
    image_rate_independent,
    image_rate_multiplier,
    models_list_config,
    peak_rate_enabled,
    peak_start,
    peak_end,
    peak_rate_multiplier,
    batch_image_discount_multiplier,
    batch_image_hold_multiplier,
    allow_batch_image_generation,
    video_rate_independent,
    video_rate_multiplier,
    video_price_480p,
    video_price_720p,
    video_price_1080p,
    web_search_price_per_call,
    max_reasoning_effort,
    reasoning_effort_mappings
)
SELECT
    '套餐-OPENAI-ULTRA-PLUS (Gpt20x账号-稳定版)',
    '',
    source.rate_multiplier,
    source.is_exclusive,
    source.status,
    source.platform,
    source.subscription_type,
    NULL,
    4000,
    16000,
    source.default_validity_days,
    source.image_price_1k,
    source.image_price_2k,
    source.image_price_4k,
    source.claude_code_only,
    source.fallback_group_id,
    source.model_routing,
    source.model_routing_enabled,
    source.fallback_group_id_on_invalid_request,
    source.mcp_xml_inject,
    source.supported_model_scopes,
    source.sort_order,
    source.allow_messages_dispatch,
    source.default_mapped_model,
    source.require_oauth_only,
    source.require_privacy_set,
    source.messages_dispatch_model_config,
    source.rpm_limit,
    source.allow_image_generation,
    source.image_rate_independent,
    source.image_rate_multiplier,
    source.models_list_config,
    source.peak_rate_enabled,
    source.peak_start,
    source.peak_end,
    source.peak_rate_multiplier,
    source.batch_image_discount_multiplier,
    source.batch_image_hold_multiplier,
    source.allow_batch_image_generation,
    source.video_rate_independent,
    source.video_rate_multiplier,
    source.video_price_480p,
    source.video_price_720p,
    source.video_price_1080p,
    source.web_search_price_per_call,
    source.max_reasoning_effort,
    source.reasoning_effort_mappings
FROM groups source
WHERE source.name = '套餐-OPENAI-ULTRA (Gpt20x账号-稳定版)'
  AND source.deleted_at IS NULL
  AND NOT EXISTS (
      SELECT 1
      FROM groups existing
      WHERE existing.name = '套餐-OPENAI-ULTRA-PLUS (Gpt20x账号-稳定版)'
        AND existing.deleted_at IS NULL
  );

WITH bundle AS (
    SELECT
        primary_group.id AS primary_group_id,
        stable_group.id AS stable_group_id
    FROM groups primary_group
    CROSS JOIN groups stable_group
    WHERE primary_group.name = '套餐-OPENAI-ULTRA-PLUS'
      AND primary_group.deleted_at IS NULL
      AND stable_group.name = '套餐-OPENAI-ULTRA-PLUS (Gpt20x账号-稳定版)'
      AND stable_group.deleted_at IS NULL
)
UPDATE groups target
SET
    description = CASE
        WHEN target.id = bundle.primary_group_id THEN
            'Ultra Plus 订阅分组：每周 4000 美元额度，每月 16000 美元额度。'
        ELSE 'Gpt 20x'
    END || E'\n' ||
        'quota_pool=plan_ultra_plus_monthly' || E'\n' ||
        'quota_pool_weekly_limit=4000' || E'\n' ||
        'quota_pool_monthly_limit=16000' || E'\n' ||
        'subscription_bundle_groups=' || bundle.primary_group_id || ',' || bundle.stable_group_id,
    daily_limit_usd = NULL,
    weekly_limit_usd = 4000,
    monthly_limit_usd = 16000,
    updated_at = NOW()
FROM bundle
WHERE target.id IN (bundle.primary_group_id, bundle.stable_group_id);

-- Preserve the current Ultra account priorities for both scheduling pools.
INSERT INTO account_groups (account_id, group_id, priority, created_at)
SELECT source_binding.account_id, target_group.id, source_binding.priority, NOW()
FROM account_groups source_binding
JOIN groups source_group
  ON source_group.id = source_binding.group_id
 AND source_group.deleted_at IS NULL
JOIN groups target_group
  ON target_group.name = CASE source_group.name
      WHEN '套餐-OPENAI-ULTRA' THEN '套餐-OPENAI-ULTRA-PLUS'
      WHEN '套餐-OPENAI-ULTRA (Gpt20x账号-稳定版)' THEN '套餐-OPENAI-ULTRA-PLUS (Gpt20x账号-稳定版)'
  END
 AND target_group.deleted_at IS NULL
WHERE source_group.name IN (
    '套餐-OPENAI-ULTRA',
    '套餐-OPENAI-ULTRA (Gpt20x账号-稳定版)'
)
ON CONFLICT (account_id, group_id) DO UPDATE
SET priority = EXCLUDED.priority;

-- Preserve each Ultra group's channel association when one exists.
INSERT INTO channel_groups (channel_id, group_id, created_at)
SELECT source_binding.channel_id, target_group.id, NOW()
FROM channel_groups source_binding
JOIN groups source_group
  ON source_group.id = source_binding.group_id
 AND source_group.deleted_at IS NULL
JOIN groups target_group
  ON target_group.name = CASE source_group.name
      WHEN '套餐-OPENAI-ULTRA' THEN '套餐-OPENAI-ULTRA-PLUS'
      WHEN '套餐-OPENAI-ULTRA (Gpt20x账号-稳定版)' THEN '套餐-OPENAI-ULTRA-PLUS (Gpt20x账号-稳定版)'
  END
 AND target_group.deleted_at IS NULL
WHERE source_group.name IN (
    '套餐-OPENAI-ULTRA',
    '套餐-OPENAI-ULTRA (Gpt20x账号-稳定版)'
)
ON CONFLICT (group_id) DO UPDATE
SET channel_id = EXCLUDED.channel_id;

INSERT INTO subscription_plans (
    group_id,
    name,
    description,
    price,
    original_price,
    validity_days,
    validity_unit,
    features,
    product_name,
    for_sale,
    sort_order,
    currency
)
SELECT
    target_group.id,
    'Ultra Plus 月度套餐',
    'Ultra Plus 月度套餐，每周 4000 刀额度，每月 16000 刀额度。',
    2398.00,
    NULL,
    1,
    'month',
    '每周 USD 4000 使用额度' || E'\n' ||
        '每月总额度 USD 16000' || E'\n' ||
        '支持最新 GPT-5.6/ 5.5 等 Codex 模型' || E'\n' ||
        'Ultra Plus 优先级通道',
    'openai_ultra_plus_monthly',
    TRUE,
    -5,
    ''
FROM groups target_group
WHERE target_group.name = '套餐-OPENAI-ULTRA-PLUS'
  AND target_group.deleted_at IS NULL
  AND NOT EXISTS (
      SELECT 1
      FROM subscription_plans existing
      WHERE existing.product_name = 'openai_ultra_plus_monthly'
  );

UPDATE subscription_plans plan
SET
    group_id = target_group.id,
    name = 'Ultra Plus 月度套餐',
    description = 'Ultra Plus 月度套餐，每周 4000 刀额度，每月 16000 刀额度。',
    price = 2398.00,
    original_price = NULL,
    validity_days = 1,
    validity_unit = 'month',
    features = '每周 USD 4000 使用额度' || E'\n' ||
        '每月总额度 USD 16000' || E'\n' ||
        '支持最新 GPT-5.6/ 5.5 等 Codex 模型' || E'\n' ||
        'Ultra Plus 优先级通道',
    for_sale = TRUE,
    sort_order = -5,
    currency = '',
    updated_at = NOW()
FROM groups target_group
WHERE plan.product_name = 'openai_ultra_plus_monthly'
  AND target_group.name = '套餐-OPENAI-ULTRA-PLUS'
  AND target_group.deleted_at IS NULL;
