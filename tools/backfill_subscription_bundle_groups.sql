-- Backfill sibling subscription groups for existing active subscriptions.
-- Usage:
--   docker exec -i sub2api-postgres psql -U sub2api -d sub2api < tools/backfill_subscription_bundle_groups.sql
--
-- The usage columns are intentionally initialized to zero on sibling
-- subscriptions so pooled usage does not double-count historical usage.

WITH bundles(source_group_id, target_group_id) AS (
  VALUES
    (8::bigint, 28::bigint),
    (7::bigint, 29::bigint),
    (5::bigint, 30::bigint),
    (13::bigint, 31::bigint),
    (34::bigint, 41::bigint)
),
source_subs AS (
  SELECT
    us.user_id,
    b.target_group_id AS group_id,
    us.starts_at,
    us.expires_at,
    us.status,
    us.assigned_by,
    us.assigned_at,
    CONCAT(
      COALESCE(us.notes, ''),
      CASE WHEN COALESCE(us.notes, '') = '' THEN '' ELSE E'\n' END,
      'backfilled sibling subscription from group ',
      b.source_group_id
    ) AS notes
  FROM user_subscriptions us
  JOIN bundles b ON b.source_group_id = us.group_id
  JOIN groups target_group ON target_group.id = b.target_group_id
  WHERE us.deleted_at IS NULL
    AND us.status = 'active'
    AND us.expires_at > NOW()
    AND target_group.deleted_at IS NULL
    AND target_group.status = 'active'
),
inserted AS (
  INSERT INTO user_subscriptions (
    user_id,
    group_id,
    starts_at,
    expires_at,
    status,
    daily_usage_usd,
    weekly_usage_usd,
    monthly_usage_usd,
    assigned_by,
    assigned_at,
    notes,
    created_at,
    updated_at
  )
  SELECT
    user_id,
    group_id,
    starts_at,
    expires_at,
    status,
    0,
    0,
    0,
    assigned_by,
    assigned_at,
    notes,
    NOW(),
    NOW()
  FROM source_subs
  ON CONFLICT DO NOTHING
  RETURNING id, user_id, group_id
)
SELECT group_id, COUNT(*) AS inserted_count
FROM inserted
GROUP BY group_id
ORDER BY group_id;
