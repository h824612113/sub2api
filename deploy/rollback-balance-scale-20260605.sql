-- Roll back the 2026-06-03 small-unit balance/billing change using a forward
-- migration on the current production database.
--
-- Source of truth for legacy group values:
--   /root/sub2api-backups/20260602T193001Z/sub2api.dump
--
-- This script intentionally restores the live "display / wallet / quota" unit
-- back to the old large-number system without rewriting historical usage logs.
-- Audit/history tables remain untouched.
--
-- Important:
-- 1. Take a fresh full backup before running this script.
-- 2. Run this during a short maintenance window.
-- 3. Deploy the frontend billing display rollback together with this SQL.
--
-- Scope:
-- - Restore known pre-change group multipliers/limits from the 2026-06-02 backup.
-- - For groups created after that backup, revert obvious /10 rates forward by x10.
-- - Restore live balances, subscription counters, API key quotas/windows, fixed
--   balance-notify thresholds, and unused wallet promo inventory to the large unit.
-- - Do not modify total_recharged, usage_logs, payment orders, or used redeem history.

BEGIN;

-- 1) Restore legacy group pricing/limits from the pre-change backup.
WITH legacy_groups (
  id,
  rate_multiplier,
  daily_limit_usd,
  weekly_limit_usd,
  monthly_limit_usd,
  image_rate_multiplier
) AS (
  VALUES
    (2,  1.5000::numeric,   0.00000000::numeric,    0.00000000::numeric, 0.00000000::numeric, 2.0000::numeric),
    (3,  1.0000::numeric,   NULL::numeric,          NULL::numeric,       NULL::numeric,       1.0000::numeric),
    (4,  1.0000::numeric,   NULL::numeric,          NULL::numeric,       NULL::numeric,       1.0000::numeric),
    (5,  1.5000::numeric, 100.00000000::numeric,  600.00000000::numeric, 0.00000000::numeric, 2.0000::numeric),
    (6,  1.0000::numeric,   NULL::numeric,          NULL::numeric,       NULL::numeric,       1.0000::numeric),
    (7,  1.5000::numeric,  50.00000000::numeric,  300.00000000::numeric, 0.00000000::numeric, 2.0000::numeric),
    (8,  1.5000::numeric,  20.00000000::numeric,  120.00000000::numeric, 0.00000000::numeric, 2.0000::numeric),
    (9,  1.0000::numeric,   0.00000000::numeric,    0.00000000::numeric, 0.00000000::numeric, 1.0000::numeric),
    (10, 1.0000::numeric,   0.00000000::numeric,    0.00000000::numeric, 0.00000000::numeric, 1.0000::numeric),
    (11, 1.0000::numeric,   0.00000000::numeric,    0.00000000::numeric, 0.00000000::numeric, 1.0000::numeric),
    (12, 1.0000::numeric,   0.00000000::numeric,    0.00000000::numeric, 0.00000000::numeric, 1.0000::numeric),
    (13, 1.5000::numeric, 200.00000000::numeric, 1200.00000000::numeric, 0.00000000::numeric, 2.0000::numeric),
    (15, 1.0000::numeric,   0.00000000::numeric,    0.00000000::numeric, 0.00000000::numeric, 1.0000::numeric),
    (16, 1.0000::numeric,   0.00000000::numeric,    0.00000000::numeric, 0.00000000::numeric, 1.0000::numeric),
    (17, 1.0000::numeric,   0.00000000::numeric,    0.00000000::numeric, 0.00000000::numeric, 1.0000::numeric),
    (18, 1.0000::numeric,   0.00000000::numeric,    0.00000000::numeric, 0.00000000::numeric, 1.0000::numeric),
    (19, 3.0000::numeric,   0.00000000::numeric,    0.00000000::numeric, 0.00000000::numeric, 1.0000::numeric),
    (20, 1.0000::numeric,   0.00000000::numeric,    0.00000000::numeric, 0.00000000::numeric, 1.0000::numeric)
)
UPDATE groups AS g
SET
  rate_multiplier = lg.rate_multiplier,
  daily_limit_usd = lg.daily_limit_usd,
  weekly_limit_usd = lg.weekly_limit_usd,
  monthly_limit_usd = lg.monthly_limit_usd,
  image_rate_multiplier = lg.image_rate_multiplier,
  updated_at = NOW()
FROM legacy_groups AS lg
WHERE g.id = lg.id;

-- 2) Revert obvious post-backup small-unit groups forward by x10.
-- Group 14 was repurposed after the backup; groups 21-23 were created later.
UPDATE groups
SET
  rate_multiplier = CASE
    WHEN rate_multiplier IN (0.1000, 0.2000, 0.3000) THEN rate_multiplier * 10
    ELSE rate_multiplier
  END,
  image_rate_multiplier = CASE
    WHEN image_rate_multiplier IN (0.1000, 0.2000, 0.3000) THEN image_rate_multiplier * 10
    ELSE image_rate_multiplier
  END,
  updated_at = NOW()
WHERE id IN (14, 21, 22, 23);

-- 3) Restore live user wallet balances and fixed notify thresholds.
UPDATE users
SET
  balance = balance * 10,
  balance_notify_threshold = CASE
    WHEN balance_notify_threshold_type = 'fixed' AND balance_notify_threshold > 0
      THEN balance_notify_threshold * 10
    ELSE balance_notify_threshold
  END,
  updated_at = NOW()
WHERE deleted_at IS NULL;

-- 4) Restore live subscription counters to the large display unit.
-- This keeps user-facing used/limit ratios stable after limits are restored x10.
UPDATE user_subscriptions
SET
  daily_usage_usd = daily_usage_usd * 10,
  weekly_usage_usd = weekly_usage_usd * 10,
  monthly_usage_usd = monthly_usage_usd * 10,
  updated_at = NOW()
WHERE deleted_at IS NULL;

-- 5) Restore API key quotas and rolling-window usage counters.
UPDATE api_keys
SET
  quota = quota * 10,
  quota_used = quota_used * 10,
  rate_limit_5h = rate_limit_5h * 10,
  rate_limit_1d = rate_limit_1d * 10,
  rate_limit_7d = rate_limit_7d * 10,
  usage_5h = usage_5h * 10,
  usage_1d = usage_1d * 10,
  usage_7d = usage_7d * 10,
  updated_at = NOW()
WHERE deleted_at IS NULL;

-- 6) Restore unused wallet/promo inventory only.
-- Used redeem history stays untouched for audit consistency.
UPDATE redeem_codes
SET value = value * 10
WHERE status = 'unused'
  AND type IN ('balance', 'admin_balance');

UPDATE promo_codes
SET
  bonus_amount = bonus_amount * 10,
  updated_at = NOW()
WHERE bonus_amount <> 0;

-- 7) Quick post-check output.
SELECT id, name, rate_multiplier, daily_limit_usd, weekly_limit_usd, image_rate_multiplier
FROM groups
WHERE id IN (2, 5, 7, 8, 13, 14, 19, 21, 22, 23)
ORDER BY id;

SELECT
  COUNT(*) AS user_count,
  ROUND(SUM(balance)::numeric, 4) AS total_balance
FROM users
WHERE deleted_at IS NULL;

SELECT
  COUNT(*) AS subscription_count,
  ROUND(SUM(daily_usage_usd)::numeric, 4) AS total_daily_usage,
  ROUND(SUM(weekly_usage_usd)::numeric, 4) AS total_weekly_usage,
  ROUND(SUM(monthly_usage_usd)::numeric, 4) AS total_monthly_usage
FROM user_subscriptions
WHERE deleted_at IS NULL;

SELECT
  COUNT(*) AS key_count,
  ROUND(SUM(quota)::numeric, 4) AS total_quota,
  ROUND(SUM(quota_used)::numeric, 4) AS total_quota_used
FROM api_keys
WHERE deleted_at IS NULL;

SELECT
  type,
  status,
  COUNT(*) AS code_count,
  ROUND(SUM(value)::numeric, 4) AS total_value
FROM redeem_codes
WHERE type IN ('balance', 'admin_balance')
GROUP BY type, status
ORDER BY type, status;

COMMIT;
